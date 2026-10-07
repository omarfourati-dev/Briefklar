package explain

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const answer = `{"authority":"Ausländerbehörde","letterType":"Verlängerung","deadline":"2026-11-15","deadlineText":"bis 15.11.2026",
"urgency":"yellow","summary":{"de":"d","en":"e","fr":"f","ar":"a"},"actions":[{"de":"1","en":"1","fr":"1","ar":"1"}],
"replyDraft":"Sehr geehrte Damen und Herren, … [NAME_1]","missingInfo":[]}`

func server(t *testing.T, status int, content string, check func(*http.Request, map[string]any)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if check != nil {
			check(r, body)
		}
		w.WriteHeader(status)
		if status != 200 {
			_, _ = io.WriteString(w, content)
			return
		}
		resp := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func client(url string) *OpenAI {
	return &OpenAI{HTTP: http.DefaultClient, BaseURL: url, APIKey: "test-key", Model: "gpt-4o-mini",
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestSendsStructuredRequestAndParsesAnswer(t *testing.T) {
	srv := server(t, 200, answer, func(r *http.Request, body map[string]any) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("path=%s auth=%s", r.URL.Path, r.Header.Get("Authorization"))
		}
		rf := body["response_format"].(map[string]any)
		if rf["type"] != "json_schema" || rf["json_schema"].(map[string]any)["strict"] != true {
			t.Errorf("response_format = %v", rf)
		}
		msgs := body["messages"].([]any)
		if !strings.Contains(msgs[1].(map[string]any)["content"].(string), "[NAME_1]") {
			t.Errorf("letter text not sent")
		}
	})
	defer srv.Close()

	e, err := client(srv.URL).Explain(context.Background(), "Brief an [NAME_1]")
	if err != nil {
		t.Fatal(err)
	}
	if e.Authority != "Ausländerbehörde" || e.Deadline == nil || *e.Deadline != "2026-11-15" || e.Summary.AR != "a" || len(e.Actions) != 1 {
		t.Fatalf("got %+v", e)
	}
}

func TestProviderErrorIsNotLeaked(t *testing.T) {
	srv := server(t, 401, `{"error":{"message":"Incorrect API key provided: ECHOED-KEY-FRAGMENT"}}`, nil)
	defer srv.Close()

	_, err := client(srv.URL).Explain(context.Background(), "text")
	if !errors.Is(err, ErrUpstream) || strings.Contains(err.Error(), "ECHOED-KEY-FRAGMENT") {
		t.Fatalf("err = %v", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client(srv.URL).Explain(ctx, "text")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v", err)
	}
}

func TestNormalizesInvalidValues(t *testing.T) {
	bad := strings.Replace(strings.Replace(answer, `"2026-11-15"`, `"Mitte November"`, 1), `"yellow"`, `"purple"`, 1)
	srv := server(t, 200, bad, nil)
	defer srv.Close()

	e, err := client(srv.URL).Explain(context.Background(), "text")
	if err != nil {
		t.Fatal(err)
	}
	if e.Deadline != nil || e.Urgency != "yellow" {
		t.Fatalf("deadline=%v urgency=%s", e.Deadline, e.Urgency)
	}
}

func TestBrokenJSONIsUpstreamError(t *testing.T) {
	srv := server(t, 200, `{"authority":`, nil)
	defer srv.Close()
	if _, err := client(srv.URL).Explain(context.Background(), "text"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
}

func TestFakeKeepsPlaceholders(t *testing.T) {
	e, err := Fake{}.Explain(context.Background(), "Sehr geehrte Frau [NAME_1]")
	if err != nil || !strings.Contains(e.ReplyDraft, "[NAME_1]") || e.Deadline == nil {
		t.Fatalf("got %+v %v", e, err)
	}
}
