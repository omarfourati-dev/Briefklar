package extract

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// renderPNG renders a PDF to a PNG via pdftoppm.
func renderPNG(t *testing.T, pdf []byte, dpi string) []byte {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("pdftoppm", "-r", dpi, "-png", "-singlefile", in, filepath.Join(dir, "p")).CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %v %s", err, out)
	}
	png, err := os.ReadFile(filepath.Join(dir, "p.png"))
	if err != nil {
		t.Fatal(err)
	}
	return png
}

func TestScannedPDFUsesOCR(t *testing.T) {
	needTools(t)
	scanned, err := makeScannedPDF(renderPNG(t, makePDF(letterLines), "200"))
	if err != nil {
		t.Fatal(err)
	}
	text, kind, err := Default().Extract(context.Background(), scanned)
	if err != nil || kind != PDF {
		t.Fatalf("kind=%s err=%v", kind, err)
	}
	if !strings.Contains(text, "Aufenthaltstitel") {
		t.Fatalf("OCR text = %q", text)
	}
}

func TestToolErrorHidesStderr(t *testing.T) {
	needTools(t)
	_, _, err := Default().Extract(context.Background(), []byte("%PDF-1.4 this is not a pdf"))
	if err == nil {
		t.Fatal("want error for corrupt PDF")
	}
	var te *ToolError
	if !errors.As(err, &te) {
		t.Fatalf("err %v is not a *ToolError", err)
	}
	stderr := te.Stderr()
	if stderr == "" || len(stderr) > 500 {
		t.Fatalf("Stderr() = %q", stderr)
	}
	first := strings.SplitN(stderr, "\n", 2)[0]
	if strings.Contains(err.Error(), first) {
		t.Fatalf("error message leaks stderr: %q", err.Error())
	}
}

// pngHeader returns a PNG signature plus a valid IHDR chunk claiming the given size (no image data).
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 0 // 8 bit, grayscale
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
	_ = binary.Write(&b, binary.BigEndian, uint32(13))
	chunk := append([]byte("IHDR"), ihdr...)
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

func TestHugeImageIsRejectedBeforeOCR(t *testing.T) {
	// no needTools: the tools must not even be started
	tools := &Tools{Tesseract: "/nonexistent/tesseract", PdfToText: "/nonexistent", PdfToPPM: "/nonexistent", MaxPages: 3}
	_, _, err := tools.Extract(context.Background(), pngHeader(10000, 10000))
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestCancelledContextReturnsPromptly(t *testing.T) {
	needTools(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, _, err := Default().Extract(ctx, makePDF(letterLines))
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("err = %v after %s", err, time.Since(start))
	}
}

func TestZeroValueToolsClampsMaxPages(t *testing.T) {
	needTools(t)
	tools := &Tools{Tesseract: "tesseract", PdfToText: "pdftotext", PdfToPPM: "pdftoppm"}
	text, _, err := tools.Extract(context.Background(), makePDF(letterLines))
	if err != nil || !strings.Contains(text, "Aufenthaltstitel") {
		t.Fatalf("text=%q err=%v", text, err)
	}
}

func TestTempFilesAreRemoved(t *testing.T) {
	needTools(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	scanned, err := makeScannedPDF(renderPNG(t, makePDF(letterLines), "200"))
	if err != nil {
		t.Fatal(err)
	}
	blank := renderPNG(t, makePDF(nil), "100")
	for name, data := range map[string][]byte{
		"text pdf":    makePDF(letterLines),
		"scanned pdf": scanned,
		"blank image": blank,
		"corrupt pdf": []byte("%PDF-1.4 not a pdf"),
	} {
		_, _, _ = Default().Extract(context.Background(), data)
		entries, _ := os.ReadDir(tmp)
		if len(entries) != 0 {
			t.Fatalf("%s: temp entries left behind: %v", name, entries)
		}
	}
}
