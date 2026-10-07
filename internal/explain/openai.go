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
		return Explanation{}, err
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.HTTP.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Explanation{}, ErrTimeout
		}
		o.Log.Warn("explain request failed", "error", err)
		return Explanation{}, ErrUpstream
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Only the status is logged: provider bodies can echo parts of the API key.
		_, _ = io.Copy(io.Discard, resp.Body)
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
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Choices) == 0 || out.Choices[0].Message.Refusal != nil {
		return Explanation{}, ErrUpstream
	}
	var e Explanation
	if err := json.Unmarshal([]byte(out.Choices[0].Message.Content), &e); err != nil {
		return Explanation{}, ErrUpstream
	}
	normalize(&e)
	return e, nil
}
