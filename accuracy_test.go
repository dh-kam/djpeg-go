package djpeggo_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/decoder"
	"github.com/dh-kam/djpeg-go/internal/testutil"
)

// decodeReferencePixels decodes a JPEG using Go's standard library as a
// reference implementation and returns the pixel data as bytes.
// For grayscale images, returns 1 byte per pixel.
// For color images, returns 3 bytes per pixel (RGB).
func decodeReferencePixels(t *testing.T, jpegData []byte) ([]byte, int, int, int) {
	t.Helper()

	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("Go stdlib jpeg.Decode failed: %v", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Convert to NRGBA for consistent pixel access
	nrgba := image.NewNRGBA(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			nrgba.Set(x, y, img.At(x, y))
		}
	}

	// Check if image is effectively grayscale
	isGray := true
	for y := 0; y < h && isGray; y++ {
		for x := 0; x < w && isGray; x++ {
			r, g, b, _ := nrgba.At(x, y).RGBA()
			if r != g || g != b {
				isGray = false
			}
		}
	}

	if isGray {
		pixels := make([]byte, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := color.GrayModel.Convert(nrgba.At(x, y)).(color.Gray)
				pixels[y*w+x] = c.Y
			}
		}
		return pixels, w, h, 1
	}

	pixels := make([]byte, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := nrgba.At(x, y).RGBA()
			idx := (y*w + x) * 3
			pixels[idx] = byte(r >> 8)
			pixels[idx+1] = byte(g >> 8)
			pixels[idx+2] = byte(b >> 8)
		}
	}
	return pixels, w, h, 3
}

func loadTestJPEG(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test file not available: %v", err)
	}
	return data
}

func TestExactMatchGrayscale(t *testing.T) {
	jpegData := loadTestJPEG(t, "test_gray.jpg")

	ourPixels, w1, h1, nc1, err := decoder.DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("djpeg-go DecodeToRGB failed: %v", err)
	}

	refPixels, w2, h2, nc2 := decodeReferencePixels(t, jpegData)

	if w1 != w2 || h1 != h2 {
		t.Fatalf("dimension mismatch: our=%dx%d ref=%dx%d", w1, h1, w2, h2)
	}
	if nc1 != nc2 {
		t.Fatalf("component count mismatch: our=%d ref=%d", nc1, nc2)
	}

	cmp := testutil.NewPixelComparator(refPixels, ourPixels, w1, h1, nc1)
	report := testutil.GenerateAccuracyReport(cmp, "grayscale_256x256", w1, h1, nc1)
	t.Log(report.String())

	// Note: Go stdlib uses a different IDCT than our IJG ISLOW port.
	// Differences in IDCT rounding are expected. Max diff of ~5 is acceptable.
	if cmp.MaxDiff() > 5 {
		t.Errorf("grayscale: max diff = %d, want <= 5", cmp.MaxDiff())
	}
}

func TestExactMatchColor444(t *testing.T) {
	jpegData := loadTestJPEG(t, "test_color.jpg")

	ourPixels, w1, h1, nc1, err := decoder.DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("djpeg-go DecodeToRGB failed: %v", err)
	}

	refPixels, w2, h2, nc2 := decodeReferencePixels(t, jpegData)

	if w1 != w2 || h1 != h2 {
		t.Fatalf("dimension mismatch: our=%dx%d ref=%dx%d", w1, h1, w2, h2)
	}
	if nc1 != nc2 {
		t.Fatalf("component count mismatch: our=%d ref=%d", nc1, nc2)
	}

	cmp := testutil.NewPixelComparator(refPixels, ourPixels, w1, h1, nc1)
	report := testutil.GenerateAccuracyReport(cmp, "color_256x256", w1, h1, nc1)
	t.Log(report.String())

	if cmp.MaxDiff() > 5 {
		t.Errorf("color: max diff = %d, want <= 5", cmp.MaxDiff())
	}
}

