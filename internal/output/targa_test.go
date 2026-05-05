package output

import (
	"bytes"
	"testing"
)

func TestTargaWriterColor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &targaWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       2,
		NumComponents: 3,
		ColorSpace:   ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify header is 18 bytes
	headerLen := buf.Len()
	if headerLen != 18 {
		t.Fatalf("header length = %d, want 18", headerLen)
	}

	// Verify key header fields
	data := buf.Bytes()
	// byte 2: image type (2 = uncompressed true-color)
	if data[2] != 2 {
		t.Errorf("image type = %d, want 2", data[2])
	}
	// bytes 12-13: width (little-endian)
	if data[12] != 2 || data[13] != 0 {
		t.Errorf("width = %d,%d, want 2,0", data[12], data[13])
	}
	// bytes 14-15: height (little-endian)
	if data[14] != 2 || data[15] != 0 {
		t.Errorf("height = %d,%d, want 2,0", data[14], data[15])
	}
	// byte 16: bits per pixel
	if data[16] != 24 {
		t.Errorf("bits per pixel = %d, want 24", data[16])
	}
	// byte 17: image descriptor (0x20 = top-down)
	if data[17] != 0x20 {
		t.Errorf("image descriptor = 0x%02x, want 0x20", data[17])
	}

	// Write 2 rows of 2 RGB pixels
	row0 := []byte{255, 0, 0, 0, 255, 0}     // red, green
	row1 := []byte{0, 0, 255, 255, 255, 255} // blue, white

	if err := w.WriteScanline(row0); err != nil {
		t.Fatalf("WriteScanline row0 failed: %v", err)
	}
	if err := w.WriteScanline(row1); err != nil {
		t.Fatalf("WriteScanline row1 failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	// Verify pixel data is in BGR order
	allData := buf.Bytes()
	pixelStart := 18

	// Row 0: red(255,0,0), green(0,255,0) -> BGR: (0,0,255), (0,255,0)
	wantRow0 := []byte{0, 0, 255, 0, 255, 0}
	for i, w := range wantRow0 {
		if allData[pixelStart+i] != w {
			t.Errorf("pixel row0[%d] = %d, want %d", i, allData[pixelStart+i], w)
		}
	}

	// Row 1: blue(0,0,255), white(255,255,255) -> BGR: (255,0,0), (255,255,255)
	wantRow1 := []byte{255, 0, 0, 255, 255, 255}
	for i, w := range wantRow1 {
		if allData[pixelStart+6+i] != w {
			t.Errorf("pixel row1[%d] = %d, want %d", i, allData[pixelStart+6+i], w)
		}
	}
}

func TestTargaWriterGrayscale(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &targaWriter{}

	info := &ImageInfo{
		Width:        4,
		Height:       1,
		NumComponents: 1,
		ColorSpace:   ColorSpaceGrayscale,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify header
	data := buf.Bytes()
	if data[2] != 3 {
		t.Errorf("image type = %d, want 3 (grayscale)", data[2])
	}
	if data[16] != 8 {
		t.Errorf("bits per pixel = %d, want 8", data[16])
	}

	// Write row
	row := []byte{0, 64, 128, 255}
	if err := w.WriteScanline(row); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	// Verify pixel data (grayscale passes through as-is)
	allData := buf.Bytes()
	for i, want := range row {
		if allData[18+i] != want {
			t.Errorf("pixel[%d] = %d, want %d", i, allData[18+i], want)
		}
	}
}
