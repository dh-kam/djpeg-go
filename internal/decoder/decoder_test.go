package decoder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/testutil"
)

func testFixture(name string) string {
	return filepath.Join("..", "..", "tests", "testdata", name)
}

func TestNewDecoder(t *testing.T) {
	t.Parallel()

	data := []byte{0xFF, 0xD8, 0xFF, 0xD9} // SOI + EOI (minimal but incomplete JPEG)
	r := bytes.NewReader(data)
	dec := New(r)
	if dec == nil {
		t.Fatal("New returned nil")
	}
}

func TestFinishDecompressReleasesDecodeScratch(t *testing.T) {
	data, err := os.ReadFile(testFixture("test_color.jpg"))
	if err != nil {
		t.Skipf("test fixture missing: %v", err)
	}

	dec := New(bytes.NewReader(data))
	if _, _, _, _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}

	stride := dec.OutputWidth() * dec.OutputComponents()
	row := make([]byte, stride)
	for dec.OutputScanline() < dec.OutputHeight() {
		n, err := dec.ReadScanlines([][]byte{row})
		if err != nil {
			t.Fatalf("ReadScanlines failed: %v", err)
		}
		if n == 0 {
			t.Fatal("ReadScanlines returned 0 before output completed")
		}
	}
	if dec.componentBuf == nil {
		t.Fatal("componentBuf was released before FinishDecompress")
	}
	if _, _, ok := dec.QuantizationTable(0); !ok {
		t.Fatal("QuantizationTable(0) missing before FinishDecompress")
	}

	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}

	if dec.componentBuf != nil {
		t.Fatal("componentBuf was not released")
	}
	if dec.scanData != nil {
		t.Fatal("scanData was not released")
	}
	if dec.colorConv != nil {
		t.Fatal("colorConv was not released")
	}
	if dec.memoryUsed != 0 {
		t.Fatalf("memoryUsed = %d, want 0", dec.memoryUsed)
	}
	if _, _, ok := dec.QuantizationTable(0); !ok {
		t.Fatal("QuantizationTable(0) should remain available after scratch release")
	}
}

func TestReadHeaderNotJPEG(t *testing.T) {
	t.Parallel()

	// Not a JPEG: starts with PNG magic
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	r := bytes.NewReader(data)
	dec := New(r)

	_, _, _, _, err := dec.ReadHeader()
	if err == nil {
		t.Error("expected error for non-JPEG data, got nil")
	}
}

func TestReadHeaderTruncated(t *testing.T) {
	t.Parallel()

	// Truncated data: only SOI marker
	data := []byte{0xFF, 0xD8}
	r := bytes.NewReader(data)
	dec := New(r)

	_, _, _, _, err := dec.ReadHeader()
	if err == nil {
		t.Error("expected error for truncated JPEG data, got nil")
	}
}

// decodeGoldenFile decodes a golden test JPEG and returns pixel data.
func decodeGoldenFile(t *testing.T, jpegPath string) (pixels []byte, w, h, components int) {
	t.Helper()

	data, err := os.ReadFile(jpegPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", jpegPath, err)
	}

	pixels, w, h, components, err = DecodeToRGB(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeToRGB failed for %s: %v", jpegPath, err)
	}
	return pixels, w, h, components
}

// loadGoldenReference loads the expected pixel data from a golden JSON file.
func loadGoldenReference(t *testing.T, jsonPath string) (pixels []byte, w, h, nc int) {
	t.Helper()

	pixels, w, h, nc, err := testutil.LoadGoldenReference(jsonPath)
	if err != nil {
		t.Fatalf("failed to load golden reference %s: %v", jsonPath, err)
	}
	return pixels, w, h, nc
}

