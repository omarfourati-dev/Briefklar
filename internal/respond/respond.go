// Package respond writes JSON responses and RFC 9457 problem details.
package respond

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Problem(w http.ResponseWriter, status int, title, detail string) {
	ProblemExtra(w, status, title, detail, nil)
}

// ProblemExtra adds extension members (RFC 9457 §3.2), e.g. the recognized text of an unreadable photo.
func ProblemExtra(w http.ResponseWriter, status int, title, detail string, extra map[string]any) {
	body := map[string]any{"type": "about:blank", "title": title, "status": status}
	if detail != "" {
		body["detail"] = detail
	}
	for k, v := range extra {
		body[k] = v
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
