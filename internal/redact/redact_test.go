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

func TestIBANShapedStringsAreRedacted(t *testing.T) {
	r := Redact("Konto DE00 1234 5678 9012 3456 78 und de89 3704 0044 0532 0130 00 sind genannt.")
	for _, v := range []string{"DE00 1234 5678 9012 3456 78", "de89 3704 0044 0532 0130 00"} {
		if strings.Contains(r.Text, v) {
			t.Errorf("%q still in %q", v, r.Text)
		}
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

func assertFindingsInText(t *testing.T, r Result) {
	t.Helper()
	for _, f := range r.Findings {
		if !strings.Contains(r.Text, f.Placeholder) {
			t.Errorf("finding %+v has no placeholder in %q", f, r.Text)
		}
	}
}

func TestNoCrossPassCollision(t *testing.T) {
	r1 := Redact("Herrn\nAnna Berg\n\nIhr Antrag ist da.")
	r2 := Redact(r1.Text + "\nHerr Jonas Okafor rief an.")
	assertFindingsInText(t, r1)
	assertFindingsInText(t, r2)
	if len(r2.Findings) == 0 {
		t.Fatalf("second pass found nothing: %q", r2.Text)
	}
	for _, f2 := range r2.Findings {
		for _, f1 := range r1.Findings {
			if f1.Placeholder == f2.Placeholder {
				t.Errorf("collision %s", f1.Placeholder)
			}
		}
	}
}

func TestFindingsAlwaysInText(t *testing.T) {
	assertFindingsInText(t, Redact(letter))
}

func TestUnicodeEmail(t *testing.T) {
	r := Redact("Schreiben Sie an max.müller@web.de")
	if strings.Contains(r.Text, "müller") || strings.Contains(r.Text, "web.de") {
		t.Fatalf("got %q", r.Text)
	}
}

func TestValueGlueToLetters(t *testing.T) {
	r := Redact("Tel.: 02261 88-1234Fax")
	if strings.Contains(r.Text, "02261") {
		t.Fatalf("got %q", r.Text)
	}
}

func TestNameVariants(t *testing.T) {
	cases := []struct {
		in   string
		hide []string
	}{
		{"Herrn\nŞahin Çelik\nMusterweg 1", []string{"Şahin", "Çelik"}},
		{"Landratsamt\r\n\r\nHerrn\r\nKarim Benali\r\nLindenweg 7\r\n51645 Gummersbach\r\n\r\nSehr geehrter Herr Benali,\r\n", []string{"Karim", "Benali", "Lindenweg 7"}},
		{"Herrn Karim Benali, Lindenweg 7", []string{"Karim", "Benali", "Lindenweg"}},
		{"Sehr geehrter Herr Li,\nIhr Antrag", []string{"Li"}},
		{"Frau Dr. med. Leila El Amrani\nMusterweg 1", []string{"Leila", "Amrani"}},
		{"Herrn Jan van der Berg\nMusterweg 1", []string{"Jan", "Berg"}},
		{"Familie Nguyen\nMusterweg 1", []string{"Nguyen"}},
		{"Herrn und Frau Okafor\nMusterweg 1", []string{"Okafor"}},
	}
	for _, c := range cases {
		r := Redact(c.in)
		for _, h := range c.hide {
			if strings.Contains(r.Text, h) {
				t.Errorf("%q: %q still in %q", c.in, h, r.Text)
			}
		}
		assertFindingsInText(t, r)
	}
}

func TestSalutationDoesNotCaptureFollowingWord(t *testing.T) {
	r := Redact("Sehr geehrte Frau Yilmaz Sie haben recht")
	if !strings.Contains(r.Text, "Sie haben recht") || strings.Contains(r.Text, "Yilmaz") {
		t.Fatalf("got %q", r.Text)
	}
}

func TestLabelsAndStreets(t *testing.T) {
	cases := []struct{ in, hide string }{
		{"Telefonnummer: 02261 88-1234", "02261 88-1234"},
		{"Tel.-Nr.: 02261 88-1234", "02261 88-1234"},
		{"Rufnummer 02261 88-1234", "02261 88-1234"},
		{"Handy: 0171 1234567", "0171 1234567"},
		{"Fax: 02261 88-9999", "02261 88-9999"},
		{"Mobil: 0171 1234567", "0171 1234567"},
		{"geb. 03.04.1995", "03.04.1995"},
		{"geb. am 03.04.1995", "03.04.1995"},
		{"geboren am 03.04.1995", "03.04.1995"},
		{"Geb.-Datum: 03.04.1995", "03.04.1995"},
		{"Steuer-Nr.: 212/5678/9012", "212/5678/9012"},
		{"St.-Nr. 212/5678/9012", "212/5678/9012"},
		{"Versicherungsnummer: 12345678A", "12345678A"},
		{"Kunden-Nr.: 4711-22", "4711-22"},
		{"BG-Nummer: 12/345", "12/345"},
		{"Antragsnummer: AN-2026-0815", "AN-2026-0815"},
		{"Wohnt in der Kölner Straße 12", "Kölner Straße 12"},
		{"Berliner Platz 3", "Berliner Platz 3"},
		{"Am Markt 4", "Am Markt 4"},
		{"An der Kirche 5", "An der Kirche 5"},
		{"Hauptstr.12", "Hauptstr.12"},
	}
	for _, c := range cases {
		r := Redact(c.in)
		if strings.Contains(r.Text, c.hide) {
			t.Errorf("%q: %q still in %q", c.in, c.hide, r.Text)
		}
		assertFindingsInText(t, r)
	}
}

func TestNoFalsePositives(t *testing.T) {
	for _, in := range []string{
		"Hotel 1234567 Titel 99887766 Automobil 5551234",
		"Am Montag 5 Tage, Im Januar 2026 wird entschieden.",
	} {
		if r := Redact(in); r.Text != in {
			t.Errorf("changed %q to %q", in, r.Text)
		}
	}
}
