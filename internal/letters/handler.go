// Package letters is the core API: preview (extract + redact) and explain (LLM on redacted text only).
// Nothing is stored; letter text is never logged.
package letters

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/omarfourati-dev/briefklar/internal/auth"
	"github.com/omarfourati-dev/briefklar/internal/explain"
	"github.com/omarfourati-dev/briefklar/internal/extract"
	"github.com/omarfourati-dev/briefklar/internal/metrics"
	"github.com/omarfourati-dev/briefklar/internal/redact"
	"github.com/omarfourati-dev/briefklar/internal/respond"
)

const (
	maxUpload      = 10 << 20
	minText        = 20
	maxText        = 20000
	extractTimeout = 45 * time.Second
	explainTimeout = 60 * time.Second
)

type Extractor interface {
	Extract(ctx context.Context, data []byte) (string, extract.Kind, error)
}

type Quota interface {
	TakeQuota(ctx context.Context, userID string, day time.Time) (int, bool, error)
}

type Handler struct {
	Extractor Extractor
	Explainer explain.Explainer
	Quota     Quota
	Metrics   *metrics.Metrics
	Now       func() time.Time
	Log       *slog.Logger
}

// inputLabel keeps the metric label inside the fixed set text/pdf/image/unknown.
func inputLabel(k extract.Kind) string {
	switch k {
	case extract.Text, extract.PDF, extract.Image:
		return string(k)
	}
	return "unknown"
}

func demoForbidden(w http.ResponseWriter, r *http.Request) bool {
	if c, _ := auth.ClaimsFrom(r.Context()); c.Role == "demo" {
		respond.Problem(w, http.StatusForbidden, "Forbidden",
			"Der Demo-Zugang zeigt nur die Beispielbriefe. Für eigene Briefe bitte einen Zugang anfragen.")
		return true
	}
	return false
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	if demoForbidden(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			h.Metrics.Previews.WithLabelValues("unknown", "too_large").Inc()
			respond.Problem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "Die Datei ist größer als 10 MB.")
			return
		}
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Bitte eine Datei oder einen Text senden.")
		return
	}
	defer r.MultipartForm.RemoveAll()

	text, kind := r.FormValue("text"), extract.Text
	if file, _, err := r.FormFile("file"); err == nil {
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			respond.Problem(w, http.StatusBadRequest, "Bad Request", "Die Datei konnte nicht gelesen werden.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), extractTimeout)
		start := time.Now()
		text, kind, err = h.Extractor.Extract(ctx, data)
		cancel()
		input := inputLabel(kind)
		h.Metrics.OCRDuration.WithLabelValues(input).Observe(time.Since(start).Seconds())
		switch {
		case errors.Is(err, extract.ErrUnsupported):
			h.Metrics.Previews.WithLabelValues("unknown", "unsupported").Inc()
			respond.Problem(w, http.StatusBadRequest, "Bad Request", "Unterstützt werden PDF, JPG, PNG und WebP.")
			return
		case errors.Is(err, extract.ErrNoText):
			h.Metrics.Previews.WithLabelValues(input, "no_text").Inc()
			respond.ProblemExtra(w, http.StatusUnprocessableEntity, "Kein Text erkannt",
				"Auf dem Bild ist kaum Text lesbar. Bitte gerade, hell und scharf fotografieren.",
				map[string]any{"recognizedText": text})
			return
		case err != nil:
			h.Metrics.Previews.WithLabelValues(input, "error").Inc()
			h.Log.Error("text extraction failed", "kind", kind, "error", err)
			respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "Die Texterkennung ist fehlgeschlagen.")
			return
		}
	}
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n < minText || n > maxText {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Der Text muss zwischen 20 und 20.000 Zeichen lang sein.")
		return
	}
	res := redact.Redact(text)
	for _, f := range res.Findings {
		h.Metrics.Redactions.WithLabelValues(string(f.Kind)).Inc()
	}
	h.Metrics.Previews.WithLabelValues(string(kind), "ok").Inc()
	respond.JSON(w, http.StatusOK, map[string]any{"text": res.Text, "findings": res.Findings, "inputKind": kind})
}

func (h *Handler) Explain(w http.ResponseWriter, r *http.Request) {
	if demoForbidden(w, r) {
		return
	}
	var in struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&in); err != nil {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Erwartet: {\"text\": \"…\"}")
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if n := utf8.RuneCountInString(in.Text); n < minText || n > maxText {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Der Text muss zwischen 20 und 20.000 Zeichen lang sein.")
		return
	}
	c, _ := auth.ClaimsFrom(r.Context())
	_, ok, err := h.Quota.TakeQuota(r.Context(), c.UserID, h.Now())
	if err != nil {
		respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if !ok {
		h.Metrics.Explanations.WithLabelValues("limit").Inc()
		respond.Problem(w, http.StatusTooManyRequests, "Too Many Requests", "Das Tageslimit für Erklärungen ist erreicht. Morgen geht es weiter.")
		return
	}
	// Defense in depth: redact again, in case something slipped through the preview.
	res := redact.Redact(in.Text)
	ctx, cancel := context.WithTimeout(r.Context(), explainTimeout)
	defer cancel()
	start := time.Now()
	e, err := h.Explainer.Explain(ctx, res.Text)
	h.Metrics.ExplainDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		outcome := "error"
		if errors.Is(err, explain.ErrTimeout) {
			outcome = "timeout"
		}
		h.Metrics.Explanations.WithLabelValues(outcome).Inc()
		h.Log.Warn("explain failed", "outcome", outcome)
		respond.Problem(w, http.StatusBadGateway, "Bad Gateway", "Die Erklärung ist gerade nicht möglich. Bitte später erneut versuchen.")
		return
	}
	h.Metrics.Explanations.WithLabelValues("ok").Inc()
	respond.JSON(w, http.StatusOK, map[string]any{"explanation": e, "findings": res.Findings})
}
