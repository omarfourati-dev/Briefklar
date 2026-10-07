// Package extract turns an uploaded PDF or photo into text with poppler (pdftotext, pdftoppm) and
// Tesseract. Everything runs on this server; the files live in a temp directory that is removed
// before Extract returns.
package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Kind string

const (
	Text  Kind = "text"
	PDF   Kind = "pdf"
	Image Kind = "image"
)

var (
	ErrUnsupported = errors.New("unsupported file type")
	ErrNoText      = errors.New("no readable text")
)

// minLetters: below this the OCR result is noise, not a letter.
const minLetters = 40

func Detect(data []byte) (Kind, error) {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return PDF, nil
	case bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}), bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return Image, nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return Image, nil
	}
	return "", ErrUnsupported
}

type Tools struct {
	Tesseract string
	PdfToText string
	PdfToPPM  string
	MaxPages  int
}

func Default() *Tools {
	return &Tools{Tesseract: "tesseract", PdfToText: "pdftotext", PdfToPPM: "pdftoppm", MaxPages: 3}
}

func (t *Tools) Extract(ctx context.Context, data []byte) (string, Kind, error) {
	kind, err := Detect(data)
	if err != nil {
		return "", "", err
	}
	dir, err := os.MkdirTemp("", "briefklar-*")
	if err != nil {
		return "", kind, err
	}
	defer os.RemoveAll(dir)

	var text string
	switch kind {
	case PDF:
		text, err = t.pdf(ctx, dir, data)
	case Image:
		text, err = t.image(ctx, dir, data)
	}
	if err != nil {
		return "", kind, err
	}
	text = strings.TrimSpace(text)
	if letters(text) < minLetters {
		return text, kind, ErrNoText
	}
	return text, kind, nil
}

func (t *Tools) pdf(ctx context.Context, dir string, data []byte) (string, error) {
	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return "", err
	}
	last := strconv.Itoa(t.MaxPages)
	text, err := run(ctx, t.PdfToText, "-layout", "-l", last, in, "-")
	if err == nil && letters(text) >= minLetters {
		return text, nil
	}
	// No text layer: scanned PDF. Render the first pages and read them like photos.
	if _, err := run(ctx, t.PdfToPPM, "-r", "200", "-png", "-l", last, in, filepath.Join(dir, "page")); err != nil {
		return "", err
	}
	pages, _ := filepath.Glob(filepath.Join(dir, "page*.png"))
	sort.Strings(pages)
	var all []string
	for _, p := range pages {
		page, err := run(ctx, t.Tesseract, p, "stdout", "-l", "deu", "--psm", "3")
		if err != nil {
			return "", err
		}
		all = append(all, page)
	}
	return strings.Join(all, "\n"), nil
}

func (t *Tools) image(ctx context.Context, dir string, data []byte) (string, error) {
	in := filepath.Join(dir, "in.img")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return "", err
	}
	return run(ctx, t.Tesseract, in, "stdout", "-l", "deu", "--psm", "3")
}

// run returns stdout; stderr only goes into the error (for the server log), never to the client.
func run(ctx context.Context, name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func letters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}
