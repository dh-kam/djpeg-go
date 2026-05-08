package djpeggo_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/decoder"
	"github.com/dh-kam/djpeg-go/internal/output"
)

// decodeWithGoStdLib decodes a JPEG using Go's standard library for reference.
func decodeWithGoStdLib(t *testing.T, data []byte) (image.Image, error) {
	t.Helper()
	return jpeg.Decode(bytes.NewReader(data))
}

func TestIntegrationDecodeGrayscale(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_gray.jpg")
	if err != nil {
		t.Skip("test_gray.jpg not available:", err)
	}

	pixels, w, h, components, err := decoder.DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	if components != 1 {
		t.Errorf("components = %d, want 1 (grayscale)", components)
	}

	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Errorf("pixel data length = %d, want %d", len(pixels), expectedLen)
	}

	// Verify that all pixels are within valid range [0, 255]
	for i, p := range pixels {
		if p > 255 {
			t.Fatalf("pixel[%d] = %d, out of range", i, p)
		}
	}

	t.Logf("Decoded grayscale JPEG: %dx%d, %d components, %d bytes", w, h, components, len(pixels))
}

func TestIntegrationDecodeColor(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_color.jpg")
	if err != nil {
		t.Skip("test_color.jpg not available:", err)
	}

	pixels, w, h, components, err := decoder.DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	if components != 3 {
		t.Errorf("components = %d, want 3 (RGB)", components)
	}

	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Errorf("pixel data length = %d, want %d", len(pixels), expectedLen)
	}

	t.Logf("Decoded color JPEG: %dx%d, %d components, %d bytes", w, h, components, len(pixels))
}

func TestIntegrationDecode420(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	pixels, w, h, components, err := decoder.DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	if components != 3 {
		t.Errorf("components = %d, want 3 (RGB)", components)
	}

	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Errorf("pixel data length = %d, want %d", len(pixels), expectedLen)
	}

	t.Logf("Decoded 4:2:0 JPEG: %dx%d, %d components, %d bytes", w, h, components, len(pixels))
}

func TestIntegrationDecodeFromBytes(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_color.jpg")
	if err != nil {
		t.Skip("test_color.jpg not available:", err)
	}

	r := bytes.NewReader(data)
	dec := decoder.New(r)

	w, h, nc, cs, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}

	if w <= 0 || h <= 0 {
		t.Errorf("dimensions = %dx%d, want positive", w, h)
	}
	if nc != 3 {
		t.Errorf("numComponents = %d, want 3", nc)
	}
	_ = cs

	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}

	rowStride := dec.OutputWidth() * dec.OutputComponents()
	allPixels := make([]byte, 0, dec.OutputHeight()*rowStride)
	scanline := make([]byte, rowStride)

	rowsRead := 0
	for rowsRead < dec.OutputHeight() {
		n, err := dec.ReadScanlines([][]byte{scanline})
		if err != nil {
			t.Fatalf("ReadScanlines failed: %v", err)
		}
		if n == 0 {
			break
		}
		allPixels = append(allPixels, scanline...)
		rowsRead += n
	}

	if rowsRead != h {
		t.Errorf("rows read = %d, want %d", rowsRead, h)
	}
	if len(allPixels) != w*h*3 {
		t.Errorf("total pixels = %d, want %d", len(allPixels), w*h*3)
	}
}

func TestIntegrationDecodeToRGB(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_gray.jpg")
	if err != nil {
		t.Skip("test_gray.jpg not available:", err)
	}

	pixels, w, h, components, err := decoder.DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	if len(pixels) == 0 {
		t.Error("pixel data is empty")
	}
	if w <= 0 || h <= 0 {
		t.Errorf("dimensions = %dx%d, want positive", w, h)
	}
	if components <= 0 {
		t.Errorf("components = %d, want > 0", components)
	}
}

func TestIntegrationNonJPEG(t *testing.T) {
	t.Parallel()

	// Try to decode a PNG file as JPEG
	_, _, _, _, err := decoder.DecodeToRGB(bytes.NewReader([]byte{0x89, 0x50, 0x4E, 0x47}))
	if err == nil {
		t.Error("expected error for non-JPEG data, got nil")
	}
}

