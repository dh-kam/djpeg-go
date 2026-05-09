package djpeg_test

import (
	"bytes"
	"encoding/base64"
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

func TestDecodeRasterPopplerCompatibilityMatchesPopplerFixture(t *testing.T) {
	jpegData, err := os.ReadFile("tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	raster, err := djpeg.DecodeRaster(
		bytes.NewReader(jpegData),
		djpeg.WithCompatibility(djpeg.CompatibilityPopplerPDF),
	)
	if err != nil {
		t.Fatalf("DecodeRaster Poppler compatibility failed: %v", err)
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
		t.Fatalf("Poppler compatibility raster differs from Poppler reference")
	}
}

func TestTurboFancyAliasMatchesPopplerCompatibility(t *testing.T) {
	jpegData, err := os.ReadFile("tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	aliasRaster, err := djpeg.DecodeRaster(bytes.NewReader(jpegData), djpeg.WithTurboFancy())
	if err != nil {
		t.Fatalf("DecodeRaster WithTurboFancy failed: %v", err)
	}
	popplerRaster, err := djpeg.DecodeRaster(bytes.NewReader(jpegData), djpeg.WithCompatibility(djpeg.CompatibilityPopplerPDF))
	if err != nil {
		t.Fatalf("DecodeRaster Poppler compatibility failed: %v", err)
	}
	if !aliasRaster.Bounds().Eq(popplerRaster.Bounds()) || aliasRaster.Format != popplerRaster.Format {
		t.Fatalf("alias raster metadata = %v/%s, want %v/%s",
			aliasRaster.Bounds(), aliasRaster.Format, popplerRaster.Bounds(), popplerRaster.Format)
	}
	if !bytes.Equal(aliasRaster.Pix, popplerRaster.Pix) {
		t.Fatal("WithTurboFancy output differs from CompatibilityPopplerPDF output")
	}
}

func TestChromaIDCTScalingOverride(t *testing.T) {
	jpegData, err := os.ReadFile("tests/testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	popplerRaster, err := djpeg.DecodeRaster(bytes.NewReader(jpegData), djpeg.WithCompatibility(djpeg.CompatibilityPopplerPDF))
	if err != nil {
		t.Fatalf("DecodeRaster Poppler compatibility failed: %v", err)
	}
	directRaster, err := djpeg.DecodeRaster(bytes.NewReader(jpegData), djpeg.WithChromaIDCTScaling(false))
	if err != nil {
		t.Fatalf("DecodeRaster WithChromaIDCTScaling(false) failed: %v", err)
	}
	if !bytes.Equal(directRaster.Pix, popplerRaster.Pix) {
		t.Fatal("direct disabled chroma IDCT scaling differs from Poppler profile")
	}

	ijgRaster, err := djpeg.DecodeRaster(bytes.NewReader(jpegData), djpeg.WithCompatibility(djpeg.CompatibilityIJG9))
	if err != nil {
		t.Fatalf("DecodeRaster IJG compatibility failed: %v", err)
	}
	overrideRaster, err := djpeg.DecodeRaster(
		bytes.NewReader(jpegData),
		djpeg.WithCompatibility(djpeg.CompatibilityPopplerPDF),
		djpeg.WithChromaIDCTScaling(true),
	)
	if err != nil {
		t.Fatalf("DecodeRaster Poppler compatibility with chroma scaling override failed: %v", err)
	}
	if !bytes.Equal(overrideRaster.Pix, ijgRaster.Pix) {
		t.Fatal("enabled chroma IDCT scaling override differs from IJG profile")
	}
}

func TestDecodeRasterWithFastMatchesExplicitOptions(t *testing.T) {
	jpegData, err := os.ReadFile("tests/testdata/test_420.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	fastRaster, err := djpeg.DecodeRaster(bytes.NewReader(jpegData), djpeg.WithFast())
	if err != nil {
		t.Fatalf("DecodeRaster WithFast failed: %v", err)
	}
	explicitRaster, err := djpeg.DecodeRaster(
		bytes.NewReader(jpegData),
		djpeg.WithIDCT(djpeg.IDCTFast),
		djpeg.WithNoSmooth(),
	)
	if err != nil {
		t.Fatalf("DecodeRaster explicit fast/no-smooth failed: %v", err)
	}

	if !fastRaster.Bounds().Eq(explicitRaster.Bounds()) {
		t.Fatalf("bounds = %v, want %v", fastRaster.Bounds(), explicitRaster.Bounds())
	}
	if fastRaster.Format != explicitRaster.Format {
		t.Fatalf("format = %s, want %s", fastRaster.Format, explicitRaster.Format)
	}
	if !bytes.Equal(fastRaster.Pix, explicitRaster.Pix) {
		t.Fatal("WithFast output differs from explicit fast/no-smooth output")
	}
}

func TestDecodeRasterWithOutputColorSpace(t *testing.T) {
	colorData, err := os.ReadFile("tests/testdata/test_420.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	grayRaster, err := djpeg.DecodeRaster(bytes.NewReader(colorData), djpeg.WithGrayscaleOutput())
	if err != nil {
		t.Fatalf("DecodeRaster WithGrayscaleOutput failed: %v", err)
	}
	if grayRaster.Format != djpeg.PixelFormatGray8 {
		t.Fatalf("forced grayscale format = %s, want gray8", grayRaster.Format)
	}

	cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(colorData), djpeg.WithGrayscaleOutput())
	if err != nil {
		t.Fatalf("DecodeRasterConfig WithGrayscaleOutput failed: %v", err)
	}
	if cfg.PixelFormat != djpeg.PixelFormatGray8 || cfg.Components != 1 || cfg.Stride != cfg.Width {
		t.Fatalf("forced grayscale config = %+v, want gray8 one-byte stride", cfg)
	}
}

func TestDecodeRasterWithRGBOutputFromGrayscaleInput(t *testing.T) {
	grayData, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	grayRaster, err := djpeg.DecodeRaster(bytes.NewReader(grayData))
	if err != nil {
		t.Fatalf("DecodeRaster grayscale failed: %v", err)
	}
	rgbRaster, err := djpeg.DecodeRaster(bytes.NewReader(grayData), djpeg.WithRGBOutput())
	if err != nil {
		t.Fatalf("DecodeRaster WithRGBOutput failed: %v", err)
	}
	if rgbRaster.Format != djpeg.PixelFormatRGB24 {
		t.Fatalf("forced RGB format = %s, want rgb24", rgbRaster.Format)
	}
	if !rgbRaster.Bounds().Eq(grayRaster.Bounds()) {
		t.Fatalf("forced RGB bounds = %v, want %v", rgbRaster.Bounds(), grayRaster.Bounds())
	}
	for i, gray := range grayRaster.Pix {
		j := i * 3
		if rgbRaster.Pix[j] != gray || rgbRaster.Pix[j+1] != gray || rgbRaster.Pix[j+2] != gray {
			t.Fatalf("RGB pixel %d = [%d %d %d], want [%d %d %d]",
				i, rgbRaster.Pix[j], rgbRaster.Pix[j+1], rgbRaster.Pix[j+2], gray, gray, gray)
		}
	}
}

func TestDecodeRasterWithRGBInputAndGrayscaleOutput(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_color.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	rgbRaster, err := djpeg.DecodeRaster(
		bytes.NewReader(data),
		djpeg.WithInputColorSpace(djpeg.InputRGB),
		djpeg.WithRGBOutput(),
	)
	if err != nil {
		t.Fatalf("DecodeRaster forced RGB failed: %v", err)
	}
	grayRaster, err := djpeg.DecodeRaster(
		bytes.NewReader(data),
		djpeg.WithInputColorSpace(djpeg.InputRGB),
		djpeg.WithGrayscaleOutput(),
	)
	if err != nil {
		t.Fatalf("DecodeRaster forced RGB-to-gray failed: %v", err)
	}
	if grayRaster.Format != djpeg.PixelFormatGray8 {
		t.Fatalf("forced RGB-to-gray format = %s, want gray8", grayRaster.Format)
	}
	for i := range grayRaster.Pix {
		j := i * 3
		want := rgbToGray(rgbRaster.Pix[j], rgbRaster.Pix[j+1], rgbRaster.Pix[j+2])
		if grayRaster.Pix[i] != want {
			t.Fatalf("gray pixel %d = %d, want %d", i, grayRaster.Pix[i], want)
		}
	}
}

func TestDecodeRasterWithColorTransform(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_color.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	base, err := djpeg.DecodeRaster(
		bytes.NewReader(data),
		djpeg.WithInputColorSpace(djpeg.InputRGB),
		djpeg.WithRGBOutput(),
		djpeg.WithColorTransform(djpeg.ColorTransformNone),
	)
	if err != nil {
		t.Fatalf("DecodeRaster ColorTransformNone failed: %v", err)
	}
	transformed, err := djpeg.DecodeRaster(
		bytes.NewReader(data),
		djpeg.WithInputColorSpace(djpeg.InputRGB),
		djpeg.WithRGBOutput(),
		djpeg.WithColorTransform(djpeg.ColorTransformSubtractGreen),
	)
	if err != nil {
		t.Fatalf("DecodeRaster ColorTransformSubtractGreen failed: %v", err)
	}
	if transformed.Format != djpeg.PixelFormatRGB24 {
		t.Fatalf("transformed format = %s, want rgb24", transformed.Format)
	}
	for i := 0; i+2 < len(base.Pix); i += 3 {
		g := base.Pix[i+1]
		wantR := byte((int(base.Pix[i]) + int(g) - 128) & 0xff)
		wantB := byte((int(base.Pix[i+2]) + int(g) - 128) & 0xff)
		if transformed.Pix[i] != wantR || transformed.Pix[i+1] != g || transformed.Pix[i+2] != wantB {
			t.Fatalf("transformed pixel %d = [%d %d %d], want [%d %d %d]",
				i/3, transformed.Pix[i], transformed.Pix[i+1], transformed.Pix[i+2], wantR, g, wantB)
		}
		if i > 96 {
			break
		}
	}
}

func TestRasterCMYKFormat(t *testing.T) {
	raster := djpeg.NewRaster(1, 1, djpeg.PixelFormatCMYK32)
	copy(raster.Pix, []byte{0, 255, 255, 0})

	if raster.Format.Channels() != 4 {
		t.Fatalf("CMYK channels = %d, want 4", raster.Format.Channels())
	}
	if raster.Format.String() != "cmyk32" {
		t.Fatalf("CMYK string = %q, want cmyk32", raster.Format.String())
	}
	if raster.ColorModel() == nil {
		t.Fatal("CMYK ColorModel is nil")
	}
	if raster.RGBA().Bounds().Dx() != 1 {
		t.Fatal("CMYK RGBA conversion produced empty image")
	}
}

func TestDecodeRasterCMYKJPEG(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(tinyCMYKJPEGBase64)
	if err != nil {
		t.Fatalf("decode embedded CMYK JPEG: %v", err)
	}

	cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(data), djpeg.WithOutputColorSpace(djpeg.ColorSpaceCMYK))
	if err != nil {
		t.Fatalf("DecodeRasterConfig CMYK failed: %v", err)
	}
	if cfg.Width != 4 || cfg.Height != 1 || cfg.PixelFormat != djpeg.PixelFormatCMYK32 || cfg.Components != 4 {
		t.Fatalf("CMYK config = %+v, want 4x1 cmyk32", cfg)
	}

	raster, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithOutputColorSpace(djpeg.ColorSpaceCMYK))
	if err != nil {
		t.Fatalf("DecodeRaster CMYK failed: %v", err)
	}
	if raster.Format != djpeg.PixelFormatCMYK32 {
		t.Fatalf("CMYK raster format = %s, want cmyk32", raster.Format)
	}
	if raster.Stride != 16 || len(raster.Pix) != 16 {
		t.Fatalf("CMYK raster stride/len = %d/%d, want 16/16", raster.Stride, len(raster.Pix))
	}
	if bytes.Equal(raster.Pix, make([]byte, len(raster.Pix))) {
		t.Fatal("CMYK raster is all zero")
	}
}

func TestDecodeRasterCMYKJPEGForcedRGBAndGray(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(tinyCMYKJPEGBase64)
	if err != nil {
		t.Fatalf("decode embedded CMYK JPEG: %v", err)
	}

	rgbRaster, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithOutputColorSpace(djpeg.ColorSpaceRGB))
	if err != nil {
		t.Fatalf("DecodeRaster CMYK forced RGB failed: %v", err)
	}
	if rgbRaster.Format != djpeg.PixelFormatRGB24 || rgbRaster.Stride != 12 || len(rgbRaster.Pix) != 12 {
		t.Fatalf("forced RGB raster format/stride/len = %s/%d/%d, want rgb24/12/12",
			rgbRaster.Format, rgbRaster.Stride, len(rgbRaster.Pix))
	}
	grayRaster, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithOutputColorSpace(djpeg.ColorSpaceGray))
	if err != nil {
		t.Fatalf("DecodeRaster CMYK forced Gray failed: %v", err)
	}
	if grayRaster.Format != djpeg.PixelFormatGray8 || grayRaster.Stride != 4 || len(grayRaster.Pix) != 4 {
		t.Fatalf("forced Gray raster format/stride/len = %s/%d/%d, want gray8/4/4",
			grayRaster.Format, grayRaster.Stride, len(grayRaster.Pix))
	}
}

