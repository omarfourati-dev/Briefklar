package redact

import (
	"strings"
	"testing"
)

// Fictional letter – every personal value appears in the "mustHide" list below.
const letter = `Landratsamt Musterkreis – Ausländerbehörde
Hauptstraße 12, 51643 Gummersbach

Herrn
Karim Benali
Lindenweg 7
51645 Gummersbach

Aktenzeichen: AB-2026/4711
Telefon: 02261 88-1234
E-Mail: auslaender@musterkreis.de

Sehr geehrter Herr Benali,

Ihr Aufenthaltstitel läuft am 30.11.2026 ab. Bitte reichen Sie die Unterlagen bis zum 15.11.2026 ein.
Geburtsdatum: 03.04.1995
Steuer-ID: 12 345 678 901
Steuernummer: 212/5678/9012
Bitte überweisen Sie die Gebühr auf DE89 3704 0044 0532 0130 00.
Mit freundlichen Grüßen
Ihre Ausländerbehörde`

func TestRedactsPersonalData(t *testing.T) {
	r := Redact(letter)
	mustHide := []string{"Karim Benali", "Benali", "Lindenweg 7", "51645 Gummersbach", "AB-2026/4711",
		"02261 88-1234", "auslaender@musterkreis.de", "03.04.1995", "12 345 678 901", "212/5678/9012",
		"DE89 3704 0044 0532 0130 00", "Hauptstraße 12"}
	for _, v := range mustHide {
		if strings.Contains(r.Text, v) {
			t.Errorf("%q still in redacted text", v)
		}
	}
	kinds := map[Kind]bool{}
	for _, f := range r.Findings {
		kinds[f.Kind] = true
	}
	for _, k := range []Kind{Name, Street, Place, IBAN, TaxID, TaxNumber, Reference, Birthdate, Phone, Email} {
		if !kinds[k] {
			t.Errorf("no finding of kind %s", k)
		}
	}
}

func TestKeepsDeadlinesAndAuthority(t *testing.T) {
	r := Redact(letter)
	for _, v := range []string{"30.11.2026", "15.11.2026", "Ausländerbehörde", "Landratsamt Musterkreis", "Aufenthaltstitel"} {
		if !strings.Contains(r.Text, v) {
			t.Errorf("%q must stay readable for the LLM", v)
		}
	}
}

func TestSameValueGetsSamePlaceholder(t *testing.T) {
	r := Redact("Sehr geehrte Frau Yilmaz,\nFrau Yilmaz hat am Montag angerufen. Yilmaz bestätigt.")
	if strings.Contains(r.Text, "Yilmaz") {
		t.Fatalf("name left in %q", r.Text)
	}
	if n := strings.Count(r.Text, "[NAME_1]"); n != 3 {
		t.Fatalf("[NAME_1] appears %d times in %q", n, r.Text)
	}
}

func TestInvalidIBANIsKept(t *testing.T) {
	r := Redact("Konto DE00 1234 5678 9012 3456 78 ist ungültig.")
	if !strings.Contains(r.Text, "DE00 1234 5678 9012 3456 78") {
		t.Fatalf("invalid IBAN was redacted: %q", r.Text)
	}
}

func TestPlaceholdersNumberedByAppearance(t *testing.T) {
	r := Redact("Mail an a@x.de und b@x.de")
	if r.Text != "Mail an [EMAIL_1] und [EMAIL_2]" {
		t.Fatalf("got %q", r.Text)
	}
}

func TestRestoreRoundTrip(t *testing.T) {
	r := Redact(letter)
	if got := Restore(r.Text, r.Findings); got != letter {
		t.Fatalf("round trip changed the text:\n%s", got)
	}
}

func TestRedactContinuesNumbering(t *testing.T) {
	// Second pass on text that already has placeholders: new findings must not reuse [NAME_1].
	r := Redact("Sehr geehrte Frau [NAME_1],\nIhr Nachbar Herr Okafor ist informiert.\nHerr\nJonas Okafor")
	for _, f := range r.Findings {
		if f.Placeholder == "[NAME_1]" {
			t.Fatalf("placeholder collision: %+v", r.Findings)
		}
	}
	if !strings.Contains(r.Text, "[NAME_1]") || strings.Contains(r.Text, "Okafor") {
		t.Fatalf("got %q", r.Text)
	}
}

func TestEmptyText(t *testing.T) {
	r := Redact("")
	if r.Text != "" || len(r.Findings) != 0 {
		t.Fatalf("got %+v", r)
	}
}