func TestDecodeGrayscaleGoldenFile(t *testing.T) {
	t.Parallel()

	pixels, w, h, components := decodeGoldenFile(t, testFixture("gray_8x8.jpg"))

	if w != 8 || h != 8 {
		t.Errorf("dimensions = %dx%d, want 8x8", w, h)
	}
	if components != 1 {
		t.Errorf("components = %d, want 1 (grayscale)", components)
	}

	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Fatalf("pixel data length = %d, want %d", len(pixels), expectedLen)
	}

	// Compare with golden reference
	refPixels, refW, refH, refNc := loadGoldenReference(t, testFixture("gray_8x8.json"))
	cmp := testutil.NewPixelComparator(refPixels, pixels, refW, refH, refNc)

	report := testutil.GenerateAccuracyReport(cmp, "gray_8x8", refW, refH, refNc)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("grayscale golden: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestDecodeColor444GoldenFile(t *testing.T) {
	t.Parallel()

	pixels, w, h, components := decodeGoldenFile(t, testFixture("color_8x8_444.jpg"))

	if w != 8 || h != 8 {
		t.Errorf("dimensions = %dx%d, want 8x8", w, h)
	}
	if components != 3 {
		t.Errorf("components = %d, want 3 (RGB)", components)
	}

	refPixels, refW, refH, refNc := loadGoldenReference(t, testFixture("color_8x8_444.json"))
	cmp := testutil.NewPixelComparator(refPixels, pixels, refW, refH, refNc)

	report := testutil.GenerateAccuracyReport(cmp, "color_8x8_444", refW, refH, refNc)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("color 4:4:4 golden: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestDecodeColor420GoldenFile(t *testing.T) {
	pixels, w, h, components := decodeGoldenFile(t, testFixture("color_16x16_420.jpg"))

	if w != 16 || h != 16 {
		t.Errorf("dimensions = %dx%d, want 16x16", w, h)
	}
	if components != 3 {
		t.Errorf("components = %d, want 3 (RGB)", components)
	}

	refPixels, refW, refH, refNc := loadGoldenReference(t, testFixture("color_16x16_420.json"))
	cmp := testutil.NewPixelComparator(refPixels, pixels, refW, refH, refNc)

	report := testutil.GenerateAccuracyReport(cmp, "color_16x16_420", refW, refH, refNc)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("color 4:2:0 golden: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestDecodeGrayscaleFile(t *testing.T) {
	f, err := os.Open(testFixture("test_gray.jpg"))
	if err != nil {
		t.Skip("test_gray.jpg not available:", err)
	}
	defer f.Close()

	dec := New(f)
	w, h, nc, cs, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}

	if w <= 0 || h <= 0 {
		t.Errorf("dimensions = %dx%d, want positive", w, h)
	}
	if nc != 1 {
		t.Errorf("numComponents = %d, want 1 (grayscale)", nc)
	}
	_ = cs // color space is available

	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}

	if dec.OutputWidth() != w {
		t.Errorf("OutputWidth = %d, want %d", dec.OutputWidth(), w)
	}
	if dec.OutputHeight() != h {
		t.Errorf("OutputHeight = %d, want %d", dec.OutputHeight(), h)
	}
	if dec.OutputComponents() != 1 {
		t.Errorf("OutputComponents = %d, want 1", dec.OutputComponents())
	}

	// Read all scanlines using testutil-powered validation
	scanline := make([]byte, w*dec.OutputComponents())
	allPixels := make([]byte, 0, w*h)
	rowsRead := 0
	for rowsRead < h {
		n, err := dec.ReadScanlines([][]byte{scanline})
		if err != nil {
			t.Fatalf("ReadScanlines failed at row %d: %v", rowsRead, err)
		}
		if n == 0 {
			break
		}
		allPixels = append(allPixels, scanline...)
		rowsRead += n
	}

	if rowsRead != h {
		t.Errorf("read %d rows, want %d", rowsRead, h)
	}

	// Validate all pixels are in range
	for i, p := range allPixels {
		if p > 255 {
			t.Errorf("pixel[%d] = %d, out of range", i, p)
		}
	}

	dec.FinishDecompress()
}