func TestAdobeAPP14FourComponentInference(t *testing.T) {
	tests := []struct {
		name      string
		transform byte
		want      djpeg.ColorSpace
	}{
		{name: "transform_0_cmyk", transform: 0, want: djpeg.ColorSpaceCMYK},
		{name: "transform_1_assume_ycck", transform: 1, want: djpeg.ColorSpaceYCCK},
		{name: "transform_2_ycck", transform: 2, want: djpeg.ColorSpaceYCCK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tinyAdobeCMYKJPEG(t, tt.transform, [4]byte{0x11, 0x12, 0x13, 0x14})
			cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("DecodeRasterConfig failed: %v", err)
			}
			if cfg.InputColorSpace != tt.want {
				t.Fatalf("InputColorSpace = %s, want %s", cfg.InputColorSpace, tt.want)
			}
			if cfg.ColorSpace != djpeg.ColorSpaceCMYK || cfg.PixelFormat != djpeg.PixelFormatCMYK32 {
				t.Fatalf("output config = %s/%s, want cmyk/cmyk32", cfg.ColorSpace, cfg.PixelFormat)
			}
		})
	}
}

func TestAdobeAPP14ComponentIDsPrecedeTransform(t *testing.T) {
	data := tinyAdobeCMYKJPEG(t, 0, [4]byte{1, 2, 3, 4})
	cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRasterConfig failed: %v", err)
	}
	if cfg.InputColorSpace != djpeg.ColorSpaceYCCK {
		t.Fatalf("InputColorSpace = %s, want ycck from component IDs", cfg.InputColorSpace)
	}
}

