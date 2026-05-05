package output

import (
	"bytes"
	"testing"
)

// TestNewWriterAllFormats tests creating writers for all supported formats.
func TestNewWriterAllFormats(t *testing.T) {
	formats := []Format{FormatPPM, FormatBMP, FormatGIF, FormatTarga, FormatRLE}
	for _, f := range formats {
		w, err := NewWriter(f)
		if err != nil {
			t.Errorf("NewWriter(%s) error: %v", f, err)
		}
		if w == nil {
			t.Errorf("NewWriter(%s) returned nil", f)
		}
	}
}

// TestNewWriterUnsupported tests unsupported format.
func TestNewWriterUnsupported(t *testing.T) {
	_, err := NewWriter("png")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

// TestPPMWriter1x1 tests PPM output for 1x1 image.
func TestPPMWriter1x1(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{128, 128, 128}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("PPM output is empty")
	}
}

// TestBMPWriter1x1 tests BMP output for 1x1 image.
func TestBMPWriter1x1(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{128, 128, 128}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'B' || data[1] != 'M' {
		t.Error("BMP magic bytes incorrect")
	}
}

// TestBMPWriter1x1Grayscale tests BMP output for 1x1 grayscale.
func TestBMPWriter1x1Grayscale(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{128}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'B' || data[1] != 'M' {
		t.Error("BMP magic bytes incorrect")
	}
}

// TestGIFWriter1x1 tests GIF output for 1x1 grayscale image.
func TestGIFWriter1x1(t *testing.T) {
	var buf bytes.Buffer
	w := &gifWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{128}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 6 {
		t.Fatal("GIF output too short")
	}
	if string(data[:6]) != "GIF87a" {
		t.Errorf("header = %q, want GIF87a", string(data[:6]))
	}
}

// TestTargaWriter1x1 tests Targa output for 1x1 image.
func TestTargaWriter1x1(t *testing.T) {
	var buf bytes.Buffer
	w := &targaWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{128, 128, 128}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	if buf.Len() != 21 { // 18 header + 3 pixel bytes
		t.Errorf("Targa output length = %d, want 21", buf.Len())
	}
}

// TestRLEWriter1x1 tests RLE output for 1x1 image.
func TestRLEWriter1x1(t *testing.T) {
	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{128}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	if !bytes.HasPrefix(buf.Bytes(), []byte("rle\x95")) {
		t.Error("RLE magic bytes incorrect")
	}
}

