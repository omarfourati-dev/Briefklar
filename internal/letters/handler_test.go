package letters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/auth"
	"github.com/omarfourati-dev/briefklar/internal/explain"
	"github.com/omarfourati-dev/briefklar/internal/extract"
	"github.com/omarfourati-dev/briefklar/internal/metrics"
)

type fakeExtractor struct {
	text string
	err  error
}

func (f fakeExtractor) Extract(context.Context, []byte) (string, extract.Kind, error) {
	return f.text, extract.Image, f.err
}

type fakeQuota struct{ ok bool }

func (f fakeQuota) TakeQuota(context.Context, string, time.Time) (int, bool, error) {
	return 1, f.ok, nil
}

type recordingExplainer struct{ got string }

func (r *recordingExplainer) Explain(ctx context.Context, text string) (explain.Explanation, error) {
	r.got = text
	return explain.Fake{}.Explain(ctx, text)
}

const sample = "Herrn\nKarim Benali\nLindenweg 7\n51645 Gummersbach\nSehr geehrter Herr Benali,\nIhr Aufenthaltstitel läuft am 30.11.2026 ab."

func handler(ex Extractor, quota bool, exp explain.Explainer) *Handler {
	return &Handler{Extractor: ex, Explainer: exp, Quota: fakeQuota{ok: quota}, Metrics: metrics.New(),
		Now: time.Now, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func as(role string, req *http.Request) *http.Request {
	return req.WithContext(auth.WithClaims(req.Context(), auth.Claims{UserID: "u1", Role: role}))
}

func upload(t *testing.T, field, value string, file []byte) *http.Request {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if file != nil {
		fw, _ := w.CreateFormFile("file", "brief.png")
		_, _ = fw.Write(file)
	} else {
		_ = w.WriteField(field, value)
	}
	_ = w.Close()
	req := httptest.NewRequest("POST", "/api/letters/preview", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestPreviewText(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(nil, true, nil).Preview(rec, as("user", upload(t, "text", sample, nil)))
	var out struct {
		Text      string                   `json:"text"`
		InputKind string                   `json:"inputKind"`
		Findings  []struct{ Value string } `json:"findings"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || out.InputKind != "text" || strings.Contains(out.Text, "Benali") || len(out.Findings) == 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestPreviewImage(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fakeExtractor{text: sample}, true, nil).Preview(rec, as("user", upload(t, "", "", []byte{0x89, 'P', 'N', 'G'})))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"inputKind":"image"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestPreviewNoText(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fakeExtractor{text: "x y", err: extract.ErrNoText}, true, nil).Preview(rec, as("user", upload(t, "", "", []byte{0x89, 'P', 'N', 'G'})))
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"recognizedText":"x y"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestPreviewUnsupported(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fakeExtractor{err: extract.ErrUnsupported}, true, nil).Preview(rec, as("user", upload(t, "", "", []byte("PK"))))
	if rec.Code != 400 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestPreviewTooLarge(t *testing.T) {
	rec := httptest.NewRecorder()
	big := make([]byte, maxUpload+128<<10) // above the reader limit (upload + 64 KB form overhead)
	copy(big, []byte{0x89, 'P', 'N', 'G'})
	handler(fakeExtractor{text: sample}, true, nil).Preview(rec, as("user", upload(t, "", "", big)))
	if rec.Code != 413 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestDemoCannotUploadOrExplain(t *testing.T) {
	h := handler(fakeExtractor{text: sample}, true, explain.Fake{})
	rec := httptest.NewRecorder()
	h.Preview(rec, as("demo", upload(t, "text", sample, nil)))
	if rec.Code != 403 {
		t.Fatalf("preview %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Explain(rec, as("demo", httptest.NewRequest("POST", "/api/letters/explain", strings.NewReader(`{"text":"`+strings.Repeat("a", 30)+`"}`))))
	if rec.Code != 403 {
		t.Fatalf("explain %d", rec.Code)
	}
}

func explainReq(text string) *http.Request {
	body, _ := json.Marshal(map[string]string{"text": text})
	return as("user", httptest.NewRequest("POST", "/api/letters/explain", bytes.NewReader(body)))
}

func TestExplainRedactsAgain(t *testing.T) {
	rec := &recordingExplainer{}
	out := httptest.NewRecorder()
	// the user removed nothing; an e-mail address slipped through the preview
	handler(nil, true, rec).Explain(out, explainReq("Sehr geehrte Frau [NAME_1], schreiben Sie an amt@stadt.de bis 15.11.2026."))
	if out.Code != 200 || strings.Contains(rec.got, "amt@stadt.de") || !strings.Contains(rec.got, "[NAME_1]") {
		t.Fatalf("%d sent=%q", out.Code, rec.got)
	}
	if !strings.Contains(out.Body.String(), `"placeholder":"[EMAIL_1]"`) || !strings.Contains(out.Body.String(), `"authority"`) {
		t.Fatalf("body %s", out.Body)
	}
}

func TestExplainLimits(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(nil, false, explain.Fake{}).Explain(rec, explainReq(strings.Repeat("Brief ", 10)))
	if rec.Code != 429 {
		t.Fatalf("quota %d", rec.Code)
	}
	for _, text := range []string{"kurz", strings.Repeat("a", 20001)} {
		rec = httptest.NewRecorder()
		handler(nil, true, explain.Fake{}).Explain(rec, explainReq(text))
		if rec.Code != 400 {
			t.Fatalf("len %d: %d", len(text), rec.Code)
		}
	}
}

type failingExplainer struct{ err error }

func (f failingExplainer) Explain(context.Context, string) (explain.Explanation, error) {
	return explain.Explanation{}, f.err
}

func TestExplainUpstreamErrors(t *testing.T) {
	for _, err := range []error{explain.ErrUpstream, explain.ErrTimeout, errors.New("boom")} {
		rec := httptest.NewRecorder()
		handler(nil, true, failingExplainer{err}).Explain(rec, explainReq(strings.Repeat("Brief ", 10)))
		if rec.Code != 502 || strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("%v: %d %s", err, rec.Code, rec.Body)
		}
	}
}
