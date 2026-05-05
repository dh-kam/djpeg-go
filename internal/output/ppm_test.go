package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestPPMWriterColor(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:        2,
		Height:       2,
		NumComponents: 3,
		ColorSpace:   ColorSpaceRGB,
	}

	if err := w.Start(&buf, info); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify header
	header := buf.String()
	if !strings.HasPrefix(header, "P6\n") {
		t.Errorf("header should start with P6, got: %q", header[:10])
	}
	if !strings.Contains(header, "2 2\n") {
		t.Errorf("header should contain dimensions, got: %q", header)
	}
	if !strings.Contains(header, "255\n") {
		t.Errorf("header should contain maxval, got: %q", header)
	}

	// Write 2 rows of 2 RGB pixels each
	row1 := []byte{255, 0, 0, 0, 255, 0}     // red, green
	row2 := []byte{0, 0, 255, 255, 255, 255} // blue, white

	if err := w.WriteScanline(row1); err != nil {
		t.Fatalf("WriteScanline row1 failed: %v", err)
	}
	if err := w.WriteScanline(row2); err != nil {
		t.Fatalf("WriteScanline row2 failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	// Verify pixel data appears after header
	headerEnd := strings.Index(header, "255\n") + 4
	pixelData := buf.Bytes()[headerEnd:]

	wantPixels := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 255}
	if len(pixelData) < len(wantPixels) {
		t.Fatalf("pixel data length %d, want at least %d", len(pixelData), len(wantPixels))
	}
	for i, w := range wantPixels {
		if pixelData[i] != w {
			t.Errorf("pixel[%d] = %d, want %d", i, pixelData[i], w)
		}
	}
}

func TestPPMWriterGrayscale(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &ppmWriter{}

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
	header := buf.String()
	if !strings.HasPrefix(header, "P5\n") {
		t.Errorf("header should start with P5, got: %q", header[:10])
	}
	if !strings.Contains(header, "4 1\n") {
		t.Errorf("header should contain dimensions, got: %q", header)
	}
	if !strings.Contains(header, "255\n") {
		t.Errorf("header should contain maxval, got: %q", header)
	}

	// Write 1 row of 4 grayscale pixels
	row := []byte{0, 85, 170, 255}

	if err := w.WriteScanline(row); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	// Verify pixel data
	headerEnd := strings.Index(header, "255\n") + 4
	pixelData := buf.Bytes()[headerEnd:]

	for i, want := range []byte{0, 85, 170, 255} {
		if pixelData[i] != want {
			t.Errorf("pixel[%d] = %d, want %d", i, pixelData[i], want)
		}
	}
}

func TestPPMWriterStartFinish(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &ppmWriter{}

	info := &ImageInfo{
		Width:        1,
		Height:       1,
		NumComponents: 3,
		ColorSpace:   ColorSpaceRGB,
	}

	// Test full lifecycle
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
		t.Error("output is empty")
	}
}

func TestPPMWriterWithColormap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := &ppmWriter{}

	// Create a simple colormap for grayscale
	cm := &Colormap{
		Maps: [][]uint8{
			{0, 128, 255, 64}, // 4-entry grayscale map
		},
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
		t.Fatalf("Start failed: %v", err)
	}

	// Write indices 0,1,2,3 which map to 0,128,255,64
	if err := w.WriteScanline([]byte{0, 1, 2, 3}); err != nil {
		t.Fatalf("WriteScanline failed: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	// Verify demapped pixel data
	headerEnd := bytes.Index(buf.Bytes(), []byte("255\n")) + 4
	pixelData := buf.Bytes()[headerEnd:]

	wantPixels := []byte{0, 128, 255, 64}
	for i, w := range wantPixels {
		if pixelData[i] != w {
			t.Errorf("pixel[%d] = %d, want %d", i, pixelData[i], w)
		}
	}
}