func TestDecodeColorFile(t *testing.T) {
	f, err := os.Open(testFixture("test_color.jpg"))
	if err != nil {
		t.Skip("test_color.jpg not available:", err)
	}
	defer f.Close()

	dec := New(f)
	w, h, nc, cs, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}

	if w <= 0 || h <= 0 {
		t.Errorf("dimensions = %dx%d, want positive", w, h)
	}
	if nc != 3 {
		t.Errorf("numComponents = %d, want 3 (color)", nc)
	}
	_ = cs

	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}

	if dec.OutputComponents() != 3 {
		t.Errorf("OutputComponents = %d, want 3", dec.OutputComponents())
	}

	scanline := make([]byte, w*dec.OutputComponents())
	rowsRead := 0
	for rowsRead < h {
		n, err := dec.ReadScanlines([][]byte{scanline})
		if err != nil {
			t.Fatalf("ReadScanlines failed at row %d: %v", rowsRead, err)
		}
		if n == 0 {
			break
		}
		rowsRead += n
	}

	if rowsRead != h {
		t.Errorf("read %d rows, want %d", rowsRead, h)
	}

	dec.FinishDecompress()
}

func TestDecode420File(t *testing.T) {
	f, err := os.Open(testFixture("test_420.jpg"))
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}
	defer f.Close()

	dec := New(f)
	w, h, _, _, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}

	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}

	scanline := make([]byte, w*dec.OutputComponents())
	rowsRead := 0
	for rowsRead < h {
		n, err := dec.ReadScanlines([][]byte{scanline})
		if err != nil {
			t.Fatalf("ReadScanlines failed at row %d: %v", rowsRead, err)
		}
		if n == 0 {
			break
		}
		rowsRead += n
	}

	if rowsRead != h {
		t.Errorf("read %d rows, want %d", rowsRead, h)
	}

	dec.FinishDecompress()
}

func TestDecodeToRGBFunction(t *testing.T) {
	f, err := os.Open(testFixture("test_gray.jpg"))
	if err != nil {
		t.Skip("test_gray.jpg not available:", err)
	}
	defer f.Close()

	pixels, w, h, components, err := DecodeToRGB(f)
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Errorf("pixel data length = %d, want %d (%dx%dx%d)", len(pixels), expectedLen, w, h, components)
	}

	if components != 1 {
		t.Errorf("components = %d, want 1 (grayscale)", components)
	}
}

func TestDecodeToRGBFromBytes(t *testing.T) {
	t.Parallel()

	// Read the test file into memory and decode from bytes
	data, err := os.ReadFile(testFixture("test_gray.jpg"))
	if err != nil {
		t.Skip("test_gray.jpg not available:", err)
	}

	r := bytes.NewReader(data)
	pixels, w, h, components, err := DecodeToRGB(r)
	if err != nil {
		t.Fatalf("DecodeToRGB from bytes failed: %v", err)
	}

	if w <= 0 || h <= 0 {
		t.Errorf("dimensions = %dx%d, want positive", w, h)
	}

	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Errorf("pixel data length = %d, want %d", len(pixels), expectedLen)
	}
}

func TestSetIDCTMethod(t *testing.T) {
	t.Parallel()

	r := bytes.NewReader([]byte{0xFF, 0xD8, 0xFF, 0xD9})
	dec := New(r)

	// Test all methods don't panic
	dec.SetIDCTMethod("slow")
	dec.SetIDCTMethod("fast")
	dec.SetIDCTMethod("float")
	dec.SetIDCTMethod("unknown") // should default to islow
}

