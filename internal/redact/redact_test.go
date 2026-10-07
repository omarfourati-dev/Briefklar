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
	// "von der" ends a name in running text; the authority must not become a name part.
	r = Redact("Landkreis Oberberg – Ausländerbehörde\nMoltkestraße 42, 51643 Gummersbach\n\n" +
		"Frau Schmidt von der Ausländerbehörde hat Ihren Antrag geprüft. Ihre Ausländerbehörde")
	if strings.Contains(r.Text, "Schmidt") {
		t.Errorf("Schmidt still in %q", r.Text)
	}
	if n := strings.Count(r.Text, "Ausländerbehörde"); n != 3 {
		t.Errorf("Ausländerbehörde appears %d times (want 3) in %q", n, r.Text)
	}
	if !strings.Contains(r.Text, "Landkreis Oberberg – Ausländerbehörde") {
		t.Errorf("letterhead changed: %q", r.Text)
	}
}

func TestAddressBlockWithoutSalutation(t *testing.T) {
	r := Redact("Karim Benali\nLindenweg 7\n51645 Gummersbach\n\nIhr Antrag vom 01.10.2026")
	for _, h := range []string{"Karim", "Benali", "Lindenweg 7", "51645 Gummersbach"} {
		if strings.Contains(r.Text, h) {
			t.Errorf("%q still in %q", h, r.Text)
		}
	}
	assertFindingsInText(t, r)
	r = Redact("Dr. Anna Lang\r\nAm Markt 4\r\n51643 Gummersbach\r\n")
	if strings.Contains(r.Text, "Anna") || strings.Contains(r.Text, "Lang") {
		t.Errorf("name with title still in %q", r.Text)
	}
	for _, in := range []string{
		"Finanzamt Musterstadt\nHauptstraße 12\n12345 Musterstadt",
		"Jobcenter Oberberg\nMoltkestraße 42\n51643 Gummersbach",
		"Landkreis Oberberg\nMoltkestraße 42\n51643 Gummersbach",
		"Familienkasse Nordrhein-Westfalen\nHauptstraße 12\n12345 Musterstadt",
		"Muster Service GmbH\nHauptstraße 12\n12345 Musterstadt",
	} {
		first := strings.SplitN(in, "\n", 2)[0]
		if r := Redact(in); !strings.Contains(r.Text, first) {
			t.Errorf("authority %q redacted: %q", first, r.Text)
		}
	}
	// only a real address block (name line, street, place) triggers the rule
	for _, in := range []string{"Karim Benali\nLindenweg 7\nIhr Antrag", "Karim Benali\n51645 Gummersbach"} {
		if r := Redact(in); !strings.Contains(r.Text, "Karim Benali") {
			t.Errorf("not an address block, but redacted: %q", r.Text)
		}
	}
	// surnames that merely contain an authority word are persons
	for _, c := range []struct{ in, hide string }{
		{"Max Neustadt\nLindenweg 7\n51645 Gummersbach", "Neustadt"},
		{"Anna Kassebaum\nLindenweg 7\n51645 Gummersbach", "Kassebaum"},
	} {
		if r := Redact(c.in); strings.Contains(r.Text, c.hide) {
			t.Errorf("%q still in %q", c.hide, r.Text)
		}
	}
}

