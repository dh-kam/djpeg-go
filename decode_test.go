package djpeg_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/color"
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

func TestDecoderMutableDecompressionParameters(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_8x8_444.jpg")
	if err != nil {
		t.Skipf("test fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	header, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if header.PixelFormat != djpeg.PixelFormatRGB24 {
		t.Fatalf("header pixel format = %s, want rgb24", header.PixelFormat)
	}

	if err := dec.SetIDCT(djpeg.IDCTFast); err != nil {
		t.Fatalf("SetIDCT failed: %v", err)
	}
	if err := dec.SetUpsampling(djpeg.UpsamplingNearest); err != nil {
		t.Fatalf("SetUpsampling failed: %v", err)
	}
	if err := dec.SetCompatibility(djpeg.CompatibilityPopplerPDF); err != nil {
		t.Fatalf("SetCompatibility failed: %v", err)
	}
	if err := dec.SetChromaIDCTScaling(false); err != nil {
		t.Fatalf("SetChromaIDCTScaling failed: %v", err)
	}
	if err := dec.SetOutputColorSpace(djpeg.ColorSpaceGray); err != nil {
		t.Fatalf("SetOutputColorSpace failed: %v", err)
	}
	if err := dec.SetScale(1, 2); err != nil {
		t.Fatalf("SetScale failed: %v", err)
	}
	if err := dec.SetMaxMemory(0); err != nil {
		t.Fatalf("SetMaxMemory failed: %v", err)
	}

	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	out := dec.OutputConfig()
	if out.PixelFormat != djpeg.PixelFormatGray8 || out.Width != 4 || out.Height != 4 || out.Stride != 4 {
		t.Fatalf("output config = %+v, want 4x4 gray8", out)
	}

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
		t.Fatalf("read rows = %d, want %d", rows, out.Height)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
	if err := dec.SetOutputColorSpace(djpeg.ColorSpaceRGB); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("SetOutputColorSpace after start error = %v, want ErrInvalidOption", err)
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

func TestDecoderSaveMarkersMethod(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(tinyCMYKJPEGBase64)
	if err != nil {
		t.Fatalf("decode embedded CMYK JPEG: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if err := dec.SaveMarkers(djpeg.MarkerAPP14, 65533); err != nil {
		t.Fatalf("SaveMarkers failed: %v", err)
	}
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	markers := dec.Markers()
	if len(markers) != 1 || markers[0].Code != djpeg.MarkerAPP14 || !bytes.HasPrefix(markers[0].Data, []byte("Adobe")) {
		t.Fatalf("markers = %+v, want saved Adobe APP14", markers)
	}

	if err := djpeg.NewDecoder(bytes.NewReader(nil)).SaveMarkers(0xd8, 10); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("SaveMarkers invalid marker error = %v, want ErrInvalidOption", err)
	}
}

func TestDecoderMarkerProcessorOption(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	payload := []byte("processor-payload")
	data = insertHeaderMarker(data, djpeg.MarkerAPP2, payload)

	var got []djpeg.Marker
	dec := djpeg.NewDecoder(
		bytes.NewReader(data),
		djpeg.WithSavedMarkers(djpeg.MarkerAPP2, 65533),
		djpeg.WithMarkerProcessor(djpeg.MarkerAPP2, func(marker djpeg.Marker) error {
			got = append(got, djpeg.Marker{
				Code:           marker.Code,
				OriginalLength: marker.OriginalLength,
				Data:           append([]byte(nil), marker.Data...),
			})
			marker.Data[0] = 0
			return nil
		}),
	)
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("marker processor calls = %d, want 1", len(got))
	}
	if got[0].Code != djpeg.MarkerAPP2 || got[0].OriginalLength != uint(len(payload)) ||
		!bytes.Equal(got[0].Data, payload) {
		t.Fatalf("processed marker = %+v data %q, want APP2 payload %q", got[0], got[0].Data, payload)
	}
	if len(dec.Markers()) != 0 {
		t.Fatal("WithMarkerProcessor should override WithSavedMarkers for the same marker")
	}
}

func TestDecoderMarkerProcessorError(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	sentinel := errors.New("marker rejected")
	dec := djpeg.NewDecoder(
		bytes.NewReader(insertHeaderMarker(data, djpeg.MarkerAPP3, []byte("reject"))),
		djpeg.WithMarkerProcessor(djpeg.MarkerAPP3, func(marker djpeg.Marker) error {
			return sentinel
		}),
	)
	if _, err := dec.ReadHeader(); !errors.Is(err, sentinel) {
		t.Fatalf("ReadHeader marker processor error = %v, want sentinel", err)
	}
}

func TestDecodeRasterQuantizedOutput(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_8x8_444.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	cfg, err := djpeg.DecodeRasterConfig(
		bytes.NewReader(data),
		djpeg.WithQuantizeColors(8),
		djpeg.WithDitherMode(djpeg.DitherNone),
	)
	if err != nil {
		t.Fatalf("DecodeRasterConfig quantized failed: %v", err)
	}
	if !cfg.Quantized || cfg.PixelFormat != djpeg.PixelFormatIndexed8 || cfg.Components != 1 || cfg.Stride != cfg.Width {
		t.Fatalf("quantized config = %+v, want indexed8 one-component output", cfg)
	}
	if cfg.DesiredNumColors != 8 {
		t.Fatalf("DesiredNumColors = %d, want 8", cfg.DesiredNumColors)
	}

	raster, err := djpeg.DecodeRaster(
		bytes.NewReader(data),
		djpeg.WithQuantizeColors(8),
		djpeg.WithDitherMode(djpeg.DitherNone),
	)
	if err != nil {
		t.Fatalf("DecodeRaster quantized failed: %v", err)
	}
	if raster.Format != djpeg.PixelFormatIndexed8 || raster.Stride != raster.Rect.Dx() {
		t.Fatalf("quantized raster format=%s stride=%d width=%d", raster.Format, raster.Stride, raster.Rect.Dx())
	}
	if len(raster.Palette) == 0 || len(raster.Palette) > 8 {
		t.Fatalf("palette length = %d, want 1..8", len(raster.Palette))
	}
	for i, idx := range raster.Pix {
		if int(idx) >= len(raster.Palette) {
			t.Fatalf("pixel %d index=%d outside palette length %d", i, idx, len(raster.Palette))
		}
	}
}

func TestDecoderQuantizedScanlines(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_8x8_444.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithQuantizeColors(4))
	cfg, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader quantized failed: %v", err)
	}
	if cfg.PixelFormat != djpeg.PixelFormatIndexed8 || cfg.Stride != cfg.Width {
		t.Fatalf("ReadHeader quantized config = %+v, want indexed8 stride width", cfg)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress quantized failed: %v", err)
	}
	if len(dec.Palette()) == 0 || dec.OutputConfig().ActualNumColors != len(dec.Palette()) {
		t.Fatalf("palette length=%d output actual colors=%d", len(dec.Palette()), dec.OutputConfig().ActualNumColors)
	}
	row := make([]byte, dec.OutputConfig().Stride)
	n, err := dec.ReadScanlines([][]byte{row})
	if err != nil {
		t.Fatalf("ReadScanlines quantized failed: %v", err)
	}
	if n != 1 || dec.OutputScanline() != 1 {
		t.Fatalf("quantized read rows=%d output_scanline=%d, want 1", n, dec.OutputScanline())
	}
	if skipped, err := dec.SkipScanlines(100); err != nil || skipped != dec.OutputConfig().Height-1 {
		t.Fatalf("SkipScanlines quantized skipped=%d err=%v, want %d nil", skipped, err, dec.OutputConfig().Height-1)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress quantized failed: %v", err)
	}
}

