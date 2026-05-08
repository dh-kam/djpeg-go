package djpeg_test

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"testing"

	djpeg "github.com/dh-kam/djpeg-go"
)

func TestDecodeRasterPublicAPI(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_color.jpg")
	if err != nil {
		t.Skipf("test fixture missing: %v", err)
	}

	cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRasterConfig failed: %v", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		t.Fatalf("invalid config dimensions: %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.PixelFormat != djpeg.PixelFormatRGB24 {
		t.Fatalf("pixel format = %s, want rgb24", cfg.PixelFormat)
	}

	raster, err := djpeg.DecodeRaster(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRaster failed: %v", err)
	}
	if raster.Rect.Dx() != cfg.Width || raster.Rect.Dy() != cfg.Height {
		t.Fatalf("raster dimensions = %dx%d, want %dx%d", raster.Rect.Dx(), raster.Rect.Dy(), cfg.Width, cfg.Height)
	}
	if raster.Stride != cfg.Width*3 {
		t.Fatalf("stride = %d, want %d", raster.Stride, cfg.Width*3)
	}
	if len(raster.Pix) != cfg.Height*raster.Stride {
		t.Fatalf("pixel length = %d, want %d", len(raster.Pix), cfg.Height*raster.Stride)
	}

	img, err := djpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !img.Bounds().Eq(raster.Bounds()) {
		t.Fatalf("Decode bounds = %v, want %v", img.Bounds(), raster.Bounds())
	}
}

func TestDecodeRasterGrayscalePublicAPI(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_gray.jpg")
	if err != nil {
		t.Skipf("test fixture missing: %v", err)
	}

	raster, err := djpeg.DecodeRaster(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRaster failed: %v", err)
	}
	if raster.Format != djpeg.PixelFormatGray8 {
		t.Fatalf("pixel format = %s, want gray8", raster.Format)
	}
	if raster.Stride != raster.Rect.Dx() {
		t.Fatalf("stride = %d, want %d", raster.Stride, raster.Rect.Dx())
	}
}

func TestNewDecoderPublicScanlineAPI(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("test fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithIDCT(djpeg.IDCTInt))
	cfg, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if cfg.Width != 8 || cfg.Height != 8 || cfg.PixelFormat != djpeg.PixelFormatGray8 {
		t.Fatalf("config = %+v, want 8x8 gray8", cfg)
	}
	if err := dec.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	out := dec.OutputConfig()
	row := make([]byte, out.Stride)
	rows := 0
	for rows < out.Height {
		n, err := dec.ReadScanlines([][]byte{row})
		if err != nil {
			t.Fatalf("ReadScanlines failed: %v", err)
		}
		if n == 0 {
			break
		}
		rows += n
	}
	if rows != out.Height {
		t.Fatalf("ReadScanlines read %d rows, want %d", rows, out.Height)
	}
	if err := dec.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

func TestDecodeRasterTurboFancyMatchesPopplerFixture(t *testing.T) {
	jpegData, err := os.ReadFile("tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	raster, err := djpeg.DecodeRaster(
		bytes.NewReader(jpegData),
		djpeg.WithIDCT(djpeg.IDCTInt),
		djpeg.WithTurboFancy(),
	)
	if err != nil {
		t.Fatalf("DecodeRaster turbo-fancy failed: %v", err)
	}
	if raster.Format != djpeg.PixelFormatRGB24 {
		t.Fatalf("pixel format = %s, want rgb24", raster.Format)
	}

	refPixels, refWidth, refHeight := loadPNGRGB(t, "tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/reference-poppler-pdfimages.png")
	if raster.Rect.Dx() != refWidth || raster.Rect.Dy() != refHeight {
		t.Fatalf("dimensions = %dx%d, want %dx%d", raster.Rect.Dx(), raster.Rect.Dy(), refWidth, refHeight)
	}
	if raster.Stride != refWidth*3 {
		t.Fatalf("stride = %d, want %d", raster.Stride, refWidth*3)
	}
	if !bytes.Equal(raster.Pix, refPixels) {
		t.Fatalf("turbo-fancy raster differs from Poppler reference")
	}
}

func TestDecodeProgressiveReturnsUnsupported(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_progressive.jpg")
	if err != nil {
		t.Skipf("test fixture missing: %v", err)
	}

	_, err = djpeg.DecodeRaster(bytes.NewReader(data))
	if !errors.Is(err, djpeg.ErrUnsupported) {
		t.Fatalf("DecodeRaster error = %v, want ErrUnsupported", err)
	}
}

func loadPNGRGB(t *testing.T, path string) ([]byte, int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open PNG reference: %v", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode PNG reference: %v", err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := make([]byte, width*height*3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			red, green, blue, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			i := (y*width + x) * 3
			pixels[i] = byte(red >> 8)
			pixels[i+1] = byte(green >> 8)
			pixels[i+2] = byte(blue >> 8)
		}
	}
	return pixels, width, height
}
