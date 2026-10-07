// Package redact replaces personal data in a letter with placeholders like [NAME_1] before the text
// leaves the server. Rules are deliberately conservative regexes; the UI shows the result so the user
// can redact more by hand.
package redact

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Kind string

const (
	Name      Kind = "NAME"
	Street    Kind = "STRASSE"
	Place     Kind = "ORT"
	IBAN      Kind = "IBAN"
	TaxID     Kind = "STEUER_ID"
	TaxNumber Kind = "STEUERNUMMER"
	Reference Kind = "AKTENZEICHEN"
	Birthdate Kind = "GEBURTSDATUM"
	Phone     Kind = "TELEFON"
	Email     Kind = "EMAIL"
)

type Finding struct {
	Placeholder string `json:"placeholder"`
	Kind        Kind   `json:"kind"`
	Value       string `json:"value"`
}

type Result struct {
	Text     string    `json:"text"`
	Findings []Finding `json:"findings"`
}

type rule struct {
	kind  Kind
	re    *regexp.Regexp
	group int               // capture group with the value; 0 = whole match
	clean bool              // cut the name at the first stop word
	valid func(string) bool // optional extra check on the value
}

const (
	// label is "Label", optional colon, spaces/tabs – never a line break, so a value from the next line is not taken.
	label = `[ \t]*:?[ \t]*`

	nameWord = `\p{Lu}[\p{L}'’-]*`
	initials = `(?:\p{Lu}\. )*`
	// Address block and salutation end at a comma or line end, so particles may be written in any case there.
	particleAny = `(?:(?i:von|van|der|den|de|del|el|al|bin|ibn|zu|zur) )`
	// In running text only lower-case particles link a second name part ("Herr van der Berg", not "zu Hause").
	particleLow = `(?:(?:von|van|der|de|del|el|al|bin|ibn) )`
	title       = `(?:(?:Prof|Dr|med|jur|Dipl|Ing)\.-?[ \t]*)*`
	salutAddr   = `(?:Herrn|Herr|Frau|Familie)`
	salutMid    = `(?:Herrn|Herr|Frau)`

	houseNo      = `\d{1,3}(?: ?[a-z])?\b`
	streetUnit   = `(?:[ \t]+(?:km|m|Minuten|Min|Meter|Stunden|Kilometer)\b)?`
	streetEnd    = `(?:straße|strasse|str\.|weg|allee|platz|gasse|ring|damm|ufer)`
	streetEndCap = `(?:Straße|Strasse|Str\.|Weg|Allee|Platz|Gasse|Ring|Damm|Ufer)`
)

// person captures a name of up to 1+extra words (optionally with initials in front).
func person(extra int, particle string) string {
	return `(` + initials + nameWord + `(?: ` + particle + `*` + nameWord + `){0,` + strconv.Itoa(extra) + `})`
}

// personMid captures exactly one name word, plus a part linked by a particle.
func personMid() string {
	return `(` + initials + nameWord + `(?: ` + particleLow + `+` + nameWord + `)?)`
}

var (
	notDate    = regexp.MustCompile(`(?i)\b(?:montag|dienstag|mittwoch|donnerstag|freitag|samstag|sonntag|januar|februar|märz|april|mai|juni|juli|august|september|oktober|november|dezember)\b`)
	unitSuffix = regexp.MustCompile(`[ \t](?:km|m|Minuten|Min|Meter|Stunden|Kilometer)$`)
)

// notLetterPhrase rejects "In der Anlage 2", "Am Ende 3 Wochen" and "Am Montag 5 Tage".
var notLetterPhrase = map[string]bool{"anlage": true, "absatz": true, "abschnitt": true, "ende": true, "jahr": true, "jahre": true,
	"monat": true, "monate": true, "woche": true, "wochen": true, "tag": true, "tage": true, "rahmen": true, "fall": true,
	"punkt": true, "paragraph": true, "kapitel": true, "seite": true, "zeitraum": true, "anfang": true, "stelle": true}