func TestDecodeRasterExternalColormap(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	palette := color.Palette{color.Gray{Y: 0}, color.Gray{Y: 255}}
	raster, err := djpeg.DecodeRaster(
		bytes.NewReader(data),
		djpeg.WithColormap(palette),
		djpeg.WithDitherMode(djpeg.DitherNone),
	)
	if err != nil {
		t.Fatalf("DecodeRaster external colormap failed: %v", err)
	}
	if raster.Format != djpeg.PixelFormatIndexed8 || len(raster.Palette) != len(palette) {
		t.Fatalf("external colormap raster format=%s palette=%d, want indexed8/%d",
			raster.Format, len(raster.Palette), len(palette))
	}
	for i, idx := range raster.Pix {
		if idx > 1 {
			t.Fatalf("pixel %d index=%d outside external colormap", i, idx)
		}
	}
}

func TestDecoderNewColormap(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_8x8_444.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(
		bytes.NewReader(data),
		djpeg.WithQuantizeColors(8),
		djpeg.WithDitherMode(djpeg.DitherNone),
	)
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	if len(dec.Palette()) == 0 {
		t.Fatal("initial quantized palette is empty")
	}
	row := make([]byte, dec.OutputConfig().Stride)
	if n, err := dec.ReadScanlines([][]byte{row}); err != nil || n != 1 {
		t.Fatalf("initial ReadScanlines rows=%d err=%v, want 1 nil", n, err)
	}
	if dec.OutputScanline() != 1 {
		t.Fatalf("OutputScanline before NewColormap = %d, want 1", dec.OutputScanline())
	}

	newPalette := color.Palette{color.Black, color.White}
	if err := dec.NewColormap(newPalette); err != nil {
		t.Fatalf("NewColormap failed: %v", err)
	}
	if dec.OutputScanline() != 0 {
		t.Fatalf("OutputScanline after NewColormap = %d, want 0", dec.OutputScanline())
	}
	if len(dec.Palette()) != len(newPalette) || dec.OutputConfig().ActualNumColors != len(newPalette) {
		t.Fatalf("palette length=%d actual=%d, want %d",
			len(dec.Palette()), dec.OutputConfig().ActualNumColors, len(newPalette))
	}
	row = make([]byte, dec.OutputConfig().Stride)
	if n, err := dec.ReadScanlines([][]byte{row}); err != nil || n != 1 {
		t.Fatalf("ReadScanlines after NewColormap rows=%d err=%v, want 1 nil", n, err)
	}
	for i, idx := range row {
		if idx >= byte(len(newPalette)) {
			t.Fatalf("row pixel %d index=%d outside new palette length %d", i, idx, len(newPalette))
		}
	}
	if _, err := dec.SkipScanlines(100); err != nil {
		t.Fatalf("SkipScanlines after NewColormap failed: %v", err)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderNewColormapInvalid(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithQuantizeColors(8))
	if err := dec.NewColormap(color.Palette{color.Black, color.White}); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("NewColormap before Start error = %v, want ErrInvalidOption", err)
	}

	dec = djpeg.NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	if err := dec.NewColormap(color.Palette{color.Black, color.White}); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("NewColormap non-quantized error = %v, want ErrInvalidOption", err)
	}
	dec.Abort()

	dec = djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithQuantizeColors(8))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader quantized failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress quantized failed: %v", err)
	}
	if err := dec.NewColormap(color.Palette{}); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("NewColormap empty palette error = %v, want ErrInvalidOption", err)
	}
	if err := dec.NewColormap(color.Palette{color.Black}); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("NewColormap single-entry palette error = %v, want ErrInvalidOption", err)
	}
}

