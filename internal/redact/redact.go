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
	valid func(string) bool // optional extra check on the value
}

const (
	// label is "Label", optional colon, spaces/tabs – never a line break, so a value from the next line is not taken.
	label = `[ \t]*:?[ \t]*`

	nameWord = `\p{Lu}[\p{L}'’-]*`
	particle = `(?:(?:von|van|der|den|de|del|el|al|bin|ibn|zu|zur) )`
	title    = `(?:(?:Prof|Dr|med|jur|Dipl|Ing)\.-?[ \t]*)*`
	salut    = `(?:Herrn|Herr|Frau|Familie)`

	houseNo      = `\d{1,3}(?: ?[a-z])?\b`
	streetEnd    = `(?:straße|strasse|str\.|weg|allee|platz|gasse|ring|damm|ufer)`
	streetEndCap = `(?:Straße|Strasse|Str\.|Weg|Allee|Platz|Gasse|Ring|Damm|Ufer)`
)

// person captures a name of up to 1+extra words, with lower-case particles allowed between words.
func person(extra int) string {
	return `(` + nameWord + `(?: ` + particle + `*` + nameWord + `){0,` + strconv.Itoa(extra) + `})`
}

var notDate = regexp.MustCompile(`(?i)\b(?:montag|dienstag|mittwoch|donnerstag|freitag|samstag|sonntag|januar|februar|märz|april|mai|juni|juli|august|september|oktober|november|dezember)\b`)

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
	// "Hauptstraße 12", "Hauptstr.12"
	{kind: Street, re: regexp.MustCompile(`\p{Lu}[\p{L}-]*` + streetEnd + `[ \t]?` + houseNo)},
	// "Kölner Straße 12", "Berliner Platz 3"
	{kind: Street, re: regexp.MustCompile(`\p{Lu}[\p{L}-]+ ` + streetEndCap + `[ \t]?` + houseNo)},
	// "Am Markt 4", "An der Kirche 5" – but not "Am Montag 5 Tage"
	{kind: Street, re: regexp.MustCompile(`(?:Am|An der|Im|Auf dem|Zum|Zur|In der|Beim) \p{Lu}[\p{L}-]+ ` + houseNo),
		valid: func(s string) bool { return !notDate.MatchString(s) }},
	{kind: Place, re: regexp.MustCompile(`\b\d{5} (?:Bad |Sankt |St\. )?\p{Lu}\p{Ll}+(?:-\p{Lu}?\p{Ll}+)*(?: (?:an der|am|im|ob der) \p{Lu}\p{Ll}+)?`)},
	// Address field: "Herrn"/"Frau"/"Familie" followed by the name on the same or the next line.
	{kind: Name, re: regexp.MustCompile(`(?m)^[ \t]*` + salut + `(?:[ \t]+|[ \t]*\r?\n[ \t]*)(?:(?:und|u\.)[ \t]+(?:Herrn|Herr|Frau)[ \t]+)?` + title + person(3) + `[ \t]*(?:,|\r?$)`), group: 1},
	{kind: Name, re: regexp.MustCompile(`(?m)Sehr geehrte(?:r)?[ \t]+` + salut + `[ \t]+` + title + person(3) + `[ \t]*(?:,|\r?$)`), group: 1},
	// "Herr Okafor ist informiert" in running text.
	{kind: Name, re: regexp.MustCompile(`\b` + salut + `[ \t]+` + title + person(2)), group: 1},
}

var particles = map[string]bool{"von": true, "van": true, "der": true, "den": true, "de": true, "del": true,
	"el": true, "al": true, "bin": true, "ibn": true, "zu": true, "zur": true}

// nameStop are capitalised words that can follow a name in running text but are not part of it.
var nameStop = map[string]bool{"Sie": true, "Ihr": true, "Ihre": true, "Ihren": true, "Ihrem": true, "Ihnen": true, "Ihrer": true,
	"Der": true, "Die": true, "Das": true, "Und": true, "Wir": true, "Er": true, "Es": true, "Bitte": true,
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

func isNamePart(w string) bool {
	r, _ := utf8.DecodeRuneInString(w)
	return unicode.IsUpper(r) && !particles[strings.ToLower(w)] && !nameStop[w]
}

var existingPlaceholder = regexp.MustCompile(`\[([A-Z_]+?)_(\d+)\]`)

// Values are swapped for private-use tokens first and numbered afterwards, so only values that really
// were substituted become findings, numbered by first appearance in the output.
const (
	tokOpen  = ''
	tokClose = ''
	tokBase  = 0xE100
	maxHits  = 0x1700
)

func token(idx int) string {
	return string(tokOpen) + string(rune(tokBase+idx)) + string(tokClose)
}

// Redact finds personal data and replaces it with placeholders. Numbering continues after placeholders
// that are already in the text, so a second pass (or manual redactions) never reuses a number.
func Redact(text string) Result {
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
			if r.kind == Name {
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
		tok := token(idx)
		for i := range segs {
			if segs[i].fixed {
				continue
			}
			if h.kind == Name {
				segs[i].s = replaceWord(segs[i].s, h.value, tok)
			} else {
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
		i := strings.IndexRune(rest, tokOpen)
		if i < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:i])
		r, w := utf8.DecodeRuneInString(rest[i+len(string(tokOpen)):])
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
		rest = rest[i+len(string(tokOpen))+w+len(string(tokClose)):]
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

// replaceWord replaces whole-word occurrences only (Go's \b knows no umlauts, so boundaries are checked by hand).
// Used for names, which also occur as parts of other words.
func replaceWord(text, word, repl string) string {
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
		if isWordRune(before) || isWordRune(after) {
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