func TestIntegrationTruncatedJPEG(t *testing.T) {
	t.Parallel()

	// Only SOI marker
	_, _, _, _, err := decoder.DecodeToRGB(bytes.NewReader([]byte{0xFF, 0xD8}))
	if err == nil {
		t.Error("expected error for truncated JPEG, got nil")
	}
}

func TestIntegrationOutputPPM(t *testing.T) {
	data, err := os.ReadFile("testdata/test_gray.jpg")
	if err != nil {
		t.Skip("test_gray.jpg not available:", err)
	}

	pixels, w, h, components, err := decoder.DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	// Write to PPM format via output writer
	var buf bytes.Buffer
	wr, err := output.NewWriter(output.FormatPPM)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	colorSpace := output.ColorSpaceGrayscale
	if components == 3 {
		colorSpace = output.ColorSpaceRGB
	}

	info := &output.ImageInfo{
		Width:         w,
		Height:        h,
		NumComponents: components,
		ColorSpace:    colorSpace,
	}

	if err := wr.Start(&buf, info); err != nil {
		t.Fatalf("PPM Start failed: %v", err)
	}

	rowStride := w * components
	for y := 0; y < h; y++ {
		row := pixels[y*rowStride : (y+1)*rowStride]
		if err := wr.WriteScanline(row); err != nil {
			t.Fatalf("PPM WriteScanline failed at row %d: %v", y, err)
		}
	}

	if err := wr.Finish(); err != nil {
		t.Fatalf("PPM Finish failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("PPM output is empty")
	}

	t.Logf("PPM output: %d bytes", buf.Len())
}

func TestIntegrationOutputBMP(t *testing.T) {
	data, err := os.ReadFile("testdata/test_color.jpg")
	if err != nil {
		t.Skip("test_color.jpg not available:", err)
	}

	pixels, w, h, components, err := decoder.DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	var buf bytes.Buffer
	wr, err := output.NewWriter(output.FormatBMP)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	info := &output.ImageInfo{
		Width:         w,
		Height:        h,
		NumComponents: components,
		ColorSpace:    output.ColorSpaceRGB,
	}

	if err := wr.Start(&buf, info); err != nil {
		t.Fatalf("BMP Start failed: %v", err)
	}

	rowStride := w * components
	for y := 0; y < h; y++ {
		row := pixels[y*rowStride : (y+1)*rowStride]
		if err := wr.WriteScanline(row); err != nil {
			t.Fatalf("BMP WriteScanline failed at row %d: %v", y, err)
		}
	}

	if err := wr.Finish(); err != nil {
		t.Fatalf("BMP Finish failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("BMP output is empty")
	}

	// Verify BMP starts with "BM"
	if buf.Bytes()[0] != 'B' || buf.Bytes()[1] != 'M' {
		t.Error("BMP magic bytes not found")
	}

	t.Logf("BMP output: %d bytes", buf.Len())
}

func TestIntegrationOutputTarga(t *testing.T) {
	data, err := os.ReadFile("testdata/test_color.jpg")
	if err != nil {
		t.Skip("test_color.jpg not available:", err)
	}

	pixels, w, h, components, err := decoder.DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	var buf bytes.Buffer
	wr, err := output.NewWriter(output.FormatTarga)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	info := &output.ImageInfo{
		Width:         w,
		Height:        h,
		NumComponents: components,
		ColorSpace:    output.ColorSpaceRGB,
	}

	if err := wr.Start(&buf, info); err != nil {
		t.Fatalf("Targa Start failed: %v", err)
	}

	rowStride := w * components
	for y := 0; y < h; y++ {
		row := pixels[y*rowStride : (y+1)*rowStride]
		if err := wr.WriteScanline(row); err != nil {
			t.Fatalf("Targa WriteScanline failed at row %d: %v", y, err)
		}
	}

	if err := wr.Finish(); err != nil {
		t.Fatalf("Targa Finish failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Targa output is empty")
	}

	t.Logf("Targa output: %d bytes", buf.Len())
}