func TestDecoderBufferedImageOutputPasses(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	reference, err := djpeg.DecodeRaster(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRaster reference failed: %v", err)
	}
	bufferedRaster, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithBufferedImage())
	if err != nil {
		t.Fatalf("DecodeRaster WithBufferedImage failed: %v", err)
	}
	if !bytes.Equal(bufferedRaster.Pix, reference.Pix) {
		t.Fatal("DecodeRaster WithBufferedImage differs from direct DecodeRaster output")
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithBufferedImage())
	cfg, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if !cfg.BufferedImage {
		t.Fatalf("ReadHeader BufferedImage = false")
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	if !dec.OutputConfig().BufferedImage || !dec.OutputConfig().InputComplete {
		t.Fatalf("output config buffered/input_complete = %v/%v, want true/true",
			dec.OutputConfig().BufferedImage, dec.OutputConfig().InputComplete)
	}
	if _, err := dec.ReadScanlines([][]byte{make([]byte, dec.OutputConfig().Stride)}); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ReadScanlines before StartOutput error = %v, want ErrInvalidOption", err)
	}

	ok, err := dec.StartOutput(0)
	if err != nil || !ok {
		t.Fatalf("StartOutput(0) ok=%v err=%v, want true nil", ok, err)
	}
	if dec.OutputScanNumber() != 1 || dec.OutputScanline() != 0 {
		t.Fatalf("output scan number/line = %d/%d, want 1/0", dec.OutputScanNumber(), dec.OutputScanline())
	}
	got := djpeg.NewRaster(dec.OutputConfig().Width, dec.OutputConfig().Height, dec.OutputConfig().PixelFormat)
	for y := 0; y < dec.OutputConfig().Height; y++ {
		row := got.Pix[y*got.Stride : y*got.Stride+got.Stride]
		n, err := dec.ReadScanlines([][]byte{row})
		if err != nil || n != 1 {
			t.Fatalf("ReadScanlines pass1 row %d rows=%d err=%v, want 1 nil", y, n, err)
		}
	}
	if !bytes.Equal(got.Pix, reference.Pix) {
		t.Fatal("buffered output pass pixels differ from direct DecodeRaster output")
	}
	ok, err = dec.FinishOutput()
	if err != nil || !ok {
		t.Fatalf("FinishOutput pass1 ok=%v err=%v, want true nil", ok, err)
	}

	ok, err = dec.StartOutput(99)
	if err != nil || !ok {
		t.Fatalf("StartOutput(99) ok=%v err=%v, want true nil", ok, err)
	}
	if dec.OutputScanNumber() != dec.InputScanNumber() {
		t.Fatalf("OutputScanNumber = %d, want clamped input scan %d",
			dec.OutputScanNumber(), dec.InputScanNumber())
	}
	row := make([]byte, dec.OutputConfig().Stride)
	if n, err := dec.ReadScanlines([][]byte{row}); err != nil || n != 1 {
		t.Fatalf("ReadScanlines pass2 first row rows=%d err=%v, want 1 nil", n, err)
	}
	if !bytes.Equal(row, reference.Pix[:reference.Stride]) {
		t.Fatal("buffered second pass first row differs from reference")
	}
	if ok, err := dec.FinishOutput(); err != nil || !ok {
		t.Fatalf("FinishOutput pass2 ok=%v err=%v, want true nil", ok, err)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderBufferedImageInvalidLifecycle(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if ok, err := dec.StartOutput(1); !errors.Is(err, djpeg.ErrInvalidOption) || ok {
		t.Fatalf("StartOutput without buffered mode ok=%v err=%v, want false ErrInvalidOption", ok, err)
	}

	dec = djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithBufferedImage(), djpeg.WithQuantizeColors(4))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader buffered quantized failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress buffered quantized failed: %v", err)
	}
	if ok, err := dec.StartOutput(1); err != nil || !ok {
		t.Fatalf("StartOutput buffered quantized ok=%v err=%v, want true nil", ok, err)
	}
	if err := dec.NewColormap(color.Palette{color.Black, color.White}); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("NewColormap during output pass error = %v, want ErrInvalidOption", err)
	}
	if err := dec.FinishDecompress(); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("FinishDecompress during output pass error = %v, want ErrInvalidOption", err)
	}
	if ok, err := dec.FinishOutput(); err != nil || !ok {
		t.Fatalf("FinishOutput ok=%v err=%v, want true nil", ok, err)
	}
	if err := dec.NewColormap(color.Palette{color.Black, color.White}); err != nil {
		t.Fatalf("NewColormap between output passes failed: %v", err)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress after FinishOutput failed: %v", err)
	}
}

func TestDecodeRasterQuantizedInvalidOptions(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if _, err := djpeg.DecodeRasterConfig(bytes.NewReader(data), djpeg.WithQuantizeColors(1)); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeRasterConfig WithQuantizeColors(1) error = %v, want ErrInvalidOption", err)
	}

	cmykData, err := base64.StdEncoding.DecodeString(tinyCMYKJPEGBase64)
	if err != nil {
		t.Fatal(err)
	}
	_, err = djpeg.DecodeRasterConfig(
		bytes.NewReader(cmykData),
		djpeg.WithOutputColorSpace(djpeg.ColorSpaceCMYK),
		djpeg.WithQuantizeColors(8),
	)
	if !errors.Is(err, djpeg.ErrUnsupported) {
		t.Fatalf("DecodeRasterConfig CMYK quantized error = %v, want ErrUnsupported", err)
	}
}

func TestDecodeRawComponents(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_16x16_420.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	components, cfg, err := djpeg.DecodeRawComponents(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRawComponents failed: %v", err)
	}
	if !cfg.RawDataOut {
		t.Fatalf("RawDataOut = false in config %+v", cfg)
	}
	if len(components) != 3 {
		t.Fatalf("raw component count = %d, want 3", len(components))
	}
	for i, comp := range components {
		if comp.Width <= 0 || comp.Height <= 0 || comp.Stride < comp.Width {
			t.Fatalf("component %d dimensions width=%d height=%d stride=%d", i, comp.Width, comp.Height, comp.Stride)
		}
		if len(comp.Pix) != comp.Height*comp.Stride {
			t.Fatalf("component %d pix len=%d, want %d", i, len(comp.Pix), comp.Height*comp.Stride)
		}
		if comp.Component.Index != i {
			t.Fatalf("component %d metadata index = %d", i, comp.Component.Index)
		}
	}
	if components[0].Width < components[1].Width || components[0].Height < components[1].Height {
		t.Fatalf("Y component %dx%d should be at least chroma %dx%d",
			components[0].Width, components[0].Height, components[1].Width, components[1].Height)
	}
}