func TestAdobeAPP14UserOverridesPrecedeMarkerInference(t *testing.T) {
	transform2 := tinyAdobeCMYKJPEG(t, 2, [4]byte{0x11, 0x12, 0x13, 0x14})
	cfg, err := djpeg.DecodeRasterConfig(
		bytes.NewReader(transform2),
		djpeg.WithInputColorSpace(djpeg.InputCMYK),
	)
	if err != nil {
		t.Fatalf("DecodeRasterConfig input override failed: %v", err)
	}
	if cfg.InputColorSpace != djpeg.ColorSpaceCMYK {
		t.Fatalf("InputColorSpace override = %s, want cmyk", cfg.InputColorSpace)
	}

	transform0 := tinyAdobeCMYKJPEG(t, 0, [4]byte{0x11, 0x12, 0x13, 0x14})
	cfg, err = djpeg.DecodeRasterConfig(
		bytes.NewReader(transform0),
		djpeg.WithOutputColorSpace(djpeg.ColorSpaceRGB),
	)
	if err != nil {
		t.Fatalf("DecodeRasterConfig output override failed: %v", err)
	}
	if cfg.InputColorSpace != djpeg.ColorSpaceCMYK {
		t.Fatalf("InputColorSpace = %s, want marker-inferred cmyk", cfg.InputColorSpace)
	}
	if cfg.ColorSpace != djpeg.ColorSpaceRGB || cfg.PixelFormat != djpeg.PixelFormatRGB24 || cfg.Components != 3 {
		t.Fatalf("Output override config = %+v, want rgb/rgb24/3", cfg)
	}

	cfg, err = djpeg.DecodeRasterConfig(
		bytes.NewReader(transform0),
		djpeg.WithInputColorSpace(djpeg.InputYCCK),
		djpeg.WithOutputColorSpace(djpeg.ColorSpaceCMYK),
	)
	if err != nil {
		t.Fatalf("DecodeRasterConfig input+output override failed: %v", err)
	}
	if cfg.InputColorSpace != djpeg.ColorSpaceYCCK || cfg.ColorSpace != djpeg.ColorSpaceCMYK {
		t.Fatalf("override config = input %s output %s, want ycck/cmyk", cfg.InputColorSpace, cfg.ColorSpace)
	}
}