func TestUnlistedAuthoritiesInAddressBlockStayReadable(t *testing.T) {
	cases := []struct{ head, street, place, running string }{
		{"Deutsche Rentenversicherung Bund", "Ruhrstraße 2", "10709 Berlin", "Die Deutsche Rentenversicherung teilt mit."},
		{"Gemeinde Engelskirchen", "Engelsplatz 4", "51766 Engelskirchen", "Die Gemeinde Engelskirchen teilt mit."},
		{"Bezirksregierung Köln", "Zeughausstraße 2", "50667 Köln", "Die Bezirksregierung teilt mit."},
		{"Wohngeldstelle Oberberg", "Moltkestraße 42", "51643 Gummersbach", "Die Wohngeldstelle Oberberg teilt mit."},
	}
	for _, c := range cases {
		r := Redact(c.head + "\n" + c.street + "\n" + c.place + "\n\n" + c.running)
		if !strings.Contains(r.Text, c.head) || !strings.Contains(r.Text, c.running) {
			t.Errorf("authority redacted: %q", r.Text)
		}
	}
	// a name found only by the address-block rule is redacted as a whole line, its parts stay untouched elsewhere
	r := Redact("Ruth Bund\nLindenweg 7\n51645 Gummersbach\n\nDie Rentenversicherung Bund teilt mit.")
	if strings.Contains(r.Text, "Ruth Bund") || !strings.Contains(r.Text, "Rentenversicherung Bund teilt mit.") {
		t.Errorf("got %q", r.Text)
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
		{"Versicherungsnummer: 12 345678 B 901", "B 901"},
		{"Versicherungsnummer: 12 345678 B 901\nIhr Antrag", "901"},
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

func TestPrivateUseCharactersInInput(t *testing.T) {
	forged := ""
	for _, in := range []string{
		"Bullet  text, Mail a@x.de",
		"Mail a@x.de " + forged + " und " + forged,
		"Herrn\nKarim Benali\n",
	} {
		r := Redact(in) // must not panic
		if strings.Contains(in, forged) && strings.Count(r.Text, forged) != strings.Count(in, forged) {
			t.Errorf("forged sequence rewritten: %q", r.Text)
		}
		if got := Restore(r.Text, r.Findings); got != in {
			t.Errorf("round trip changed %q to %q", in, got)
		}
		assertFindingsInText(t, r)
	}
}

func TestRunningTextTakesOnlyOneWord(t *testing.T) {
	r := Redact("Wir stellen fest, dass Herr Benali Kindergeld bezieht. Das Kindergeld wird gezahlt. Für Ihre Familie Leistungen beantragt.")
	if strings.Contains(r.Text, "Benali") || strings.Count(r.Text, "Kindergeld") != 2 || !strings.Contains(r.Text, "Leistungen") {
		t.Fatalf("got %q", r.Text)
	}
}

func TestCapitalisedParticlesAndInitials(t *testing.T) {
	cases := []struct {
		in   string
		hide []string
	}{
		{"Herrn\nJan Van Der Berg\nMusterweg 1", []string{"Jan", "Van", "Der", "Berg"}},
		{"Herrn\nMaria De Souza\nMusterweg 1", []string{"Maria", "Souza"}},
		{"Herrn\nA. Müller\nMusterweg 1", []string{"A.", "Müller"}},
		{"Sehr geehrter Herr A. Müller,\nIhr Antrag", []string{"A.", "Müller"}},
		{"Herrn\nProf. Dr. Anna Lang\nMusterweg 1", []string{"Anna", "Lang"}},
		{"Herrn\nPeter von Stein\nMusterweg 1", []string{"Peter", "Stein"}},
		{"Herrn\nOmar al Rashid\nMusterweg 1", []string{"Omar", "Rashid"}},
		{"Frau\nAna de Souza\nMusterweg 1", []string{"Ana", "Souza"}},
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

func TestLetterPhrasesAreNotStreets(t *testing.T) {
	for _, in := range []string{"In der Anlage 2 finden Sie", "Im Absatz 3 steht", "Am Ende 3 Wochen später", "Der Arbeitsweg 12 km ist lang"} {
		if r := Redact(in); r.Text != in {
			t.Errorf("changed %q to %q", in, r.Text)
		}
	}
	for _, c := range []struct{ in, hide string }{
		{"Am Markt 4, 51643 Gummersbach", "Am Markt 4"},
		{"An der Kirche 5, 51643 Gummersbach", "An der Kirche 5"},
	} {
		if r := Redact(c.in); strings.Contains(r.Text, c.hide) {
			t.Errorf("%q still in %q", c.hide, r.Text)
		}
	}
}

func TestNumbersNotReplacedInsideOtherNumbers(t *testing.T) {
	r := Redact("Az.: 123\nBetrag 1234,00 EUR")
	if !strings.Contains(r.Text, "1234,00 EUR") || strings.Contains(r.Text, "Az.: 123\n") {
		t.Fatalf("got %q", r.Text)
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