func TestDecoderRawDataLifecycle(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_8x8_444.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithRawDataOutput())
	cfg, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader raw failed: %v", err)
	}
	if !cfg.RawDataOut {
		t.Fatalf("ReadHeader RawDataOut = false")
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress raw failed: %v", err)
	}
	if _, err := dec.ReadScanlines([][]byte{make([]byte, dec.OutputConfig().Stride)}); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ReadScanlines in raw mode error = %v, want ErrInvalidOption", err)
	}
	components, err := dec.ReadRawData()
	if err != nil {
		t.Fatalf("ReadRawData failed: %v", err)
	}
	if len(components) != 3 {
		t.Fatalf("ReadRawData components = %d, want 3", len(components))
	}
	if dec.OutputScanline() != dec.OutputConfig().Height {
		t.Fatalf("OutputScanline after ReadRawData = %d, want %d", dec.OutputScanline(), dec.OutputConfig().Height)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress raw failed: %v", err)
	}
}

func TestDecoderRawDataRows(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_16x16_420.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	all, cfg, err := djpeg.DecodeRawComponents(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRawComponents reference failed: %v", err)
	}
	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithRawDataOutput())
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader raw rows failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress raw rows failed: %v", err)
	}
	linesPerIMCU := dec.RawDataLinesPerIMCURow()
	if linesPerIMCU != cfg.MaxVSampFactor*cfg.MinDCTVScaledSize {
		t.Fatalf("RawDataLinesPerIMCURow = %d, want %d",
			linesPerIMCU, cfg.MaxVSampFactor*cfg.MinDCTVScaledSize)
	}
	if _, _, err := dec.ReadRawDataRows(linesPerIMCU - 1); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ReadRawDataRows small buffer error = %v, want ErrInvalidOption", err)
	}

	components, rows, err := dec.ReadRawDataRows(linesPerIMCU)
	if err != nil {
		t.Fatalf("ReadRawDataRows failed: %v", err)
	}
	if rows != linesPerIMCU {
		t.Fatalf("ReadRawDataRows rows = %d, want %d", rows, linesPerIMCU)
	}
	if len(components) != len(all) {
		t.Fatalf("ReadRawDataRows components = %d, want %d", len(components), len(all))
	}
	for i := range components {
		if components[i].Width != all[i].Width || components[i].Height != all[i].Height ||
			components[i].Stride != all[i].Stride || !bytes.Equal(components[i].Pix, all[i].Pix) {
			t.Fatalf("raw row component %d differs from all-at-once component", i)
		}
	}
	if dec.OutputScanline() != linesPerIMCU {
		t.Fatalf("OutputScanline after ReadRawDataRows = %d, want %d", dec.OutputScanline(), linesPerIMCU)
	}
	if components, rows, err := dec.ReadRawDataRows(linesPerIMCU); err != nil || rows != 0 || components != nil {
		t.Fatalf("ReadRawDataRows after end components=%v rows=%d err=%v, want nil/0/nil", components, rows, err)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress raw rows failed: %v", err)
	}
}

func TestDecoderProgressMonitor(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	var updates []djpeg.Progress
	dec := djpeg.NewDecoder(
		bytes.NewReader(data),
		djpeg.WithProgressMonitor(func(progress djpeg.Progress) {
			updates = append(updates, progress)
		}),
	)
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	row := make([]byte, dec.OutputConfig().Stride)
	if n, err := dec.ReadScanlines([][]byte{row}); err != nil || n != 1 {
		t.Fatalf("ReadScanlines rows=%d err=%v, want 1 nil", n, err)
	}
	if skipped, err := dec.SkipScanlines(2); err != nil || skipped != 2 {
		t.Fatalf("SkipScanlines skipped=%d err=%v, want 2 nil", skipped, err)
	}
	if len(updates) < 2 {
		t.Fatalf("progress updates = %d, want at least 2", len(updates))
	}
	if updates[0].PassCounter != 0 || updates[0].PassLimit != int64(dec.OutputConfig().Height) ||
		updates[0].CompletedPasses != 0 || updates[0].TotalPasses != 1 {
		t.Fatalf("first progress update = %+v, want counter 0 limit %d passes 0/1",
			updates[0], dec.OutputConfig().Height)
	}
	if updates[1].PassCounter != 1 || updates[1].PassLimit != int64(dec.OutputConfig().Height) {
		t.Fatalf("second progress update = %+v, want counter 1 limit %d",
			updates[1], dec.OutputConfig().Height)
	}
}

func TestDecodeRawComponentsInvalidOptions(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if _, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithRawDataOutput()); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeRaster raw mode error = %v, want ErrInvalidOption", err)
	}
	if _, _, err := djpeg.DecodeRawComponents(bytes.NewReader(data), djpeg.WithQuantizeColors(8)); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeRawComponents quantized error = %v, want ErrInvalidOption", err)
	}
	if _, _, err := djpeg.DecodeRawComponents(bytes.NewReader(data), djpeg.WithScale(1, 2)); !errors.Is(err, djpeg.ErrUnsupported) {
		t.Fatalf("DecodeRawComponents scaled error = %v, want ErrUnsupported", err)
	}
}

func TestDecodeCoefficients(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	components, cfg, err := djpeg.DecodeCoefficients(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeCoefficients failed: %v", err)
	}
	if cfg.Width != 8 || cfg.Height != 8 || cfg.InputComponents != 1 {
		t.Fatalf("coefficient config = %+v, want 8x8 one-component", cfg)
	}
	if len(components) != 1 {
		t.Fatalf("coefficient components = %d, want 1", len(components))
	}
	comp := components[0]
	if comp.WidthInBlocks != 1 || comp.HeightInBlocks != 1 || len(comp.Blocks) != 1 {
		t.Fatalf("coefficient block layout = %dx%d len=%d, want 1x1 len=1",
			comp.WidthInBlocks, comp.HeightInBlocks, len(comp.Blocks))
	}
	for i, coef := range comp.Blocks[0] {
		if coef != 0 {
			t.Fatalf("gray_8x8 coefficient %d = %d, want 0", i, coef)
		}
	}
}