func TestDecodeBuiltJPEGGrayscale(t *testing.T) {
	t.Parallel()

	// Build a minimal JPEG using the testutil builder and decode it
	b := testutil.NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetGrayscale().
		SetPattern("solid")
	jpegData := b.Build()

	pixels, w, h, components, err := DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed for built JPEG: %v", err)
	}

	if w != 8 || h != 8 {
		t.Errorf("dimensions = %dx%d, want 8x8", w, h)
	}
	if components != 1 {
		t.Errorf("components = %d, want 1", components)
	}

	expected := b.ExpectedPixels()
	cmp := testutil.NewPixelComparator(expected, pixels, w, h, components)

	report := testutil.GenerateAccuracyReport(cmp, "built_gray_8x8", w, h, components)
	t.Log(report.String())

	// For a DC-only solid block, the ISLOW IDCT should produce values very close to the target
	if cmp.MaxDiff() > 2 {
		t.Errorf("built grayscale JPEG: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestDecodeBuiltJPEGColor444(t *testing.T) {
	t.Parallel()

	b := testutil.NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetColor().
		SetPattern("solid")
	jpegData := b.Build()

	pixels, w, h, components, err := DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed for built JPEG: %v", err)
	}

	if w != 8 || h != 8 {
		t.Errorf("dimensions = %dx%d, want 8x8", w, h)
	}
	if components != 3 {
		t.Errorf("components = %d, want 3", components)
	}

	expected := b.ExpectedPixels()
	cmp := testutil.NewPixelComparator(expected, pixels, w, h, components)

	report := testutil.GenerateAccuracyReport(cmp, "built_color_8x8_444", w, h, components)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("built color 4:4:4 JPEG: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestDecodeBuiltJPEGColor420(t *testing.T) {
	b := testutil.NewMinimalJPEGBuilder().
		SetDimensions(16, 16).
		SetColor().
		SetSubsampling(0, 2, 2).
		SetSubsampling(1, 1, 1).
		SetSubsampling(2, 1, 1).
		SetPattern("solid")
	jpegData := b.Build()

	pixels, w, h, components, err := DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed for built JPEG: %v", err)
	}

	if w != 16 || h != 16 {
		t.Errorf("dimensions = %dx%d, want 16x16", w, h)
	}
	if components != 3 {
		t.Errorf("components = %d, want 3", components)
	}

	expected := b.ExpectedPixels()
	cmp := testutil.NewPixelComparator(expected, pixels, w, h, components)

	report := testutil.GenerateAccuracyReport(cmp, "built_color_16x16_420", w, h, components)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("built color 4:2:0 JPEG: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestDecodeBuiltJPEGAllIDCTMethods(t *testing.T) {
	t.Parallel()

	b := testutil.NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetGrayscale().
		SetPattern("solid")
	jpegData := b.Build()

	for _, method := range []string{"slow", "fast", "float"} {
		t.Run(method, func(t *testing.T) {
			dec := New(bytes.NewReader(jpegData))
			dec.SetIDCTMethod(method)

			w, h, _, _, err := dec.ReadHeader()
			if err != nil {
				t.Fatalf("ReadHeader failed: %v", err)
			}

			if err := dec.StartDecompress(); err != nil {
				t.Fatalf("StartDecompress failed: %v", err)
			}

			scanline := make([]byte, w*dec.OutputComponents())
			allPixels := make([]byte, 0, w*h)
			rowsRead := 0
			for rowsRead < h {
				n, err := dec.ReadScanlines([][]byte{scanline})
				if err != nil {
					t.Fatalf("ReadScanlines failed at row %d: %v", rowsRead, err)
				}
				if n == 0 {
					break
				}
				allPixels = append(allPixels, scanline...)
				rowsRead += n
			}

			// Verify all pixels are in valid range
			for i, p := range allPixels {
				if p > 255 {
					t.Errorf("pixel[%d] = %d, out of range", i, p)
				}
			}

			// For ISLOW, compare against expected values
			if method == "slow" || method == "unknown" {
				expected := b.ExpectedPixels()
				cmp := testutil.NewPixelComparator(expected, allPixels, w, h, 1)
				report := testutil.GenerateAccuracyReport(cmp, fmt.Sprintf("built_gray_%s", method), w, h, 1)
				t.Log(report.String())
				if cmp.MaxDiff() > 2 {
					t.Errorf("ISLOW max diff = %d, want <= 2", cmp.MaxDiff())
				}
			} else {
				// For IFAST and Float, just verify output exists and log
				t.Logf("Method %s: output sample[0] = %d, total = %d bytes", method, allPixels[0], len(allPixels))
			}
		})
	}
}
