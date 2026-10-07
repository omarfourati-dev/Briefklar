package extract

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var letterLines = []string{
	"Landratsamt Musterkreis - Auslaenderbehoerde",
	"Ihr Aufenthaltstitel laeuft am 30.11.2026 ab.",
	"Bitte reichen Sie die Unterlagen bis zum 15.11.2026 ein.",
}

func needTools(t *testing.T) {
	for _, bin := range []string{"tesseract", "pdftotext", "pdftoppm"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed (runs in scripts/go.sh and CI)", bin)
		}
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want Kind
		err  error
	}{
		{"pdf", []byte("%PDF-1.4 ..."), PDF, nil},
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A}, Image, nil},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, Image, nil},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), Image, nil},
		{"zip", []byte("PK\x03\x04"), "", ErrUnsupported},
		{"empty", nil, "", ErrUnsupported},
	}
	for _, c := range cases {
		got, err := Detect(c.data)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%s: got %q, %v", c.name, got, err)
		}
	}
}

func TestPDFTextLayer(t *testing.T) {
	needTools(t)
	text, kind, err := Default().Extract(context.Background(), makePDF(letterLines))
	if err != nil || kind != PDF {
		t.Fatalf("kind=%s err=%v", kind, err)
	}
	if !strings.Contains(text, "Aufenthaltstitel") || !strings.Contains(text, "15.11.2026") {
		t.Fatalf("text = %q", text)
	}
}

func TestImageOCR(t *testing.T) {
	needTools(t)
	dir := t.TempDir()
	pdf := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(pdf, makePDF(letterLines), 0o600); err != nil {
		t.Fatal(err)
	}
	// render the PDF to a PNG, which then has no text layer – like a photo of the letter
	if out, err := exec.Command("pdftoppm", "-r", "200", "-png", "-singlefile", pdf, filepath.Join(dir, "page")).CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %v %s", err, out)
	}
	png, err := os.ReadFile(filepath.Join(dir, "page.png"))
	if err != nil {
		t.Fatal(err)
	}
	text, kind, err := Default().Extract(context.Background(), png)
	if err != nil || kind != Image {
		t.Fatalf("kind=%s err=%v", kind, err)
	}
	if !strings.Contains(text, "Aufenthaltstitel") {
		t.Fatalf("OCR text = %q", text)
	}
}

func TestBlankImageHasNoText(t *testing.T) {
	needTools(t)
	dir := t.TempDir()
	pdf := filepath.Join(dir, "blank.pdf")
	_ = os.WriteFile(pdf, makePDF(nil), 0o600)
	if out, err := exec.Command("pdftoppm", "-r", "100", "-png", "-singlefile", pdf, filepath.Join(dir, "blank")).CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %v %s", err, out)
	}
	png, _ := os.ReadFile(filepath.Join(dir, "blank.png"))
	_, _, err := Default().Extract(context.Background(), png)
	if !errors.Is(err, ErrNoText) {
		t.Fatalf("err = %v, want ErrNoText", err)
	}
}

func TestTempFilesAreRemoved(t *testing.T) {
	needTools(t)
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "briefklar-*"))
	_, _, _ = Default().Extract(context.Background(), makePDF(letterLines))
	after, _ := filepath.Glob(filepath.Join(os.TempDir(), "briefklar-*"))
	if len(after) > len(before) {
		t.Fatalf("temp directories left behind: %v", after)
	}
}