func TestDecoderReadCoefficientsLifecycle(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/color_8x8_444.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	components, err := dec.ReadCoefficients()
	if err != nil {
		t.Fatalf("ReadCoefficients failed: %v", err)
	}
	if !dec.InputComplete() {
		t.Fatal("InputComplete should be true after ReadCoefficients")
	}
	if len(components) != 3 {
		t.Fatalf("coefficient components = %d, want 3", len(components))
	}
	for i, comp := range components {
		if comp.Component.Index != i || comp.WidthInBlocks != 1 || comp.HeightInBlocks != 1 || len(comp.Blocks) != 1 {
			t.Fatalf("component %d layout = index %d %dx%d len=%d, want index %d 1x1 len=1",
				i, comp.Component.Index, comp.WidthInBlocks, comp.HeightInBlocks, len(comp.Blocks), i)
		}
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress after ReadCoefficients failed: %v", err)
	}
}

func TestDecodeCoefficientsInvalidOptions(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	if _, _, err := djpeg.DecodeCoefficients(bytes.NewReader(data), djpeg.WithRawDataOutput()); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeCoefficients raw data error = %v, want ErrInvalidOption", err)
	}
	if _, _, err := djpeg.DecodeCoefficients(bytes.NewReader(data), djpeg.WithQuantizeColors(8)); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeCoefficients quantized error = %v, want ErrInvalidOption", err)
	}
	if _, _, err := djpeg.DecodeCoefficients(bytes.NewReader(data), djpeg.WithScale(1, 2)); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeCoefficients scaled error = %v, want ErrInvalidOption", err)
	}
}

func TestDecoderReadCoefficientsProgressiveUnsupported(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_progressive.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if _, status, err := dec.ReadHeaderRequireImage(true); err != nil || status != djpeg.HeaderOK {
		t.Fatalf("ReadHeaderRequireImage progressive status=%v err=%v, want ok nil", status, err)
	}
	if _, err := dec.ReadCoefficients(); !errors.Is(err, djpeg.ErrUnsupported) {
		t.Fatalf("ReadCoefficients progressive error = %v, want ErrUnsupported", err)
	}
}

func TestDecoderTables(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if _, ok := dec.QuantizationTable(0); ok {
		t.Fatal("QuantizationTable should be unavailable before ReadHeader")
	}
	if _, ok := dec.HuffmanTable(0, djpeg.HuffmanTableDC); ok {
		t.Fatal("HuffmanTable should be unavailable before ReadHeader")
	}
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}

	qt, ok := dec.QuantizationTable(0)
	if !ok {
		t.Fatal("QuantizationTable(0) not found")
	}
	if qt.Values[0] == 0 {
		t.Fatalf("QuantizationTable(0).Values[0] = 0, want parsed value")
	}
	qt.Values[0] = 0
	again, ok := dec.QuantizationTable(0)
	if !ok || again.Values[0] == 0 {
		t.Fatal("QuantizationTable returned mutable decoder-owned data")
	}

	dc, ok := dec.HuffmanTable(0, djpeg.HuffmanTableDC)
	if !ok {
		t.Fatal("HuffmanTable(0, DC) not found")
	}
	if dc.Bits[0] != 0 {
		t.Fatalf("HuffmanTable(0, DC).Bits[0] = %d, want 0", dc.Bits[0])
	}
	if _, ok := dec.HuffmanTable(99, djpeg.HuffmanTableDC); ok {
		t.Fatal("HuffmanTable out-of-range lookup unexpectedly found a table")
	}
	if _, ok := dec.HuffmanTable(0, djpeg.HuffmanTableClass(99)); ok {
		t.Fatal("HuffmanTable invalid class unexpectedly found a table")
	}
}

func TestDecoderComponents(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if got := dec.Components(); len(got) != 0 {
		t.Fatalf("Components before ReadHeader length = %d, want 0", len(got))
	}
	if dec.RestartInterval() != 0 {
		t.Fatalf("RestartInterval before ReadHeader = %d, want 0", dec.RestartInterval())
	}
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}

	components := dec.Components()
	if len(components) != 1 {
		t.Fatalf("Components length = %d, want 1", len(components))
	}
	comp := components[0]
	if comp.ID != 1 || comp.Index != 0 || comp.HSampFactor != 1 || comp.VSampFactor != 1 {
		t.Fatalf("component identity/sampling = %+v, want id=1 index=0 h=1 v=1", comp)
	}
	if comp.QuantizationTableIndex != 0 || comp.DCHuffmanTableIndex != 0 || comp.ACHuffmanTableIndex != 0 {
		t.Fatalf("component table indexes = q:%d dc:%d ac:%d, want 0/0/0",
			comp.QuantizationTableIndex, comp.DCHuffmanTableIndex, comp.ACHuffmanTableIndex)
	}
	components[0].ID = 99
	if again := dec.Components(); again[0].ID != 1 {
		t.Fatalf("Components returned mutable decoder-owned data: id=%d, want 1", again[0].ID)
	}
	if dec.RestartInterval() != 0 {
		t.Fatalf("RestartInterval = %d, want 0 for fixture without DRI", dec.RestartInterval())
	}
}

