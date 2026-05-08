package testutil

import (
	"os"
	"testing"
)

// GenerateGoldenFiles generates golden test JPEG files and their reference data.
// This is intended to be called from a test with -update flag.
func GenerateGoldenFiles(t *testing.T, dir string) {
	t.Helper()

	// 1. gray_8x8.jpg: 8x8 grayscale with DC-only coefficient
	grayBuilder := NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetGrayscale().
		SetPattern("solid")
	grayJPEG := grayBuilder.Build()
	grayExpected := grayBuilder.ExpectedPixels()

	// 2. color_8x8_444.jpg: 8x8 YCbCr 4:4:4 with DC-only coefficients
	color444Builder := NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetColor().
		SetPattern("solid")
	color444JPEG := color444Builder.Build()
	color444Expected := color444Builder.ExpectedPixels()

	// 3. color_16x16_420.jpg: 16x16 YCbCr 4:2:0
	color420Builder := NewMinimalJPEGBuilder().
		SetDimensions(16, 16).
		SetColor().
		SetSubsampling(0, 2, 2).
		SetSubsampling(1, 1, 1).
		SetSubsampling(2, 1, 1).
		SetPattern("solid")
	color420JPEG := color420Builder.Build()
	color420Expected := color420Builder.ExpectedPixels()

	// Write JPEG files
	writeTestFile(t, dir+"/gray_8x8.jpg", grayJPEG)
	writeTestFile(t, dir+"/color_8x8_444.jpg", color444JPEG)
	writeTestFile(t, dir+"/color_16x16_420.jpg", color420JPEG)

	// Write reference JSON files
	writeGoldenJSON(t, dir+"/gray_8x8.json", GoldenTestData{
		Width:        8,
		Height:       8,
		Components:   1,
		PixelsBase64: encodeBase64(grayExpected),
		Description:  "8x8 grayscale solid gray (128), DC-only coefficient",
	})
	writeGoldenJSON(t, dir+"/color_8x8_444.json", GoldenTestData{
		Width:        8,
		Height:       8,
		Components:   3,
		PixelsBase64: encodeBase64(color444Expected),
		Description:  "8x8 YCbCr 4:4:4 solid gray (Y=128,Cb=128,Cr=128), DC-only",
	})
	writeGoldenJSON(t, dir+"/color_16x16_420.json", GoldenTestData{
		Width:        16,
		Height:       16,
		Components:   3,
		PixelsBase64: encodeBase64(color420Expected),
		Description:  "16x16 YCbCr 4:2:0 solid gray (Y=128,Cb=128,Cr=128), DC-only",
	})
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func writeGoldenJSON(t *testing.T, path string, gd GoldenTestData) {
	t.Helper()
	writeGoldenJSONE(path, gd)
}

// TestGenerateGoldenFiles generates golden test files when -update flag is set.
func TestGenerateGoldenFiles(t *testing.T) {
	if os.Getenv("UPDATE_GOLDEN") != "1" {
		t.Skip("Set UPDATE_GOLDEN=1 to regenerate golden test files")
	}
	GenerateGoldenFiles(t, "../../tests/testdata")
}