func validPrefixStreet(s string) bool {
	words := strings.Fields(s)
	if len(words) < 3 || notDate.MatchString(s) {
		return false
	}
	return !notLetterPhrase[strings.ToLower(words[len(words)-2])]
}

func noUnit(s string) bool { return !unitSuffix.MatchString(s) }

var rules = []rule{
	{kind: Email, re: regexp.MustCompile(`[\p{L}\p{N}._%+-]+@[\p{L}\p{N}.-]+\.\p{L}{2,}`)},
	// IBAN-shaped strings are redacted even if the checksum fails: a typo in a letter is still personal.
	{kind: IBAN, re: regexp.MustCompile(`(?i)\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{4}){2,7}(?: ?[A-Z0-9]{1,4})?\b`)},
	{kind: TaxID, re: regexp.MustCompile(`(?i)\b(?:steuer-?id(?:entifikationsnummer)?|identifikationsnummer|idnr\.?)` + label + `((?:\d ?){10}\d)`), group: 1},
	{kind: TaxNumber, re: regexp.MustCompile(`(?i)\b(?:steuernummer|steuer-nr\.?|st\.-nr\.?)` + label + `(\d[\d/ ]{8,14}\d)`), group: 1},
	{kind: Birthdate, re: regexp.MustCompile(`(?i)\b(?:geb\.-?datum|geb\.(?:[ \t]+am)?|geboren(?:[ \t]+am)?|geburtsdatum)` + label + `(\d{1,2}\.\d{1,2}\.\d{4})`), group: 1},
	{kind: Phone, re: regexp.MustCompile(`(?i)\b(?:telefon(?:nummer|-nr\.?)?|tel(?:\.-?nr\.?|\.)?|rufnummer|mobil(?:nummer)?|handy(?:nummer)?|fax)` + label + `(\+?\d[\d /()-]{5,}\d)`), group: 1},
	{kind: Phone, re: regexp.MustCompile(`\+49[\d /()-]{6,}\d`)},
	{kind: Reference, re: regexp.MustCompile(`(?i)\b(?:aktenzeichen|az\.|gesch(?:ä|ae)ftszeichen|unser zeichen|ihr zeichen|kundennummer|kunden-nr\.?|beitragsnummer|kassenzeichen|vorgangsnummer|versichertennummer|versicherungsnummer|mitgliedsnummer|bg-nummer|antragsnummer)` +
		label + `([A-Za-z0-9./-]*\d[A-Za-z0-9./-]*(?: [A-Za-z0-9./-]*\d[A-Za-z0-9./-]*)*)`), group: 1},
	// "Hauptstraße 12", "Hauptstr.12" – but not "Arbeitsweg 12 km"
	{kind: Street, re: regexp.MustCompile(`\p{Lu}[\p{L}-]*` + streetEnd + `[ \t]?` + houseNo + streetUnit), valid: noUnit},
	// "Kölner Straße 12", "Berliner Platz 3"
	{kind: Street, re: regexp.MustCompile(`\p{Lu}[\p{L}-]+ ` + streetEndCap + `[ \t]?` + houseNo + streetUnit), valid: noUnit},
	// "Am Markt 4", "An der Kirche 5" – but not "Am Montag 5 Tage" or "In der Anlage 2"
	{kind: Street, re: regexp.MustCompile(`(?:Am|An der|Im|Auf dem|Zum|Zur|In der|Beim) \p{Lu}[\p{L}-]+ ` + houseNo), valid: validPrefixStreet},
	{kind: Place, re: regexp.MustCompile(`\b\d{5} (?:Bad |Sankt |St\. )?\p{Lu}\p{Ll}+(?:-\p{Lu}?\p{Ll}+)*(?: (?:an der|am|im|ob der) \p{Lu}\p{Ll}+)?`)},
	// Address field: "Herrn"/"Frau"/"Familie" followed by the name on the same or the next line.
	{kind: Name, re: regexp.MustCompile(`(?m)^[ \t]*` + salutAddr + `(?:[ \t]+|[ \t]*\r?\n[ \t]*)(?:(?:und|u\.)[ \t]+(?:Herrn|Herr|Frau)[ \t]+)?` + title + person(3, particleAny) + `[ \t]*(?:,|\r?$)`), group: 1},
	{kind: Name, re: regexp.MustCompile(`(?m)Sehr geehrte(?:r)?[ \t]+` + salutAddr + `[ \t]+` + title + person(3, particleAny) + `[ \t]*(?:,|\r?$)`), group: 1},
	// "Herr Okafor ist informiert" in running text: exactly one name word.
	{kind: Name, re: regexp.MustCompile(`\b` + salutMid + `[ \t]+` + title + personMid()), group: 1, clean: true},
}

