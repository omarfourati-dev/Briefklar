// Package redact replaces personal data in a letter with placeholders like [NAME_1] before the text
// leaves the server. Rules are deliberately conservative regexes; the UI shows the result so the user
// can redact more by hand.
package redact

import (
	"regexp"
	"slices"
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
	valid func(string) bool // optional extra check, e.g. IBAN checksum
}

// label is "Label", optional colon, spaces/tabs – never a line break, so a value from the next line is not taken.
const label = `[ \t]*:?[ \t]*`

var rules = []rule{
	{kind: Email, re: regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)},
	{kind: IBAN, re: regexp.MustCompile(`\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{4}){2,7}(?: ?[A-Z0-9]{1,4})?\b`), valid: validIBAN},
	{kind: TaxID, re: regexp.MustCompile(`(?i)(?:steuer-?id(?:entifikationsnummer)?|identifikationsnummer|idnr\.?)` + label + `((?:\d ?){10}\d)`), group: 1},
	{kind: TaxNumber, re: regexp.MustCompile(`(?i)steuernummer` + label + `(\d[\d/ ]{8,14}\d)`), group: 1},
	{kind: Birthdate, re: regexp.MustCompile(`(?i)(?:geb\.|geboren(?: am)?|geburtsdatum)` + label + `(\d{1,2}\.\d{1,2}\.\d{4})`), group: 1},
	{kind: Phone, re: regexp.MustCompile(`(?i)(?:tel(?:efon)?\.?|fax|mobil)` + label + `(\+?\d[\d /()-]{5,}\d)`), group: 1},
	{kind: Phone, re: regexp.MustCompile(`\+49[\d /()-]{6,}\d`)},
	{kind: Reference, re: regexp.MustCompile(`(?i)(?:aktenzeichen|az\.|gesch(?:ä|ae)ftszeichen|unser zeichen|ihr zeichen|kundennummer|beitragsnummer|kassenzeichen|vorgangsnummer|versichertennummer|mitgliedsnummer)` +
		label + `([A-Za-z0-9./-]*\d[A-Za-z0-9./-]*(?: [A-Za-z0-9./-]*\d[A-Za-z0-9./-]*)*)`), group: 1},
	{kind: Street, re: regexp.MustCompile(`[A-ZÄÖÜ][a-zäöüß-]+(?:straße|strasse|str\.|weg|allee|platz|gasse|ring|damm|ufer) \d+ ?[a-z]?\b`)},
	{kind: Place, re: regexp.MustCompile(`\b\d{5} (?:Bad |Sankt |St\. )?[A-ZÄÖÜ][a-zäöüß]+(?:-[A-ZÄÖÜ]?[a-zäöüß]+)*(?: (?:an der|am|im|ob der) [A-ZÄÖÜ][a-zäöüß]+)?`)},
	// Address field: "Herrn"/"Frau" followed by the name on the same or the next line.
	{kind: Name, re: regexp.MustCompile(`(?m)^(?:Herrn|Herr|Frau)(?:[ \t]+|\r?\n)((?:Dr\. )?[A-ZÄÖÜ][\p{L}-]+(?: [A-ZÄÖÜ][\p{L}-]+){0,3})[ \t]*$`), group: 1},
	{kind: Name, re: regexp.MustCompile(`Sehr geehrte[r]? (?:Herr|Frau) ((?:Dr\. )?[A-ZÄÖÜ][\p{L}-]+(?: [A-ZÄÖÜ][\p{L}-]+)?)`), group: 1},
}

var existingPlaceholder = regexp.MustCompile(`\[([A-Z_]+?)_(\d+)\]`)

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
		if utf8.RuneCountInString(v) < 3 || seen[v] || existingPlaceholder.MatchString(v) {
			return
		}
		seen[v] = true
		hits = append(hits, hit{k, v})
	}
	for _, r := range rules {
		for _, m := range r.re.FindAllStringSubmatch(text, -1) {
			v := m[r.group]
			if r.valid != nil && !r.valid(v) {
				continue
			}
			add(r.kind, v)
			if r.kind == Name {
				// "Herr Benali" later in the text must disappear as well
				for _, part := range strings.Fields(v) {
					if part != "Dr." {
						add(Name, part)
					}
				}
			}
		}
	}

	sort.SliceStable(hits, func(i, j int) bool {
		return strings.Index(text, hits[i].value) < strings.Index(text, hits[j].value)
	})
	counters := map[Kind]int{}
	for _, m := range existingPlaceholder.FindAllStringSubmatch(text, -1) {
		if n, _ := strconv.Atoi(m[2]); n > counters[Kind(m[1])] {
			counters[Kind(m[1])] = n
		}
	}
	findings := make([]Finding, 0, len(hits))
	for _, h := range hits {
		counters[h.kind]++
		findings = append(findings, Finding{
			Placeholder: "[" + string(h.kind) + "_" + strconv.Itoa(counters[h.kind]) + "]",
			Kind:        h.kind,
			Value:       h.value,
		})
	}

	// Longest values first, so "Karim Benali" is replaced as a whole before "Benali".
	order := slices.Clone(findings)
	sort.SliceStable(order, func(i, j int) bool { return len(order[i].Value) > len(order[j].Value) })
	out := text
	for _, f := range order {
		out = replaceWord(out, f.Value, f.Placeholder)
	}
	return Result{Text: out, Findings: findings}
}

// Restore puts the original values back. It runs where the user sees the result.
func Restore(text string, findings []Finding) string {
	for _, f := range findings {
		text = strings.ReplaceAll(text, f.Placeholder, f.Value)
	}
	return text
}

// replaceWord replaces whole-word occurrences only (Go's \b knows no umlauts, so boundaries are checked by hand).
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

func validIBAN(s string) bool {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	mod := 0
	for _, c := range s[4:] + s[:4] {
		switch {
		case c >= '0' && c <= '9':
			mod = (mod*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			mod = (mod*100 + int(c-'A'+10)) % 97
		default:
			return false
		}
	}
	return mod == 1
}