func TestDecoderCalcOutputDimensions(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithScale(1, 2), djpeg.WithRGBOutput())
	cfg, err := dec.CalcOutputDimensions()
	if err != nil {
		t.Fatalf("CalcOutputDimensions failed: %v", err)
	}
	if cfg.Width != 4 || cfg.Height != 4 || cfg.PixelFormat != djpeg.PixelFormatRGB24 || cfg.Stride != 12 {
		t.Fatalf("CalcOutputDimensions config = %+v, want 4x4 rgb24 stride 12", cfg)
	}
	if cfg.ImageWidth != 8 || cfg.ImageHeight != 8 || cfg.DataPrecision != 8 {
		t.Fatalf("CalcOutputDimensions header fields = image %dx%d precision %d, want 8x8 precision 8",
			cfg.ImageWidth, cfg.ImageHeight, cfg.DataPrecision)
	}
	if cfg.MaxHSampFactor != 1 || cfg.MaxVSampFactor != 1 || cfg.MinDCTHScaledSize != 8 || cfg.MinDCTVScaledSize != 8 {
		t.Fatalf("CalcOutputDimensions sampling fields = max %dx%d minDCT %dx%d, want 1x1 and 8x8",
			cfg.MaxHSampFactor, cfg.MaxVSampFactor, cfg.MinDCTHScaledSize, cfg.MinDCTVScaledSize)
	}
	if cfg.BlockSize != 8 || cfg.ScaleNum != 8 || cfg.ScaleDenom != 8 {
		t.Fatalf("CalcOutputDimensions block/scale = block %d scale %d/%d, want 8 and marker scale 8/8",
			cfg.BlockSize, cfg.ScaleNum, cfg.ScaleDenom)
	}
	if cfg.RecOutbufHeight != 1 {
		t.Fatalf("RecOutbufHeight = %d, want 1", cfg.RecOutbufHeight)
	}
	if _, _, err := dec.CropScanline(0, 1); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("CropScanline after CalcOutputDimensions before Start error = %v, want ErrInvalidOption", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	out := dec.OutputConfig()
	if out.Width != cfg.Width || out.Height != cfg.Height || out.Stride != cfg.Stride || out.RecOutbufHeight != cfg.RecOutbufHeight {
		t.Fatalf("OutputConfig after Start = %+v, want dimensions from CalcOutputDimensions %+v", out, cfg)
	}
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
	if rows != out.Height {
		t.Fatalf("rows read = %d, want %d", rows, out.Height)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderConsumeInput(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	status, err := dec.ConsumeInput()
	if err != nil {
		t.Fatalf("ConsumeInput failed: %v", err)
	}
	if status != djpeg.InputReachedSOS || status.String() != "reached-sos" {
		t.Fatalf("ConsumeInput status = %v (%s), want reached SOS", status, status.String())
	}
	cfg := dec.Header()
	if cfg.Width != 8 || cfg.Height != 8 || cfg.PixelFormat != djpeg.PixelFormatGray8 {
		t.Fatalf("Header after ConsumeInput = %+v, want 8x8 gray", cfg)
	}
	if status, err = dec.ConsumeInput(); err != nil || status != djpeg.InputReachedSOS {
		t.Fatalf("second ConsumeInput status=%v err=%v, want reached SOS nil", status, err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress after ConsumeInput failed: %v", err)
	}
	row := make([]byte, dec.OutputConfig().Stride)
	rows := 0
	for dec.OutputScanline() < dec.OutputConfig().Height {
		n, err := dec.ReadScanlines([][]byte{row})
		if err != nil {
			t.Fatalf("ReadScanlines failed: %v", err)
		}
		if n == 0 {
			break
		}
		rows += n
	}
	if rows != cfg.Height {
		t.Fatalf("rows read = %d, want %d", rows, cfg.Height)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderConsumeInputTablesOnly(t *testing.T) {
	dec := djpeg.NewDecoder(bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xd9}))
	status, err := dec.ConsumeInput()
	if err != nil {
		t.Fatalf("ConsumeInput tables-only failed: %v", err)
	}
	if status != djpeg.InputReachedEOI || status.String() != "reached-eoi" {
		t.Fatalf("ConsumeInput tables-only status = %v (%s), want reached EOI", status, status.String())
	}
	if !dec.InputComplete() {
		t.Fatal("InputComplete should be true after tables-only EOI")
	}
	if dec.Header() != (djpeg.Config{}) {
		t.Fatalf("Header after tables-only ConsumeInput = %+v, want empty", dec.Header())
	}
}

func TestDecoderConsumeInputProgressiveStillRejectedAtStart(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_progressive.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	status, err := dec.ConsumeInput()
	if err != nil {
		t.Fatalf("ConsumeInput progressive failed: %v", err)
	}
	if status != djpeg.InputReachedSOS {
		t.Fatalf("ConsumeInput progressive status = %v, want reached SOS", status)
	}
	if !dec.IsProgressive() {
		t.Fatal("ConsumeInput should expose progressive header state")
	}
	if err := dec.StartDecompress(); !errors.Is(err, djpeg.ErrUnsupported) {
		t.Fatalf("StartDecompress progressive error = %v, want ErrUnsupported", err)
	}
}

func TestDecoderReadHeaderRequireImage(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	cfg, status, err := dec.ReadHeaderRequireImage(true)
	if err != nil {
		t.Fatalf("ReadHeaderRequireImage failed: %v", err)
	}
	if status != djpeg.HeaderOK || status.String() != "ok" {
		t.Fatalf("ReadHeaderRequireImage status = %v (%s), want ok", status, status.String())
	}
	if cfg.Width != 8 || cfg.Height != 8 || cfg.PixelFormat != djpeg.PixelFormatGray8 {
		t.Fatalf("ReadHeaderRequireImage config = %+v, want 8x8 gray", cfg)
	}
	if dec.Header() != cfg {
		t.Fatalf("Header() = %+v, want %+v", dec.Header(), cfg)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress after ReadHeaderRequireImage failed: %v", err)
	}
	if skipped, err := dec.SkipScanlines(100); err != nil || skipped != cfg.Height {
		t.Fatalf("SkipScanlines after ReadHeaderRequireImage skipped=%d err=%v, want %d nil", skipped, err, cfg.Height)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderReadHeaderRequireImageTablesOnly(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0xd9}
	dec := djpeg.NewDecoder(bytes.NewReader(data))
	cfg, status, err := dec.ReadHeaderRequireImage(false)
	if err != nil {
		t.Fatalf("ReadHeaderRequireImage(false) tables-only failed: %v", err)
	}
	if status != djpeg.HeaderTablesOnly || status.String() != "tables-only" {
		t.Fatalf("ReadHeaderRequireImage(false) status = %v (%s), want tables-only", status, status.String())
	}
	if cfg != (djpeg.Config{}) || dec.Header() != (djpeg.Config{}) {
		t.Fatalf("tables-only config=%+v header=%+v, want empty", cfg, dec.Header())
	}

	dec = djpeg.NewDecoder(bytes.NewReader(tableOnlyDQTJPEG()))
	cfg, status, err = dec.ReadHeaderRequireImage(false)
	if err != nil {
		t.Fatalf("ReadHeaderRequireImage(false) DQT tables-only failed: %v", err)
	}
	if status != djpeg.HeaderTablesOnly || cfg != (djpeg.Config{}) || dec.Header() != (djpeg.Config{}) {
		t.Fatalf("DQT tables-only status=%v cfg=%+v header=%+v, want tables-only empty config/header",
			status, cfg, dec.Header())
	}
	qt, ok := dec.QuantizationTable(0)
	if !ok {
		t.Fatal("QuantizationTable(0) should be preserved after tables-only header")
	}
	if qt.Values[0] != 1 || qt.Values[63] != 64 {
		t.Fatalf("tables-only quant table edge values = %d/%d, want 1/64", qt.Values[0], qt.Values[63])
	}

	dec = djpeg.NewDecoder(bytes.NewReader(data))
	_, status, err = dec.ReadHeaderRequireImage(true)
	if !errors.Is(err, djpeg.ErrInvalidJPEG) || status != djpeg.HeaderSuspended {
		t.Fatalf("ReadHeaderRequireImage(true) tables-only status=%v err=%v, want suspended ErrInvalidJPEG", status, err)
	}
}

func tableOnlyDQTJPEG() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xff, 0xd8})
	buf.Write([]byte{0xff, 0xdb})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(byte(i + 1))
	}
	buf.Write([]byte{0xff, 0xd9})
	return buf.Bytes()
}

