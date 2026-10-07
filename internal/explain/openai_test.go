package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
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

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body) // lets the server notice the client hanging up
		select {
		case <-time.After(5 * time.Second):
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

func TestProviderErrorIsNotLeaked(t *testing.T) {
	srv := server(t, 401, `{"error":{"message":"Incorrect API key provided: ECHOED-KEY-FRAGMENT"}}`, nil)
	defer srv.Close()

	var logs bytes.Buffer
	c := client(srv.URL)
	c.Log = slog.New(slog.NewTextHandler(&logs, nil))
	_, err := c.Explain(context.Background(), "text")
	if !errors.Is(err, ErrUpstream) || strings.Contains(err.Error(), "ECHOED-KEY-FRAGMENT") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(logs.String(), "ECHOED-KEY-FRAGMENT") || logs.Len() == 0 {
		t.Fatalf("log = %q", logs.String())
	}
}

func rawServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, body)
	}))
}

func TestRefusalIsUpstreamError(t *testing.T) {
	srv := rawServer(`{"choices":[{"message":{"content":"","refusal":"I cannot help"}}]}`)
	defer srv.Close()
	if _, err := client(srv.URL).Explain(context.Background(), "text"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
}

func TestEmptyChoicesIsUpstreamError(t *testing.T) {
	srv := rawServer(`{"choices":[]}`)
	defer srv.Close()
	if _, err := client(srv.URL).Explain(context.Background(), "text"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
}

func TestOversizedResponseIsUpstreamError(t *testing.T) {
	padded := strings.Replace(answer, "Ausländerbehörde", strings.Repeat("a", 2<<20), 1)
	content, _ := json.Marshal(padded)
	body := `{"choices":[{"message":{"content":` + string(content) + `}}]}`
	srv := rawServer(body)
	defer srv.Close()

	start := time.Now()
	_, err := client(srv.URL).Explain(context.Background(), "text")
	if !errors.Is(err, ErrUpstream) || time.Since(start) > 2*time.Second {
		t.Fatalf("err = %v after %v", err, time.Since(start))
	}
}

func TestSchemaIsStrictCompatible(t *testing.T) {
	var walk func(path string, node any)
	walk = func(path string, node any) {
		m, ok := node.(map[string]any)
		if !ok {
			return
		}
		if m["type"] == "object" {
			if m["additionalProperties"] != false {
				t.Errorf("%s: additionalProperties must be false", path)
			}
			props, _ := m["properties"].(map[string]any)
			req, _ := m["required"].([]string)
			var keys []string
			for k := range props {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			sortedReq := append([]string(nil), req...)
			sort.Strings(sortedReq)
			if !reflect.DeepEqual(keys, sortedReq) {
				t.Errorf("%s: required %v != properties %v", path, sortedReq, keys)
			}
			for k, v := range props {
				walk(path+"."+k, v)
			}
		}
		if items, ok := m["items"]; ok {
			walk(path+"[]", items)
		}
	}
	walk("schema", schema)
}
