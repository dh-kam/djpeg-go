package output

import (
	"bytes"
	"testing"
)

func TestGIFWriterGrayscale(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &gifWriter{}

	info := &ImageInfo{
		Width:        4,
		Height:       2,
		NumComponents: 1,
		ColorSpace:   ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Write 2 rows of 4 grayscale pixels
	row0 := []byte{0, 85, 170, 255}
	row1 := []byte{255, 170, 85, 0}

	if err := w.WriteScanline(row0); err != nil {
		t.Fatalf("WriteScanline row0 failed: %v", err)
	}
	if err := w.WriteScanline(row1); err != nil {
		t.Fatalf("WriteScanline row1 failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	data := buf.Bytes()

	// Verify GIF87a header
	if len(data) < 6 {
		t.Fatal("output too short")
	}
	if string(data[:6]) != "GIF87a" {
		t.Errorf("header = %q, want GIF87a", string(data[:6]))
	}

	// Verify logical screen descriptor
	// Width (bytes 6-7), Height (bytes 8-9)
	if data[6] != 4 || data[7] != 0 {
		t.Errorf("width = %d,%d, want 4,0", data[6], data[7])
	}
	if data[8] != 2 || data[9] != 0 {
		t.Errorf("height = %d,%d, want 2,0", data[8], data[9])
	}

	// Verify global color table flag is set
	if data[10]&0x80 == 0 {
		t.Error("global color table flag not set")
	}

	// The output should end with the GIF trailer ';'
	if data[len(data)-1] != ';' {
		t.Errorf("last byte = 0x%02x, want ';'", data[len(data)-1])
	}
}

func TestGIFWriterRGBRequiresQuantization(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &gifWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       2,
		NumComponents: 3,
		ColorSpace:   ColorSpaceRGB,
		// No QuantizeColors set
	}

	err := w.Start(&buf, info)
	if err == nil {
		t.Error("expected error for RGB GIF without quantization, got nil")
	}
}

func TestGIFWriterTooManyScanlines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &gifWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       1,
		NumComponents: 1,
		ColorSpace:   ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Write first row (OK)
	if err := w.WriteScanline([]byte{0, 0}); err != nil {
		t.Fatalf("WriteScanline 1 failed: %v", err)
	}

	// Write second row (should fail)
	if err := w.WriteScanline([]byte{0, 0}); err == nil {
		t.Error("expected error for too many scanlines, got nil")
	}
}