func TestDecoderSavedMarkers(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(tinyCMYKJPEGBase64)
	if err != nil {
		t.Fatalf("decode embedded CMYK JPEG: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithSavedMarkers(djpeg.MarkerAPP14, 65533))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	markers := dec.Markers()
	if len(markers) != 1 {
		t.Fatalf("saved marker count = %d, want 1", len(markers))
	}
	if markers[0].Code != djpeg.MarkerAPP14 {
		t.Fatalf("marker code = 0x%02x, want APP14", markers[0].Code)
	}
	if markers[0].OriginalLength != 12 || len(markers[0].Data) != 12 || !bytes.HasPrefix(markers[0].Data, []byte("Adobe")) {
		t.Fatalf("APP14 marker = len(original=%d saved=%d) data %q, want Adobe APP14",
			markers[0].OriginalLength, len(markers[0].Data), markers[0].Data)
	}

	markers[0].Data[0] = 0
	again := dec.Markers()
	if !bytes.HasPrefix(again[0].Data, []byte("Adobe")) {
		t.Fatal("Markers returned aliased marker data")
	}
}

func TestDecodeRasterConfigExposesHeaderMetadata(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(tinyCMYKJPEGBase64)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(data), djpeg.WithOutputColorSpace(djpeg.ColorSpaceCMYK))
	if err != nil {
		t.Fatalf("DecodeRasterConfig failed: %v", err)
	}
	if !cfg.Baseline || cfg.Progressive || cfg.Arithmetic {
		t.Fatalf("coding metadata baseline=%v progressive=%v arithmetic=%v, want baseline only",
			cfg.Baseline, cfg.Progressive, cfg.Arithmetic)
	}
	if cfg.HasMultipleScans || cfg.InputComplete {
		t.Fatalf("stream state multiple_scans=%v input_complete=%v, want false after header",
			cfg.HasMultipleScans, cfg.InputComplete)
	}
	if !cfg.SawJFIFMarker || cfg.JFIFMajorVersion != 1 || cfg.JFIFMinorVersion != 1 {
		t.Fatalf("JFIF metadata saw=%v version=%d.%02d, want 1.01 marker",
			cfg.SawJFIFMarker, cfg.JFIFMajorVersion, cfg.JFIFMinorVersion)
	}
	if !cfg.SawAdobeMarker || cfg.AdobeTransform != 2 {
		t.Fatalf("Adobe metadata saw=%v transform=%d, want transform 2",
			cfg.SawAdobeMarker, cfg.AdobeTransform)
	}
}

func TestDecoderStateMethodsAndAbort(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if dec.InputComplete() || dec.HasMultipleScans() || dec.IsBaseline() {
		t.Fatal("new decoder should not report parsed JPEG state before ReadHeader")
	}
	cfg, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if !dec.IsBaseline() || dec.IsProgressive() || dec.IsArithmetic() {
		t.Fatalf("decoder coding state baseline=%v progressive=%v arithmetic=%v, want baseline only",
			dec.IsBaseline(), dec.IsProgressive(), dec.IsArithmetic())
	}
	if cfg.HasMultipleScans != dec.HasMultipleScans() || cfg.InputComplete != dec.InputComplete() {
		t.Fatalf("config state multiple=%v complete=%v differs from decoder multiple=%v complete=%v",
			cfg.HasMultipleScans, cfg.InputComplete, dec.HasMultipleScans(), dec.InputComplete())
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	row := make([]byte, dec.OutputConfig().Stride)
	n, err := dec.ReadScanlines([][]byte{row})
	if err != nil {
		t.Fatalf("ReadScanlines failed: %v", err)
	}
	if n != 1 || dec.OutputScanline() != 1 {
		t.Fatalf("read rows=%d output_scanline=%d, want 1", n, dec.OutputScanline())
	}
	dec.Abort()
	if dec.Header() != (djpeg.Config{}) || dec.OutputConfig() != (djpeg.Config{}) || dec.OutputScanline() != 0 {
		t.Fatalf("Abort left header=%+v output=%+v output_scanline=%d, want cleared state",
			dec.Header(), dec.OutputConfig(), dec.OutputScanline())
	}
	if dec.InputComplete() || dec.HasMultipleScans() || dec.IsBaseline() {
		t.Fatal("Abort should clear public JPEG state")
	}
}

func TestSavedMarkersInvalidMarkerCode(t *testing.T) {
	dec := djpeg.NewDecoder(bytes.NewReader(nil), djpeg.WithSavedMarkers(0xd8, 10))
	if _, err := dec.ReadHeader(); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ReadHeader invalid saved marker error = %v, want ErrInvalidOption", err)
	}
}

func TestMarkerAPP(t *testing.T) {
	code, err := djpeg.MarkerAPP(2)
	if err != nil {
		t.Fatalf("MarkerAPP(2) failed: %v", err)
	}
	if code != djpeg.MarkerAPP2 {
		t.Fatalf("MarkerAPP(2) = 0x%02x, want APP2", code)
	}
	if _, err := djpeg.MarkerAPP(16); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("MarkerAPP(16) error = %v, want ErrInvalidOption", err)
	}
}