func TestDecoderReadHeaderRequireImageProgressive(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/test_progressive.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	cfg, status, err := dec.ReadHeaderRequireImage(true)
	if err != nil {
		t.Fatalf("ReadHeaderRequireImage progressive failed: %v", err)
	}
	if status != djpeg.HeaderOK || !cfg.Progressive || !dec.IsProgressive() {
		t.Fatalf("progressive header status=%v cfg.Progressive=%v dec.IsProgressive=%v, want ok/progressive",
			status, cfg.Progressive, dec.IsProgressive())
	}
	if err := dec.StartDecompress(); !errors.Is(err, djpeg.ErrUnsupported) {
		t.Fatalf("StartDecompress progressive error = %v, want ErrUnsupported", err)
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

func TestDecodeRasterConfigExposesDecompressParameters(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	cfg, err := djpeg.DecodeRasterConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRasterConfig failed: %v", err)
	}
	if cfg.OutputGamma != 1.0 || !cfg.DoBlockSmoothing {
		t.Fatalf("default params gamma=%v block_smoothing=%v, want 1.0/true",
			cfg.OutputGamma, cfg.DoBlockSmoothing)
	}

	cfg, err = djpeg.DecodeRasterConfig(
		bytes.NewReader(data),
		djpeg.WithOutputGamma(2.2),
		djpeg.WithBlockSmoothing(false),
	)
	if err != nil {
		t.Fatalf("DecodeRasterConfig parameter overrides failed: %v", err)
	}
	if cfg.OutputGamma != 2.2 || cfg.DoBlockSmoothing {
		t.Fatalf("overridden params gamma=%v block_smoothing=%v, want 2.2/false",
			cfg.OutputGamma, cfg.DoBlockSmoothing)
	}

	dec := djpeg.NewDecoder(
		bytes.NewReader(data),
		djpeg.WithOutputGamma(1.8),
		djpeg.WithBlockSmoothing(true),
	)
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	if out := dec.OutputConfig(); out.OutputGamma != 1.8 || !out.DoBlockSmoothing {
		t.Fatalf("OutputConfig params gamma=%v block_smoothing=%v, want 1.8/true",
			out.OutputGamma, out.DoBlockSmoothing)
	}
}

func TestDecodeRasterConfigInvalidDecompressParameters(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	if _, err := djpeg.DecodeRasterConfig(bytes.NewReader(data), djpeg.WithOutputGamma(-1)); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeRasterConfig negative gamma error = %v, want ErrInvalidOption", err)
	}
	opts := &djpeg.Options{BlockSmoothing: djpeg.BlockSmoothingMode(99)}
	if _, err := djpeg.DecodeRasterConfigWithOptions(bytes.NewReader(data), opts); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("DecodeRasterConfig invalid block smoothing error = %v, want ErrInvalidOption", err)
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

	dec = djpeg.NewDecoder(bytes.NewReader(nil), djpeg.WithMarkerProcessor(0xd8, func(marker djpeg.Marker) error {
		return nil
	}))
	if _, err := dec.ReadHeader(); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ReadHeader invalid marker processor code error = %v, want ErrInvalidOption", err)
	}

	dec = djpeg.NewDecoder(bytes.NewReader(nil), djpeg.WithMarkerProcessor(djpeg.MarkerAPP1, nil))
	if _, err := dec.ReadHeader(); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("ReadHeader nil marker processor error = %v, want ErrInvalidOption", err)
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

