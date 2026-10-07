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
)

// maxResponseBytes bounds how much of a provider response is read.
const maxResponseBytes = 1 << 20

type OpenAI struct {
	HTTP    *http.Client
	BaseURL string
	APIKey  string
	Model   string
	Log     *slog.Logger
}

const systemPrompt = `Du erklärst deutsche Behördenbriefe für Menschen, die Amtsdeutsch schwer verstehen.
Persönliche Daten sind durch Platzhalter wie [NAME_1] oder [IBAN_1] ersetzt. Übernimm Platzhalter unverändert,
erfinde keine Namen, Beträge oder Daten. Eine Frist (deadline) nur angeben, wenn sie im Brief steht, im Format
YYYY-MM-DD, sonst null. urgency: red = Frist in weniger als 7 Tagen oder Mahnung/Zwangsmaßnahme, yellow = Frist
vorhanden, green = nur zur Information. summary: 2-4 einfache Sätze je Sprache (de, en, fr, ar). actions: konkrete
Schritte in allen vier Sprachen. replyDraft: höflicher, kurzer Antwortbrief auf Deutsch mit Platzhaltern.
missingInfo: was im Brief unklar ist oder fehlt (auf Deutsch). Keine Rechtsberatung.`

var texts = map[string]any{
	"type": "object", "additionalProperties": false, "required": []string{"de", "en", "fr", "ar"},
	"properties": map[string]any{"de": str, "en": str, "fr": str, "ar": str},
}

var str = map[string]any{"type": "string"}

var schema = map[string]any{
	"type": "object", "additionalProperties": false,
	"required": []string{"authority", "letterType", "deadline", "deadlineText", "urgency", "summary", "actions", "replyDraft", "missingInfo"},
	"properties": map[string]any{
		"authority":    str,
		"letterType":   str,
		"deadline":     map[string]any{"type": []string{"string", "null"}},
		"deadlineText": str,
		"urgency":      map[string]any{"type": "string", "enum": []string{"red", "yellow", "green"}},
		"summary":      texts,
		"actions":      map[string]any{"type": "array", "items": texts},
		"replyDraft":   str,
		"missingInfo":  map[string]any{"type": "array", "items": str},
	},
}

func (o *OpenAI) Explain(ctx context.Context, text string) (Explanation, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":       o.Model,
		"temperature": 0.2,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": text},
		},
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "explanation", "strict": true, "schema": schema},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		// Not wrapping err: *url.Error contains the URL.
		return Explanation{}, fmt.Errorf("%w: invalid request", ErrUpstream)
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.HTTP.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Explanation{}, ErrTimeout
		}
		// Fixed message only: the raw *url.Error contains the URL.
		o.Log.Warn("explain request failed", "reason", "network")
		return Explanation{}, ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Only the status is logged: provider bodies can echo parts of the API key.
		o.Log.Warn("explain provider error", "status", resp.StatusCode)
		return Explanation{}, fmt.Errorf("%w: status %d", ErrUpstream, resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string  `json:"content"`
				Refusal *string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&out); err != nil {
		return Explanation{}, o.badAnswer(ctx, "unreadable response")
	}
	if len(out.Choices) == 0 {
		return Explanation{}, o.badAnswer(ctx, "no choices")
	}
	if out.Choices[0].Message.Refusal != nil {
		return Explanation{}, o.badAnswer(ctx, "refusal")
	}
	var e Explanation
	if err := json.Unmarshal([]byte(out.Choices[0].Message.Content), &e); err != nil {
		return Explanation{}, o.badAnswer(ctx, "invalid json in answer")
	}
	normalize(&e)
	return e, nil
}

// badAnswer logs a fixed reason (never content) and maps a deadline hit during body reading to ErrTimeout.
func (o *OpenAI) badAnswer(ctx context.Context, reason string) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	o.Log.Warn("explain bad answer", "reason", reason)
	return ErrUpstream
}