func TestExactMatchColor420(t *testing.T) {
	jpegData := loadTestJPEG(t, "test_420.jpg")

	ourPixels, w1, h1, nc1, err := decoder.DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("djpeg-go DecodeToRGB failed: %v", err)
	}

	refPixels, w2, h2, nc2 := decodeReferencePixels(t, jpegData)

	if w1 != w2 || h1 != h2 {
		t.Fatalf("dimension mismatch: our=%dx%d ref=%dx%d", w1, h1, w2, h2)
	}
	if nc1 != nc2 {
		t.Fatalf("component count mismatch: our=%d ref=%d", nc1, nc2)
	}

	cmp := testutil.NewPixelComparator(refPixels, ourPixels, w1, h1, nc1)
	report := testutil.GenerateAccuracyReport(cmp, "420_256x256", w1, h1, nc1)
	t.Log(report.String())

	// 4:2:0 subsampling has larger differences due to chroma interpolation
	if cmp.MaxDiff() > 5 {
		t.Errorf("4:2:0: max diff = %d, want <= 5", cmp.MaxDiff())
	}
}

func TestGoldenFileAccuracyGrayscale(t *testing.T) {
	t.Parallel()

	jpegData, err := os.ReadFile("testdata/gray_8x8.jpg")
	if err != nil {
		t.Skip("golden file not available:", err)
	}

	ourPixels, w, h, nc, err := decoder.DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	refPixels, refW, refH, refNc, err := testutil.LoadGoldenReference("testdata/gray_8x8.json")
	if err != nil {
		t.Fatalf("LoadGoldenReference failed: %v", err)
	}

	if w != refW || h != refH || nc != refNc {
		t.Fatalf("dimension mismatch: our=%dx%dx%d ref=%dx%dx%d", w, h, nc, refW, refH, refNc)
	}

	cmp := testutil.NewPixelComparator(refPixels, ourPixels, w, h, nc)
	report := testutil.GenerateAccuracyReport(cmp, "golden_gray_8x8", w, h, nc)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("golden grayscale: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestGoldenFileAccuracyColor444(t *testing.T) {
	t.Parallel()

	jpegData, err := os.ReadFile("testdata/color_8x8_444.jpg")
	if err != nil {
		t.Skip("golden file not available:", err)
	}

	ourPixels, w, h, nc, err := decoder.DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	refPixels, refW, refH, refNc, err := testutil.LoadGoldenReference("testdata/color_8x8_444.json")
	if err != nil {
		t.Fatalf("LoadGoldenReference failed: %v", err)
	}

	if w != refW || h != refH || nc != refNc {
		t.Fatalf("dimension mismatch: our=%dx%dx%d ref=%dx%dx%d", w, h, nc, refW, refH, refNc)
	}

	cmp := testutil.NewPixelComparator(refPixels, ourPixels, w, h, nc)
	report := testutil.GenerateAccuracyReport(cmp, "golden_color_8x8_444", w, h, nc)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("golden color 4:4:4: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}

func TestGoldenFileAccuracyColor420(t *testing.T) {
	jpegData, err := os.ReadFile("testdata/color_16x16_420.jpg")
	if err != nil {
		t.Skip("golden file not available:", err)
	}

	ourPixels, w, h, nc, err := decoder.DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}

	refPixels, refW, refH, refNc, err := testutil.LoadGoldenReference("testdata/color_16x16_420.json")
	if err != nil {
		t.Fatalf("LoadGoldenReference failed: %v", err)
	}

	if w != refW || h != refH || nc != refNc {
		t.Fatalf("dimension mismatch: our=%dx%dx%d ref=%dx%dx%d", w, h, nc, refW, refH, refNc)
	}

	cmp := testutil.NewPixelComparator(refPixels, ourPixels, w, h, nc)
	report := testutil.GenerateAccuracyReport(cmp, "golden_color_16x16_420", w, h, nc)
	t.Log(report.String())

	if cmp.MaxDiff() > 2 {
		t.Errorf("golden color 4:2:0: max diff = %d, want <= 2", cmp.MaxDiff())
	}
}
