package output

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBMPWriterColor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       2,
		NumComponents: 3,
		ColorSpace:   ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Write 2 rows of 2 RGB pixels each
	// Row 0 (top in image, bottom in BMP): red, green
	row0 := []byte{255, 0, 0, 0, 255, 0}
	// Row 1 (bottom in image, top in BMP): blue, white
	row1 := []byte{0, 0, 255, 255, 255, 255}

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

	// Verify BMP magic bytes
	if data[0] != 'B' || data[1] != 'M' {
		t.Fatalf("BMP magic = %x, want 'BM'", data[:2])
	}

	// Verify file size
	fileSize := binary.LittleEndian.Uint32(data[2:6])
	// Header (14+40=54) + pixel data (2 rows * 2 pixels * 3 bytes = 12, padded to 8 per row)
	// rowWidth = (2*3 + 3) &^ 3 = 8, so 2 * 8 = 16 bytes of pixel data
	expectedFileSize := uint32(54 + 16)
	if fileSize != expectedFileSize {
		t.Errorf("file size = %d, want %d", fileSize, expectedFileSize)
	}

	// Verify pixel data offset
	pixelOffset := binary.LittleEndian.Uint32(data[10:14])
	if pixelOffset != 54 {
		t.Errorf("pixel offset = %d, want 54", pixelOffset)
	}

	// Verify DIB header size
	dibSize := binary.LittleEndian.Uint32(data[14:18])
	if dibSize != 40 {
		t.Errorf("DIB header size = %d, want 40", dibSize)
	}

	// Verify width and height
	bmpWidth := int32(binary.LittleEndian.Uint32(data[18:22]))
	bmpHeight := int32(binary.LittleEndian.Uint32(data[22:26]))
	if bmpWidth != 2 {
		t.Errorf("BMP width = %d, want 2", bmpWidth)
	}
	if bmpHeight != 2 {
		t.Errorf("BMP height = %d, want 2", bmpHeight)
	}

	// Verify bits per pixel
	bpp := binary.LittleEndian.Uint16(data[28:30])
	if bpp != 24 {
		t.Errorf("bits per pixel = %d, want 24", bpp)
	}

	// Verify bottom-up row order and BGR pixel order
	// BMP stores bottom row first. Our image:
	//   Image row 0 (top): R,G,B, R,G,B = 255,0,0, 0,255,0  -> stored as last in BMP
	//   Image row 1 (bottom): R,G,B, R,G,B = 0,0,255, 255,255,255 -> stored first in BMP
	// BMP pixels are BGR:
	//   First BMP row (image row 1): B=255,G=0,R=0, B=255,G=255,R=255 + 2 padding
	//   Second BMP row (image row 0): B=0,G=0,R=255, B=0,G=255,R=0 + 2 padding
	pixelStart := int(pixelOffset)

	// First BMP row (image bottom = row1): blue, white in BGR
	// pixel(0,0): B=255,G=0,R=0  pixel(0,1): B=255,G=255,R=255 + 2 pad
	wantRow0 := []byte{255, 0, 0, 255, 255, 255, 0, 0}
	for i, w := range wantRow0 {
		if data[pixelStart+i] != w {
			t.Errorf("BMP pixel row0[%d] = %d, want %d", i, data[pixelStart+i], w)
		}
	}

	// Second BMP row (image top = row0): red, green in BGR
	// pixel(1,0): B=0,G=0,R=255  pixel(1,1): B=0,G=255,R=0 + 2 pad
	wantRow1 := []byte{0, 0, 255, 0, 255, 0, 0, 0}
	for i, w := range wantRow1 {
		if data[pixelStart+8+i] != w {
			t.Errorf("BMP pixel row1[%d] = %d, want %d", i, data[pixelStart+8+i], w)
		}
	}
}

func TestBMPWriterGrayscale(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &bmpWriter{}

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

	// Verify BMP magic
	if data[0] != 'B' || data[1] != 'M' {
		t.Fatalf("BMP magic = %x, want 'BM'", data[:2])
	}

	// Verify bits per pixel (8-bit grayscale)
	bpp := binary.LittleEndian.Uint16(data[28:30])
	if bpp != 8 {
		t.Errorf("bits per pixel = %d, want 8", bpp)
	}

	// Verify color table entries count
	clrUsed := binary.LittleEndian.Uint32(data[46:50])
	if clrUsed != 256 {
		t.Errorf("color table entries = %d, want 256", clrUsed)
	}

	// Verify pixel data offset (14 + 40 + 256*4 = 1078)
	pixelOffset := binary.LittleEndian.Uint32(data[10:14])
	if pixelOffset != 1078 {
		t.Errorf("pixel offset = %d, want 1078", pixelOffset)
	}

	// Verify grayscale palette (first few entries)
	paletteStart := 54
	// Entry 0 should be B=0,G=0,R=0,0
	for i := 0; i < 3; i++ {
		if data[paletteStart+i] != 0 {
			t.Errorf("palette[0][%d] = %d, want 0", i, data[paletteStart+i])
		}
	}
	// Entry 255 should be B=255,G=255,R=255,0
	entry255 := paletteStart + 255*4
	for i := 0; i < 3; i++ {
		if data[entry255+i] != 255 {
			t.Errorf("palette[255][%d] = %d, want 255", i, data[entry255+i])
		}
	}

	// Verify pixel data (bottom-up)
	// rowWidth for grayscale 4-wide: (4+3) &^ 3 = 4 bytes per row
	pixelStart := int(pixelOffset)
	// First BMP row (image bottom = row1): 255, 170, 85, 0
	for i, want := range []byte{255, 170, 85, 0} {
		if data[pixelStart+i] != want {
			t.Errorf("pixel row1[%d] = %d, want %d", i, data[pixelStart+i], want)
		}
	}
	// Second BMP row (image top = row0): 0, 85, 170, 255
	for i, want := range []byte{0, 85, 170, 255} {
		if data[pixelStart+4+i] != want {
			t.Errorf("pixel row0[%d] = %d, want %d", i, data[pixelStart+4+i], want)
		}
	}
}

func TestBMPWriterTooManyScanlines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &bmpWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       1,
		NumComponents: 3,
		ColorSpace:   ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Write first row (OK)
	if err := w.WriteScanline([]byte{0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("WriteScanline 1 failed: %v", err)
	}

	// Write second row (should fail)
	if err := w.WriteScanline([]byte{0, 0, 0, 0, 0, 0}); err == nil {
		t.Error("expected error for too many scanlines, got nil")
	}
}
