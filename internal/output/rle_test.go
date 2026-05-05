package output

import (
	"bytes"
	"testing"
)

func TestRLEWriterGrayscale(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:        4,
		Height:       2,
		NumComponents: 1,
		ColorSpace:   ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Write 2 rows
	row0 := []byte{0, 85, 170, 255}
	row1 := []byte{255, 255, 255, 255}

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

	// Verify RLE magic: "rle" + 0x95
	if len(data) < 4 {
		t.Fatal("output too short for magic")
	}
	if !bytes.Equal(data[:4], []byte("rle\x95")) {
		t.Errorf("magic = %q, want \"rle\\x95\"", string(data[:4]))
	}
}

func TestRLEWriterRGB(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       1,
		NumComponents: 3,
		ColorSpace:   ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	row := []byte{255, 0, 0, 0, 255, 0}
	if err := w.WriteScanline(row); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	data := buf.Bytes()

	// Verify RLE magic
	if !bytes.Equal(data[:4], []byte("rle\x95")) {
		t.Errorf("magic = %q, want \"rle\\x95\"", string(data[:4]))
	}

	// The file should contain an EOF opcode (5, 0) near the end
	if len(data) < 6 {
		t.Fatal("output too short")
	}
	// Last 2 bytes should be EOF opcode
	lastTwo := data[len(data)-2:]
	if lastTwo[0] != 5 { // rleOpEOF = 5
		t.Errorf("last opcode = %d, want 5 (EOF)", lastTwo[0])
	}
}

func TestRLEWriterTooLarge(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:        40000,
		Height:       1,
		NumComponents: 1,
		ColorSpace:   ColorSpaceGrayscale,
	}

	err := w.Start(&buf, info)
	if err == nil {
		t.Error("expected error for too-large image, got nil")
	}
}

func TestRLEWriterTooManyScanlines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &rleWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       1,
		NumComponents: 1,
		ColorSpace:   ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := w.WriteScanline([]byte{0, 0}); err != nil {
		t.Fatalf("WriteScanline 1 failed: %v", err)
	}

	if err := w.WriteScanline([]byte{0, 0}); err == nil {
		t.Error("expected error for too many scanlines, got nil")
	}
}