func TestDecodeRasterWithMaxMemory(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_420.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	if _, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithMaxMemory(1_000_000)); err != nil {
		t.Fatalf("DecodeRaster WithMaxMemory sufficient limit failed: %v", err)
	}
	_, err = djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithMaxMemory(1_000))
	if !errors.Is(err, djpeg.ErrMemoryLimit) {
		t.Fatalf("DecodeRaster WithMaxMemory small limit error = %v, want ErrMemoryLimit", err)
	}
}

const tinyCMYKJPEGBase64 = "/9j/4AAQSkZJRgABAQAAAAAAAAD/7gAOQWRvYmUAZAAAAAAC/9sAQwADAgICAgIDAgICAwMDAwQGBAQEBAQIBgYFBgkICgoJCAkJCgwPDAoLDgsJCQ0RDQ4PEBAREAoMEhMSEBMPEBAQ/9sAQwEDAwMEAwQIBAQIEAsJCxAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQ/8AAFAgAAQAEBAERAAIRAQMRAQQRAP/EABUAAQEAAAAAAAAAAAAAAAAAAAcJ/8QAGhAAAQUBAAAAAAAAAAAAAAAAAAMFCDeEtP/EABUBAQEAAAAAAAAAAAAAAAAAAAYI/8QAHxEAAAMJAAAAAAAAAAAAAAAAAAIEBQcINjeChLO0/9oADgQBAAIRAxEEAAA/ADmQlvv+XlSAzypoVWayCqYJaEsLK7FAqmf/2Q=="

