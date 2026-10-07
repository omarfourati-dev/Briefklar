// Package explain turns a redacted letter into a structured explanation. The Explainer interface lets
// tests and CI use a fake instead of the paid API.
package explain

import (
	"context"
	"errors"
	"time"
)

type Texts struct {
	DE string `json:"de"`
	EN string `json:"en"`
	FR string `json:"fr"`
	AR string `json:"ar"`
}

type Explanation struct {
	Authority    string   `json:"authority"`
	LetterType   string   `json:"letterType"`
	Deadline     *string  `json:"deadline"` // YYYY-MM-DD or nil
	DeadlineText string   `json:"deadlineText"`
	Urgency      string   `json:"urgency"` // red | yellow | green
	Summary      Texts    `json:"summary"`
	Actions      []Texts  `json:"actions"`
	ReplyDraft   string   `json:"replyDraft"`
	MissingInfo  []string `json:"missingInfo"`
}

type Explainer interface {
	Explain(ctx context.Context, redactedText string) (Explanation, error)
}

var (
	ErrUpstream = errors.New("explanation service failed")
	ErrTimeout  = errors.New("explanation service timed out")
)

// normalize repairs values the model may get wrong, so the UI can rely on them.
func normalize(e *Explanation) {
	switch e.Urgency {
	case "red", "yellow", "green":
	default:
		e.Urgency = "yellow"
	}
	if e.Deadline != nil {
		if _, err := time.Parse("2006-01-02", *e.Deadline); err != nil {
			e.Deadline = nil
		}
	}
	if e.Actions == nil {
		e.Actions = []Texts{}
	}
	if e.MissingInfo == nil {
		e.MissingInfo = []string{}
	}
}
