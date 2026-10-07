package extract

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// makePDF builds a one-page PDF with Helvetica text lines (ASCII only), with a correct xref table.
func makePDF(lines []string) []byte {
	var content strings.Builder
	content.WriteString("BT /F1 16 Tf 72 760 Td 22 TL\n")
	for _, l := range lines {
		l = strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(l)
		content.WriteString("(" + l + ") Tj T*\n")
	}
	content.WriteString("ET")
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// makeScannedPDF wraps a PNG into a one-page PDF that has an image but no text layer.
func makeScannedPDF(pngData []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	gray := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	_, _ = zw.Write(gray.Pix)
	_ = zw.Close()
	content := "q 595 0 0 842 0 0 cm /Im1 Do Q"
	objs := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im1 4 0 R >> >> /Contents 5 0 R >>"),
		append([]byte(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n",
			b.Dx(), b.Dy(), z.Len())), append(z.Bytes(), []byte("\nendstream")...)...),
		[]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(o)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes(), nil
}