func tinyAdobeCMYKJPEG(t *testing.T, transform byte, componentIDs [4]byte) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(tinyCMYKJPEGBase64)
	if err != nil {
		t.Fatalf("decode embedded CMYK JPEG: %v", err)
	}
	adobe := bytes.Index(data, []byte("Adobe"))
	if adobe < 0 || adobe+11 >= len(data) {
		t.Fatal("embedded CMYK JPEG missing Adobe APP14 marker")
	}
	data[adobe+11] = transform

	for pos := 2; pos+4 <= len(data); {
		if data[pos] != 0xff {
			t.Fatalf("invalid marker prefix at offset %d", pos)
		}
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos >= len(data) {
			break
		}
		marker := data[pos]
		pos++
		if marker == 0xd9 || marker == 0xda {
			if marker == 0xda {
				if pos+2 > len(data) {
					t.Fatal("truncated SOS marker")
				}
				length := int(data[pos])<<8 | int(data[pos+1])
				if pos+length > len(data) || length < 2+1+2*len(componentIDs)+3 {
					t.Fatal("invalid SOS marker length")
				}
				count := int(data[pos+2])
				if count != len(componentIDs) {
					t.Fatalf("SOS component count = %d, want %d", count, len(componentIDs))
				}
				for i, id := range componentIDs {
					data[pos+3+i*2] = id
				}
			}
			break
		}
		if pos+2 > len(data) {
			t.Fatalf("truncated marker 0x%x", marker)
		}
		length := int(data[pos])<<8 | int(data[pos+1])
		if length < 2 || pos+length > len(data) {
			t.Fatalf("invalid marker 0x%x length %d at offset %d", marker, length, pos)
		}
		if marker == 0xc0 {
			if length < 8+3*len(componentIDs) {
				t.Fatal("invalid SOF0 marker length")
			}
			count := int(data[pos+7])
			if count != len(componentIDs) {
				t.Fatalf("SOF0 component count = %d, want %d", count, len(componentIDs))
			}
			for i, id := range componentIDs {
				data[pos+8+i*3] = id
			}
		}
		pos += length
	}
	return data
}

