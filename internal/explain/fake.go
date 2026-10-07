package explain

import (
	"context"
	"strings"
)

// Fake answers without any API call. Used with EXPLAINER=fake in CI, e2e tests and local development.
type Fake struct{}

func (Fake) Explain(_ context.Context, text string) (Explanation, error) {
	signature := "Ihr Name"
	if strings.Contains(text, "[NAME_1]") {
		signature = "[NAME_1]"
	}
	deadline := "2026-11-15"
	return Explanation{
		Authority:    "Musterbehörde (Testmodus)",
		LetterType:   "Testbrief",
		Deadline:     &deadline,
		DeadlineText: "bis zum 15.11.2026",
		Urgency:      "yellow",
		Summary: Texts{
			DE: "Testmodus: Die Behörde bittet Sie, Unterlagen einzureichen.",
			EN: "Test mode: the authority asks you to submit documents.",
			FR: "Mode test : l'administration vous demande d'envoyer des documents.",
			AR: "وضع الاختبار: تطلب منك الجهة تقديم المستندات.",
		},
		Actions: []Texts{{
			DE: "Unterlagen bis zum 15.11.2026 einreichen", EN: "Submit the documents by 15 Nov 2026",
			FR: "Envoyer les documents avant le 15/11/2026", AR: "قدّم المستندات قبل 15‏/11‏/2026",
		}},
		ReplyDraft:  "Sehr geehrte Damen und Herren,\n\nanbei sende ich Ihnen die angeforderten Unterlagen.\n\nMit freundlichen Grüßen\n" + signature,
		MissingInfo: []string{},
	}, nil
}