var particles = map[string]bool{"von": true, "van": true, "der": true, "den": true, "de": true, "del": true,
	"el": true, "al": true, "bin": true, "ibn": true, "zu": true, "zur": true}

// nameStop are capitalised words that can follow "Herr"/"Frau" in running text but are not a name.
// Particles are never stop words.
var nameStop = map[string]bool{"Sie": true, "Ihr": true, "Ihre": true, "Ihren": true, "Ihrem": true, "Ihnen": true, "Ihrer": true,
	"Die": true, "Das": true, "Und": true, "Wir": true, "Er": true, "Es": true, "Bitte": true,
	"Mit": true, "Am": true, "Im": true, "In": true, "An": true, "Auf": true, "Für": true}

// cleanName cuts the value at the first word that cannot belong to a name.
func cleanName(v string) string {
	var keep []string
	for _, w := range strings.Fields(v) {
		if nameStop[w] {
			break
		}
		keep = append(keep, w)
	}
	return strings.Join(keep, " ")
}

// isNamePart tells whether a word of a full name is worth redacting on its own (not particles, not initials).
func isNamePart(w string) bool {
	r, _ := utf8.DecodeRuneInString(w)
	return unicode.IsUpper(r) && !strings.HasSuffix(w, ".") && !particles[strings.ToLower(w)]
}

var existingPlaceholder = regexp.MustCompile(`\[([A-Z_]+?)_(\d+)\]`)

// Values are swapped for private-use tokens first and numbered afterwards, so only values that really
// were substituted become findings, numbered by first appearance in the output. The token delimiter is a
// private-use character that does not occur in the input, so no input can collide with or forge a token.
const (
	tokBase = 0xE100
	maxHits = 0x1700
)

func isPrivateUse(r rune) bool { return r >= 0xE000 && r <= 0xF8FF }

// tokenDelimiter returns a private-use rune that is not in text; text is returned without private-use
// characters in the (practically impossible) case that all of them are taken.
func tokenDelimiter(text string) (rune, string) {
	for r := rune(0xE000); r <= 0xF8FF; r++ {
		if !strings.ContainsRune(text, r) {
			return r, text
		}
	}
	return 0xE000, strings.Map(func(r rune) rune {
		if isPrivateUse(r) {
			return -1
		}
		return r
	}, text)
}

func token(delim rune, idx int) string {
	return string(delim) + string(rune(tokBase+idx)) + string(delim)
}