func TestDecodeRasterWithScale(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_420.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(data), djpeg.WithScale(1, 2))
	if err != nil {
		t.Fatalf("DecodeRasterConfig WithScale failed: %v", err)
	}
	if cfg.Width != 128 || cfg.Height != 128 {
		t.Fatalf("scaled config dimensions = %dx%d, want 128x128", cfg.Width, cfg.Height)
	}

	raster, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithScale(1, 2))
	if err != nil {
		t.Fatalf("DecodeRaster WithScale failed: %v", err)
	}
	if raster.Rect.Dx() != 128 || raster.Rect.Dy() != 128 {
		t.Fatalf("scaled raster dimensions = %dx%d, want 128x128", raster.Rect.Dx(), raster.Rect.Dy())
	}

	upscaled, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithScale(16, 8))
	if err != nil {
		t.Fatalf("DecodeRaster WithScale upscale failed: %v", err)
	}
	if upscaled.Rect.Dx() != 512 || upscaled.Rect.Dy() != 512 {
		t.Fatalf("upscaled raster dimensions = %dx%d, want 512x512", upscaled.Rect.Dx(), upscaled.Rect.Dy())
	}
}

func TestScanlineDecoderWithScale(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithScale(1, 2))
	cfg, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader WithScale failed: %v", err)
	}
	if cfg.Width != 4 || cfg.Height != 4 || cfg.Stride != 4 {
		t.Fatalf("scaled header config = %+v, want 4x4 gray", cfg)
	}
	if err := dec.Start(); err != nil {
		t.Fatalf("Start WithScale failed: %v", err)
	}
	out := dec.OutputConfig()
	row := make([]byte, out.Stride)
	rows := 0
	for {
		n, err := dec.ReadScanlines([][]byte{row})
		if err != nil {
			t.Fatalf("ReadScanlines WithScale failed: %v", err)
		}
		if n == 0 {
			break
		}
		rows += n
	}
	if rows != 4 {
		t.Fatalf("scaled scanline count = %d, want 4", rows)
	}
	if err := dec.Finish(); err != nil {
		t.Fatalf("Finish WithScale failed: %v", err)
	}
}

