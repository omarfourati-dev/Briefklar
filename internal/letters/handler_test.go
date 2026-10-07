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

type errQuota struct{}

func (errQuota) TakeQuota(context.Context, string, time.Time) (int, bool, error) {
	return 0, false, errors.New("db down")
}

func TestExplainQuotaError(t *testing.T) {
	h := handler(nil, true, explain.Fake{})
	h.Quota = errQuota{}
	rec := httptest.NewRecorder()
	h.Explain(rec, explainReq(strings.Repeat("Brief ", 10)))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "Die Erklärung ist gerade nicht möglich") || strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestPreviewExactLimit(t *testing.T) {
	png := func(n int) []byte {
		b := make([]byte, n)
		copy(b, []byte{0x89, 'P', 'N', 'G'})
		return b
	}
	rec := httptest.NewRecorder()
	handler(fakeExtractor{text: sample}, true, nil).Preview(rec, as("user", upload(t, "", "", png(maxUpload+1))))
	if rec.Code != 413 {
		t.Fatalf("limit+1: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	handler(fakeExtractor{text: sample}, true, nil).Preview(rec, as("user", upload(t, "", "", png(maxUpload))))
	if rec.Code == 413 {
		t.Fatalf("exact limit rejected: %d", rec.Code)
	}
}

func TestExplainBodyTooLarge(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(nil, true, explain.Fake{}).Explain(rec, explainReq(strings.Repeat("a", 130<<10)))
	if rec.Code != 413 {
		t.Fatalf("%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	handler(nil, true, explain.Fake{}).Explain(rec, as("user", httptest.NewRequest("POST", "/x", strings.NewReader("{kaputt"))))
	if rec.Code != 400 {
		t.Fatalf("malformed %d", rec.Code)
	}
}

func TestMissingClaimsUnauthorized(t *testing.T) {
	h := handler(fakeExtractor{text: sample}, true, explain.Fake{})
	rec := httptest.NewRecorder()
	h.Preview(rec, upload(t, "text", sample, nil))
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), "Bitte anmelden.") {
		t.Fatalf("preview %d %s", rec.Code, rec.Body)
	}
	body, _ := json.Marshal(map[string]string{"text": sample})
	rec = httptest.NewRecorder()
	h.Explain(rec, httptest.NewRequest("POST", "/x", bytes.NewReader(body)))
	if rec.Code != 401 {
		t.Fatalf("explain %d", rec.Code)
	}
}

type mustNotExtract struct{ t *testing.T }

func (m mustNotExtract) Extract(context.Context, []byte) (string, extract.Kind, error) {
	m.t.Error("extractor called")
	return "", extract.Text, nil
}

type mustNotQuota struct{ t *testing.T }

func (m mustNotQuota) TakeQuota(context.Context, string, time.Time) (int, bool, error) {
	m.t.Error("quota called")
	return 0, false, nil
}

func TestDemoDoesNotTouchExtractorOrQuota(t *testing.T) {
	h := handler(mustNotExtract{t}, true, explain.Fake{})
	h.Quota = mustNotQuota{t}
	rec := httptest.NewRecorder()
	h.Preview(rec, as("demo", upload(t, "", "", []byte{0x89, 'P', 'N', 'G'})))
	if rec.Code != 403 {
		t.Fatalf("preview %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.Explain(rec, as("demo", explainReq(sample)))
	if rec.Code != 403 {
		t.Fatalf("explain %d", rec.Code)
	}
}

type orderRecorder struct{ calls []string }

func (o *orderRecorder) TakeQuota(context.Context, string, time.Time) (int, bool, error) {
	o.calls = append(o.calls, "quota")
	return 1, true, nil
}

func (o *orderRecorder) Explain(ctx context.Context, text string) (explain.Explanation, error) {
	o.calls = append(o.calls, "explain")
	return explain.Fake{}.Explain(ctx, text)
}

func TestExplainQuotaOrder(t *testing.T) {
	o := &orderRecorder{}
	h := handler(nil, true, o)
	h.Quota = o
	h.Explain(httptest.NewRecorder(), explainReq("kurz"))
	if len(o.calls) != 0 {
		t.Fatalf("invalid length took quota or explained: %v", o.calls)
	}
	rec := httptest.NewRecorder()
	h.Explain(rec, explainReq(sample))
	if rec.Code != 200 || strings.Join(o.calls, ",") != "quota,explain" {
		t.Fatalf("%d %v", rec.Code, o.calls)
	}
}

func TestPreviewGenericExtractionError(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fakeExtractor{err: errors.New("tesseract exit status 1 /tmp/secret")}, true, nil).
		Preview(rec, as("user", upload(t, "", "", []byte{0x89, 'P', 'N', 'G'})))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "tesseract") || strings.Contains(rec.Body.String(), "/tmp") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestLogsNeverContainLetterText(t *testing.T) {
	var logs bytes.Buffer
	h := handler(fakeExtractor{text: sample}, true, failingExplainer{errors.New("upstream failure")})
	h.Log = slog.New(slog.NewTextHandler(&logs, nil))
	png := []byte{0x89, 'P', 'N', 'G'}

	h.Preview(httptest.NewRecorder(), as("user", upload(t, "text", sample, nil)))
	h.Preview(httptest.NewRecorder(), as("user", upload(t, "", "", png)))
	h.Extractor = fakeExtractor{text: sample, err: extract.ErrNoText}
	h.Preview(httptest.NewRecorder(), as("user", upload(t, "", "", png)))
	h.Extractor = fakeExtractor{text: sample, err: errors.New("tool exit status 1")}
	h.Preview(httptest.NewRecorder(), as("user", upload(t, "", "", png)))
	h.Explain(httptest.NewRecorder(), explainReq(sample))
	h.Quota = errQuota{}
	h.Explain(httptest.NewRecorder(), explainReq(sample))

	if logs.Len() == 0 {
		t.Fatal("expected log output from the error paths")
	}
	for _, s := range []string{"Benali", "Lindenweg"} {
		if strings.Contains(logs.String(), s) {
			t.Fatalf("log contains %q: %s", s, logs.String())
		}
	}
}