func TestDecoderSkipScanlines(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	full, err := djpeg.DecodeRaster(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRaster reference failed: %v", err)
	}
	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}

	if _, err := dec.SkipScanlines(-1); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("SkipScanlines negative error = %v, want ErrInvalidOption", err)
	}
	skipped, err := dec.SkipScanlines(3)
	if err != nil {
		t.Fatalf("SkipScanlines failed: %v", err)
	}
	if skipped != 3 || dec.OutputScanline() != 3 {
		t.Fatalf("SkipScanlines skipped=%d output_scanline=%d, want 3", skipped, dec.OutputScanline())
	}

	row := make([]byte, dec.OutputConfig().Stride)
	n, err := dec.ReadScanlines([][]byte{row})
	if err != nil {
		t.Fatalf("ReadScanlines after skip failed: %v", err)
	}
	wantRow := full.Pix[3*full.Stride : 4*full.Stride]
	if n != 1 || !bytes.Equal(row, wantRow) {
		t.Fatalf("row after skip read=%d equal=%v, want row 3", n, bytes.Equal(row, wantRow))
	}

	skipped, err = dec.SkipScanlines(100)
	if err != nil {
		t.Fatalf("SkipScanlines past end failed: %v", err)
	}
	if skipped != 4 || dec.OutputScanline() != full.Rect.Dy() {
		t.Fatalf("final skip skipped=%d output_scanline=%d, want 4/%d",
			skipped, dec.OutputScanline(), full.Rect.Dy())
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderCropScanline(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	full, err := djpeg.DecodeRaster(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeRaster reference failed: %v", err)
	}
	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if _, _, err := dec.CropScanline(0, 1); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("CropScanline before Start error = %v, want ErrInvalidOption", err)
	}
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	x, width, err := dec.CropScanline(2, 20)
	if err != nil {
		t.Fatalf("CropScanline failed: %v", err)
	}
	if x != 2 || width != 6 {
		t.Fatalf("CropScanline returned x=%d width=%d, want 2/6 after right clamp", x, width)
	}
	out := dec.OutputConfig()
	if out.Width != 6 || out.Stride != 6 {
		t.Fatalf("cropped output config = width %d stride %d, want 6/6", out.Width, out.Stride)
	}

	row := make([]byte, out.Stride)
	n, err := dec.ReadScanlines([][]byte{row})
	if err != nil {
		t.Fatalf("ReadScanlines after crop failed: %v", err)
	}
	want := full.Pix[2:8]
	if n != 1 || !bytes.Equal(row, want) {
		t.Fatalf("cropped row read=%d equal=%v, want columns 2..7", n, bytes.Equal(row, want))
	}
	if skipped, err := dec.SkipScanlines(100); err != nil || skipped != 7 {
		t.Fatalf("SkipScanlines after crop skipped=%d err=%v, want 7 nil", skipped, err)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderCropScanlineAfterReadRejected(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress failed: %v", err)
	}
	if _, err := dec.SkipScanlines(1); err != nil {
		t.Fatalf("SkipScanlines failed: %v", err)
	}
	if _, _, err := dec.CropScanline(0, 1); !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("CropScanline after scanline movement error = %v, want ErrInvalidOption", err)
	}
	if skipped, err := dec.SkipScanlines(100); err != nil || skipped != 7 {
		t.Fatalf("final SkipScanlines skipped=%d err=%v, want 7 nil", skipped, err)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress failed: %v", err)
	}
}

func TestDecoderCropScanlineWithScale(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	full, err := djpeg.DecodeRaster(bytes.NewReader(data), djpeg.WithScale(1, 2))
	if err != nil {
		t.Fatalf("DecodeRaster WithScale reference failed: %v", err)
	}
	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithScale(1, 2))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader WithScale failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress WithScale failed: %v", err)
	}
	x, width, err := dec.CropScanline(1, 2)
	if err != nil {
		t.Fatalf("CropScanline WithScale failed: %v", err)
	}
	if x != 1 || width != 2 || dec.OutputConfig().Stride != 2 {
		t.Fatalf("scaled crop x=%d width=%d stride=%d, want 1/2/2", x, width, dec.OutputConfig().Stride)
	}
	row := make([]byte, dec.OutputConfig().Stride)
	n, err := dec.ReadScanlines([][]byte{row})
	if err != nil {
		t.Fatalf("ReadScanlines scaled crop failed: %v", err)
	}
	want := full.Pix[1:3]
	if n != 1 || !bytes.Equal(row, want) {
		t.Fatalf("scaled cropped row read=%d equal=%v, want columns 1..2", n, bytes.Equal(row, want))
	}
	if skipped, err := dec.SkipScanlines(100); err != nil || skipped != 3 {
		t.Fatalf("scaled final skip skipped=%d err=%v, want 3 nil", skipped, err)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress WithScale failed: %v", err)
	}
}

func TestDecoderSkipScanlinesWithScale(t *testing.T) {
	data, err := os.ReadFile("tests/testdata/gray_8x8.jpg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}

	dec := djpeg.NewDecoder(bytes.NewReader(data), djpeg.WithScale(1, 2))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader WithScale failed: %v", err)
	}
	if err := dec.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress WithScale failed: %v", err)
	}
	out := dec.OutputConfig()
	if out.Height != 4 {
		t.Fatalf("scaled height = %d, want 4", out.Height)
	}
	skipped, err := dec.SkipScanlines(2)
	if err != nil {
		t.Fatalf("scaled SkipScanlines failed: %v", err)
	}
	if skipped != 2 || dec.OutputScanline() != 2 {
		t.Fatalf("scaled skip skipped=%d output_scanline=%d, want 2", skipped, dec.OutputScanline())
	}
	skipped, err = dec.SkipScanlines(10)
	if err != nil {
		t.Fatalf("scaled SkipScanlines past end failed: %v", err)
	}
	if skipped != 2 || dec.OutputScanline() != out.Height {
		t.Fatalf("scaled final skip skipped=%d output_scanline=%d, want 2/%d",
			skipped, dec.OutputScanline(), out.Height)
	}
	if err := dec.FinishDecompress(); err != nil {
		t.Fatalf("FinishDecompress WithScale failed: %v", err)
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

func insertHeaderMarker(data []byte, markerCode int, payload []byte) []byte {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		panic("test JPEG must start with SOI")
	}
	length := len(payload) + 2
	if length > 0xffff {
		panic("test marker payload too large")
	}
	out := make([]byte, 0, len(data)+len(payload)+4)
	out = append(out, data[:2]...)
	out = append(out, 0xff, byte(markerCode), byte(length>>8), byte(length))
	out = append(out, payload...)
	out = append(out, data[2:]...)
	return out
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