func TestLibjpegNamedScanlineMethods(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	cfg, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if dec.Header() != cfg {
		t.Fatalf("Header() = %+v, want %+v", dec.Header(), cfg)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	out := dec.OutputConfig()
	row := make([]byte, out.Stride)
	rows := 0
	for dec.OutputScanline() < out.Height {
		n, err := dec.ReadScanlines([][]byte{row})
		if err != nil {
			t.Fatalf("ReadScanlines failed: %v", err)
		}
		if n == 0 {
			break
		}
		rows += n
	}
	if rows != out.Height || dec.OutputScanline() != out.Height {
		t.Fatalf("read rows=%d output_scanline=%d, want %d", rows, dec.OutputScanline(), out.Height)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestParseCompatibilityMode(t *testing.T) {
	tests := []struct {
		input string
		want  djpeg.CompatibilityMode
	}{
		{input: "", want: djpeg.CompatibilityDefault},
		{input: "default", want: djpeg.CompatibilityDefault},
		{input: "ijg9", want: djpeg.CompatibilityIJG9},
		{input: "libjpeg-9f", want: djpeg.CompatibilityIJG9},
		{input: "poppler-pdf", want: djpeg.CompatibilityPopplerPDF},
		{input: "imagemagick", want: djpeg.CompatibilityPopplerPDF},
		{input: "turbo-fancy", want: djpeg.CompatibilityPopplerPDF},
	}
	for _, tt := range tests {
		got, err := djpeg.ParseCompatibilityMode(tt.input)
		if err != nil {
			t.Fatalf("ParseCompatibilityMode(%q) failed: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseCompatibilityMode(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}

	if _, err := djpeg.ParseCompatibilityMode("bad"); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ParseCompatibilityMode invalid error = %v, want ErrInvalidOption", err)
	}
}

func TestParseColorTransform(t *testing.T) {
	tests := []struct {
		input string
		want  djpeg.ColorTransform
	}{
		{input: "", want: djpeg.ColorTransformDefault},
		{input: "auto", want: djpeg.ColorTransformDefault},
		{input: "none", want: djpeg.ColorTransformNone},
		{input: "0", want: djpeg.ColorTransformNone},
		{input: "subtract-green", want: djpeg.ColorTransformSubtractGreen},
		{input: "rgb1", want: djpeg.ColorTransformSubtractGreen},
	}
	for _, tt := range tests {
		got, err := djpeg.ParseColorTransform(tt.input)
		if err != nil {
			t.Fatalf("ParseColorTransform(%q) failed: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseColorTransform(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}

	if _, err := djpeg.ParseColorTransform("bad"); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ParseColorTransform invalid error = %v, want ErrInvalidOption", err)
	}
}

func TestParseColorSpaces(t *testing.T) {
	inputTests := []struct {
		input string
		want  djpeg.InputColorSpace
	}{
		{input: "gray", want: djpeg.InputGray},
		{input: "rgb", want: djpeg.InputRGB},
		{input: "ycbcr", want: djpeg.InputYCbCr},
		{input: "cmyk", want: djpeg.InputCMYK},
		{input: "ycck", want: djpeg.InputYCCK},
	}
	for _, tt := range inputTests {
		got, err := djpeg.ParseInputColorSpace(tt.input)
		if err != nil {
			t.Fatalf("ParseInputColorSpace(%q) failed: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseInputColorSpace(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}

	outputTests := []struct {
		input string
		want  djpeg.ColorSpace
	}{
		{input: "gray", want: djpeg.ColorSpaceGray},
		{input: "rgb", want: djpeg.ColorSpaceRGB},
		{input: "cmyk", want: djpeg.ColorSpaceCMYK},
		{input: "ycck", want: djpeg.ColorSpaceYCCK},
	}
	for _, tt := range outputTests {
		got, err := djpeg.ParseOutputColorSpace(tt.input)
		if err != nil {
			t.Fatalf("ParseOutputColorSpace(%q) failed: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseOutputColorSpace(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{input: "1", want: 1000},
		{input: "2k", want: 2000},
		{input: "3K", want: 3000},
		{input: "4m", want: 4_000_000},
		{input: "5MB", want: 5_000_000},
	}
	for _, tt := range tests {
		got, err := djpeg.ParseMemoryLimit(tt.input)
		if err != nil {
			t.Fatalf("ParseMemoryLimit(%q) failed: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseMemoryLimit(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}

	if _, err := djpeg.ParseMemoryLimit("bad"); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ParseMemoryLimit invalid error = %v, want ErrInvalidOption", err)
	}
}

func TestParseScale(t *testing.T) {
	tests := []struct {
		input     string
		numerator int
		denom     int
	}{
		{input: "1/2", numerator: 1, denom: 2},
		{input: "16/8", numerator: 16, denom: 8},
		{input: "2", numerator: 2, denom: 8},
	}
	for _, tt := range tests {
		numerator, denom, err := djpeg.ParseScale(tt.input)
		if err != nil {
			t.Fatalf("ParseScale(%q) failed: %v", tt.input, err)
		}
		if numerator != tt.numerator || denom != tt.denom {
			t.Fatalf("ParseScale(%q) = %d/%d, want %d/%d", tt.input, numerator, denom, tt.numerator, tt.denom)
		}
	}

	for _, input := range []string{"", "0/1", "1/0", "bad", "1/2/3"} {
		if _, _, err := djpeg.ParseScale(input); !errors.Is(err, djpeg.ErrInvalidOption) {
			t.Fatalf("ParseScale(%q) error = %v, want ErrInvalidOption", input, err)
		}
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

func rgbToGray(r, g, b byte) byte {
	const (
		redScale   = 19595
		greenScale = 38470
		blueScale  = 7471
		oneHalf    = 1 << 15
	)
	y := redScale*int(r) + greenScale*int(g) + blueScale*int(b) + oneHalf
	return byte(y >> 16)
}
