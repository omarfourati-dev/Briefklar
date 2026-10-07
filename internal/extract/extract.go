// Package extract turns an uploaded PDF or photo into text with poppler (pdftotext, pdftoppm) and
// Tesseract. Everything runs on this server; the files live in a temp directory that is removed
// before Extract returns.
package extract

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	_ "golang.org/x/image/webp"
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
	// ErrImageTooLarge: the photo has more pixels than we are willing to OCR.
	ErrImageTooLarge = errors.New("image too large")
)

const tempPattern = "briefklar-*"

// CleanStale removes temp directories left behind by a process that was killed mid-extraction
// (e.g. OOM kill), so letter bytes do not survive a restart. It returns the number removed.
func CleanStale(dir string) (int, error) {
	matches, err := filepath.Glob(filepath.Join(dir, tempPattern))
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, m := range matches {
		if fi, err := os.Lstat(m); err != nil || !fi.IsDir() {
			continue
		}
		if err := os.RemoveAll(m); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

// minLetters: below this the OCR result is noise, not a letter.
const minLetters = 40

const (
	maxPixels      = 25_000_000 // decoded size of a photo we are willing to OCR
	maxStdout      = 2 << 20
	maxStderr      = 4 << 10
	stderrKeep     = 500
	toolWaitDelay  = 2 * time.Second
	renderLongSide = "2500"
)

// ToolError reports a failed external tool. Error() carries only the tool name and exit status;
// the tool's stderr (which may echo document content) is available via Stderr() for the server log only.
type ToolError struct {
	Tool   string
	Err    error
	stderr string
}

func (e *ToolError) Error() string { return e.Tool + ": " + e.Err.Error() }
func (e *ToolError) Unwrap() error { return e.Err }

// Stderr returns the tool's stderr, truncated to 500 bytes.
func (e *ToolError) Stderr() string {
	if len(e.stderr) > stderrKeep {
		return e.stderr[:stderrKeep]
	}
	return e.stderr
}

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
	if kind == Image {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return "", kind, ErrUnsupported
		}
		if cfg.Width*cfg.Height > maxPixels {
			return "", kind, ErrImageTooLarge
		}
	}
	dir, err := os.MkdirTemp("", tempPattern)
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
	last := strconv.Itoa(max(t.MaxPages, 1))
	text, textErr := t.run(ctx, nil, t.PdfToText, "-layout", "-l", last, in, "-")
	if textErr == nil && letters(text) >= minLetters {
		return text, nil
	}
	// No (usable) text layer: scanned PDF. Render the first pages and read them like photos.
	ocr, err := t.ocrPages(ctx, dir, in, last)
	if err != nil {
		return "", errors.Join(textErr, err)
	}
	if letters(ocr) >= minLetters || letters(ocr) >= letters(text) {
		return ocr, nil
	}
	return text, nil
}

func (t *Tools) ocrPages(ctx context.Context, dir, in, last string) (string, error) {
	if _, err := t.run(ctx, nil, t.PdfToPPM, "-r", "200", "-scale-to", renderLongSide, "-png", "-l", last, in, filepath.Join(dir, "page")); err != nil {
		return "", err
	}
	pages, _ := filepath.Glob(filepath.Join(dir, "page*.png"))
	sort.Strings(pages)
	var all []string
	for _, p := range pages {
		page, err := t.tesseract(ctx, p)
		if err != nil {
			return "", err
		}
		all = append(all, page)
	}
	return strings.Join(all, "\n"), nil
}

func (t *Tools) tesseract(ctx context.Context, file string) (string, error) {
	return t.run(ctx, []string{"OMP_THREAD_LIMIT=1"}, t.Tesseract, file, "stdout", "-l", "deu", "--psm", "3")
}

func (t *Tools) image(ctx context.Context, dir string, data []byte) (string, error) {
	in := filepath.Join(dir, "in.img")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return "", err
	}
	return t.tesseract(ctx, in)
}

// limitedWriter keeps the first n bytes and silently discards the rest.
type limitedWriter struct {
	buf bytes.Buffer
	n   int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if room := w.n - w.buf.Len(); room > 0 {
		w.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

var _ io.Writer = (*limitedWriter)(nil)

// run returns stdout; stderr only goes into the ToolError (for the server log), never into its message.
func (t *Tools) run(ctx context.Context, env []string, name string, args ...string) (string, error) {
	stdout, stderr := &limitedWriter{n: maxStdout}, &limitedWriter{n: maxStderr}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = toolWaitDelay
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return "", &ToolError{Tool: name, Err: err, stderr: strings.TrimSpace(stderr.buf.String())}
	}
	return stdout.buf.String(), nil
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