// TestPPMWriterOddDimensions tests PPM with odd dimensions.
func TestPPMWriterOddDimensions(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:         3,
		Height:        5,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for row := 0; row < 5; row++ {
		pixels := make([]byte, 9) // 3 pixels * 3 bytes
		for i := range pixels {
			pixels[i] = byte(row * 50)
		}
		if err := w.WriteScanline(pixels); err != nil {
			t.Fatalf("WriteScanline row %d failed: %v", row, err)
		}
	}

	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

// TestBMPWriterOddDimensions tests BMP with odd width.
func TestBMPWriterOddDimensions(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:         3,
		Height:        2,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	row0 := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255} // R, G, B
	row1 := []byte{128, 128, 128, 64, 64, 64, 32, 32, 32}

	if err := w.WriteScanline(row0); err != nil {
		t.Fatalf("WriteScanline row0: %v", err)
	}
	if err := w.WriteScanline(row1); err != nil {
		t.Fatalf("WriteScanline row1: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'B' || data[1] != 'M' {
		t.Error("BMP magic incorrect")
	}
}

// TestPPMWriterEmptyScanline tests PPM with empty scanline data.
func TestPPMWriterEmptyScanline(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

// TestBMPFinishWithoutWrite tests BMP finish without writing scanlines.
func TestBMPFinishWithoutWrite(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        2,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	// Don't write any scanlines
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	// Should still have valid BMP header
	data := buf.Bytes()
	if data[0] != 'B' || data[1] != 'M' {
		t.Error("BMP magic incorrect")
	}
}

// TestTargaFinishWithoutWrite tests Targa finish without writing scanlines.
func TestTargaFinishWithoutWrite(t *testing.T) {
	var buf bytes.Buffer
	w := &targaWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        2,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

// TestGIFWriterQuantized tests GIF with quantized colors.
func TestGIFWriterQuantized(t *testing.T) {
	var buf bytes.Buffer
	w := &gifWriter{}

	cm := &Colormap{
		Maps: [][]uint8{
			{0, 128, 255},
		},
		NumColors: 3,
	}

	info := &ImageInfo{
		Width:          2,
		Height:         1,
		NumComponents:  1,
		ColorSpace:     ColorSpaceGrayscale,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 2}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

// TestRLEWriterFinishWithoutWrite tests RLE finish without writing.
func TestRLEWriterFinishWithoutWrite(t *testing.T) {
	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        2,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

// TestPPMWriterGrayscaleColormap tests PPM grayscale with colormap.
func TestPPMWriterGrayscaleColormap(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	cm := &Colormap{
		Maps: [][]uint8{
			{0, 64, 128, 192, 255},
		},
		NumColors: 5,
	}

	info := &ImageInfo{
		Width:          5,
		Height:         1,
		NumComponents:  1,
		ColorSpace:     ColorSpaceGrayscale,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 1, 2, 3, 4}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

// TestPPMWriterRGBColormap tests PPM RGB with colormap.
func TestPPMWriterRGBColormap(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	cm := &Colormap{
		Maps: [][]uint8{
			{255, 0},    // R
			{0, 255},    // G
			{0, 0},      // B
		},
		NumColors: 2,
	}

	info := &ImageInfo{
		Width:          2,
		Height:         1,
		NumComponents:  3,
		ColorSpace:     ColorSpaceRGB,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 1}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

// TestTargaWriterExtraScanline tests writing extra scanlines beyond height.
func TestTargaWriterExtraScanline(t *testing.T) {
	var buf bytes.Buffer
	w := &targaWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("WriteScanline 1 failed: %v", err)
	}
	// Writing an extra scanline should not panic; it just writes more data
	_ = w.WriteScanline([]byte{0, 0, 0, 0, 0, 0})
}

// TestTargaWriterGrayscaleExtraScanline tests grayscale targa with extra scanlines.
func TestTargaWriterGrayscaleExtraScanline(t *testing.T) {
	var buf bytes.Buffer
	w := &targaWriter{}

	info := &ImageInfo{
		Width:         4,
		Height:        1,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 0, 0, 0}); err != nil {
		t.Fatalf("WriteScanline 1 failed: %v", err)
	}
	// Writing an extra scanline should not panic
	_ = w.WriteScanline([]byte{0, 0, 0, 0})
}

// TestPPMStartWritesHeader verifies Start writes header bytes immediately.
func TestPPMStartWritesHeader(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:         8,
		Height:        8,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Start should write header bytes")
	}
}

// TestPPMWriteScanlineExtra tests PPM writing extra scanlines past height.
func TestPPMWriteScanlineExtra(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("WriteScanline 1 failed: %v", err)
	}
	// Writing an extra scanline should not panic; it just appends more data
	_ = w.WriteScanline([]byte{0, 0, 0, 0, 0, 0})
}

// TestBMPWriterDensity tests BMP with density info.
func TestBMPWriterDensity(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        2,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
		XDensity:      72,
		YDensity:      72,
		DensityUnit:   2,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.WriteScanline([]byte{255, 0, 0, 0, 255, 0}); err != nil {
		t.Fatalf("WriteScanline 1: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 0, 255, 128, 128, 128}); err != nil {
		t.Fatalf("WriteScanline 2: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'B' || data[1] != 'M' {
		t.Error("BMP magic incorrect")
	}
}

// TestBMPWriterGrayscaleColormapQuantized tests BMP grayscale with quantized colormap.
func TestBMPWriterGrayscaleColormapQuantized(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	cm := &Colormap{
		Maps:      [][]uint8{{10, 20, 30}},
		NumColors: 3,
	}

	info := &ImageInfo{
		Width:          3,
		Height:         1,
		NumComponents:  1,
		ColorSpace:     ColorSpaceGrayscale,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 1, 2}); err != nil {
		t.Fatalf("WriteScanline: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'B' || data[1] != 'M' {
		t.Error("BMP magic incorrect")
	}
}

// TestBMPWriterRGBQuantized tests BMP with quantized RGB colormap.
func TestBMPWriterRGBQuantized(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	cm := &Colormap{
		Maps:      [][]uint8{{255, 0}, {0, 255}, {0, 0}},
		NumColors: 2,
	}

	info := &ImageInfo{
		Width:          2,
		Height:         1,
		NumComponents:  3,
		ColorSpace:     ColorSpaceRGB,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 1}); err != nil {
		t.Fatalf("WriteScanline: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'B' || data[1] != 'M' {
		t.Error("BMP magic incorrect")
	}
}

// TestBMPWriterTooManyScanlinesEdge tests BMP error on too many scanlines.
func TestBMPWriterTooManyScanlinesEdge(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 0, 0}); err != nil {
		t.Fatalf("WriteScanline 1: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 0, 0}); err == nil {
		t.Error("expected error for too many scanlines")
	}
}

// TestPPMWriterGrayscaleFull tests PPM grayscale full write cycle.
func TestPPMWriterGrayscaleFull(t *testing.T) {
	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:         4,
		Height:        2,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 64, 128, 255}); err != nil {
		t.Fatalf("WriteScanline 1: %v", err)
	}
	if err := w.WriteScanline([]byte{32, 96, 160, 224}); err != nil {
		t.Fatalf("WriteScanline 2: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("P5")) {
		t.Error("expected P5 header for grayscale")
	}
}

// TestTargaWriterGrayscaleFull tests Targa grayscale full write cycle.
func TestTargaWriterGrayscaleFull(t *testing.T) {
	var buf bytes.Buffer
	w := &targaWriter{}

	info := &ImageInfo{
		Width:         4,
		Height:        2,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 64, 128, 255}); err != nil {
		t.Fatalf("WriteScanline 1: %v", err)
	}
	if err := w.WriteScanline([]byte{32, 96, 160, 224}); err != nil {
		t.Fatalf("WriteScanline 2: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

// TestTargaWriterGrayscaleColormap tests Targa grayscale with colormap.
func TestTargaWriterGrayscaleColormap(t *testing.T) {
	var buf bytes.Buffer
	w := &targaWriter{}

	cm := &Colormap{
		Maps:      [][]uint8{{0, 64, 128, 255}},
		NumColors: 4,
	}

	info := &ImageInfo{
		Width:          4,
		Height:         1,
		NumComponents:  1,
		ColorSpace:     ColorSpaceGrayscale,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 1, 2, 3}); err != nil {
		t.Fatalf("WriteScanline: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

// TestTargaWriterRGBColormap tests Targa RGB with colormap.
func TestTargaWriterRGBColormap(t *testing.T) {
	var buf bytes.Buffer
	w := &targaWriter{}

	cm := &Colormap{
		Maps:      [][]uint8{{255, 0}, {0, 255}, {0, 0}},
		NumColors: 2,
	}

	info := &ImageInfo{
		Width:          2,
		Height:         1,
		NumComponents:  3,
		ColorSpace:     ColorSpaceRGB,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 1}); err != nil {
		t.Fatalf("WriteScanline: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

// TestRLEWriterRGBEdge tests RLE output for RGB image.
func TestRLEWriterRGBEdge(t *testing.T) {
	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:         2,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{255, 0, 0, 0, 255, 0}); err != nil {
		t.Fatalf("WriteScanline: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

// TestRLEWriterLargeRow tests RLE encoding with a longer row.
func TestRLEWriterLargeRow(t *testing.T) {
	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:         128,
		Height:        1,
		NumComponents: 1,
		ColorSpace:    ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}

	row := make([]byte, 128)
	for i := range row {
		row[i] = byte(i % 16)
	}
	if err := w.WriteScanline(row); err != nil {
		t.Fatalf("WriteScanline: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

// TestGIFWriterRGBQuantized tests GIF with quantized RGB input.
func TestGIFWriterRGBQuantized(t *testing.T) {
	var buf bytes.Buffer
	w := &gifWriter{}

	cm := &Colormap{
		Maps:      [][]uint8{{0, 255}, {0, 0}, {0, 0}},
		NumColors: 2,
	}

	info := &ImageInfo{
		Width:          2,
		Height:         1,
		NumComponents:  1,
		ColorSpace:     ColorSpaceGrayscale,
		QuantizeColors: true,
		Colormap:       cm,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := w.WriteScanline([]byte{0, 1}); err != nil {
		t.Fatalf("WriteScanline: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	data := buf.Bytes()
	if string(data[:6]) != "GIF87a" {
		t.Errorf("header = %q, want GIF87a", string(data[:6]))
	}
}

// TestBMPUnsupportedColorSpace tests BMP with unsupported color space.
func TestBMPUnsupportedColorSpace(t *testing.T) {
	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:         1,
		Height:        1,
		NumComponents: 4,
		ColorSpace:    ColorSpace(99),
	}

	if err := w.Start(&buf, info); err == nil {
		t.Error("expected error for unsupported color space")
	}
}

// TestNewWriterAndUse tests creating and using writers via the factory.
func TestNewWriterAndUse(t *testing.T) {
	for _, f := range []Format{FormatPPM, FormatBMP, FormatTarga} {
		t.Run(string(f), func(t *testing.T) {
			w, err := NewWriter(f)
			if err != nil {
				t.Fatalf("NewWriter(%s): %v", f, err)
			}
			var buf bytes.Buffer
			info := &ImageInfo{
				Width:         2,
				Height:        1,
				NumComponents: 3,
				ColorSpace:    ColorSpaceRGB,
			}
			if err := w.Start(&buf, info); err != nil {
				t.Fatalf("Start: %v", err)
			}
			if err := w.WriteScanline([]byte{128, 128, 128, 64, 64, 64}); err != nil {
				t.Fatalf("WriteScanline: %v", err)
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}
			if buf.Len() == 0 {
				t.Errorf("%s: output is empty", f)
			}
		})
	}
}