// Redact finds personal data and replaces it with placeholders. Numbering continues after placeholders
// that are already in the text, so a second pass (or manual redactions) never reuses a number.
func Redact(text string) Result {
	delim, text := tokenDelimiter(text)
	type hit struct {
		kind  Kind
		value string
	}
	var hits []hit
	seen := map[string]bool{}
	add := func(k Kind, v string) {
		v = strings.TrimSpace(v)
		minLen := 3
		if k == Name {
			minLen = 2
		}
		if utf8.RuneCountInString(v) < minLen || seen[v] || existingPlaceholder.MatchString(v) || len(hits) >= maxHits {
			return
		}
		seen[v] = true
		hits = append(hits, hit{k, v})
	}
	for _, r := range rules {
		for _, m := range r.re.FindAllStringSubmatch(text, -1) {
			v := m[r.group]
			if r.clean {
				v = cleanName(v)
			}
			if r.valid != nil && !r.valid(v) {
				continue
			}
			add(r.kind, v)
			if r.kind == Name {
				// "Herr Benali" later in the text must disappear as well
				for _, part := range strings.Fields(v) {
					if isNamePart(part) {
						add(Name, part)
					}
				}
			}
		}
	}

	// Text outside already existing placeholders may be changed; the placeholders themselves never.
	type seg struct {
		s     string
		fixed bool
	}
	var segs []seg
	last := 0
	for _, loc := range existingPlaceholder.FindAllStringIndex(text, -1) {
		segs = append(segs, seg{text[last:loc[0]], false}, seg{text[loc[0]:loc[1]], true})
		last = loc[1]
	}
	segs = append(segs, seg{text[last:], false})

	// Longest values first, so "Karim Benali" is replaced as a whole before "Benali".
	order := make([]int, len(hits))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return len(hits[order[a]].value) > len(hits[order[b]].value) })
	for _, idx := range order {
		h := hits[idx]
		tok := token(delim, idx)
		for i := range segs {
			if segs[i].fixed {
				continue
			}
			switch h.kind {
			case Name:
				segs[i].s = replaceBounded(segs[i].s, h.value, tok, func(b, a rune) bool { return isWordRune(b) || isWordRune(a) })
			case Phone, Reference, TaxID, TaxNumber, IBAN, Birthdate:
				// never inside a longer number ("123" in "1234,00"), but "1234Fax" is fine
				segs[i].s = replaceBounded(segs[i].s, h.value, tok, func(b, a rune) bool { return unicode.IsDigit(b) || unicode.IsDigit(a) })
			default:
				segs[i].s = strings.ReplaceAll(segs[i].s, h.value, tok)
			}
		}
	}
	var joined strings.Builder
	for _, s := range segs {
		joined.WriteString(s.s)
	}

	counters := map[Kind]int{}
	for _, m := range existingPlaceholder.FindAllStringSubmatch(text, -1) {
		if n, _ := strconv.Atoi(m[2]); n > counters[Kind(m[1])] {
			counters[Kind(m[1])] = n
		}
	}
	findings := make([]Finding, 0, len(hits))
	assigned := map[int]string{}
	var out strings.Builder
	rest := joined.String()
	for {
		i := strings.IndexRune(rest, delim)
		if i < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:i])
		r, w := utf8.DecodeRuneInString(rest[i+utf8.RuneLen(delim):])
		idx := int(r - tokBase)
		ph, ok := assigned[idx]
		if !ok {
			h := hits[idx]
			counters[h.kind]++
			ph = "[" + string(h.kind) + "_" + strconv.Itoa(counters[h.kind]) + "]"
			assigned[idx] = ph
			findings = append(findings, Finding{Placeholder: ph, Kind: h.kind, Value: h.value})
		}
		out.WriteString(ph)
		rest = rest[i+2*utf8.RuneLen(delim)+w:]
	}
	return Result{Text: out.String(), Findings: findings}
}

// Restore puts the original values back. It runs where the user sees the result.
func Restore(text string, findings []Finding) string {
	for _, f := range findings {
		text = strings.ReplaceAll(text, f.Placeholder, f.Value)
	}
	return text
}

// replaceBounded replaces occurrences of word unless blocked(rune before, rune after) says the occurrence
// touches something it must not (Go's \b knows no umlauts, so boundaries are checked by hand).
func replaceBounded(text, word, repl string, blocked func(before, after rune) bool) string {
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(text[i:], word)
		if j < 0 {
			b.WriteString(text[i:])
			return b.String()
		}
		j += i
		end := j + len(word)
		before, _ := utf8.DecodeLastRuneInString(text[:j])
		after, _ := utf8.DecodeRuneInString(text[end:])
		b.WriteString(text[i:j])
		if blocked(before, after) {
			b.WriteString(word)
		} else {
			b.WriteString(repl)
		}
		i = end
	}
}

func isWordRune(r rune) bool {
	return r != utf8.RuneError && (unicode.IsLetter(r) || unicode.IsDigit(r))
}
