package respond

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestProblemWritesRFC9457(t *testing.T) {
	rec := httptest.NewRecorder()
	ProblemExtra(rec, 422, "Kein Text erkannt", "Bitte schärfer fotografieren.", map[string]any{"recognizedText": "abc"})

	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("content type = %q", got)
	}
	if rec.Code != 422 {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"type": "about:blank", "title": "Kein Text erkannt", "status": float64(422),
		"detail": "Bitte schärfer fotografieren.", "recognizedText": "abc"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
}

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, 201, map[string]string{"a": "b"})
	if rec.Code != 201 || rec.Header().Get("Content-Type") != "application/json" || rec.Body.String() != "{\"a\":\"b\"}\n" {
		t.Fatalf("got %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
}
