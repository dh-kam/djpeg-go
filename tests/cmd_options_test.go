package djpeggo_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/gif"
	"os"
	"testing"

	djpeg "github.com/dh-kam/djpeg-go"
	"github.com/dh-kam/djpeg-go/internal/djpegcli"
	"github.com/dh-kam/djpeg-go/internal/output"
)

func TestFastOptionMatchesExplicitFastNoSmooth(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	fastOutput := decompressForOptionTest(t, data, &djpegcli.Options{
		Format: output.FormatPPM,
		Fast:   true,
	})
	explicitOutput := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:    output.FormatPPM,
		DctMethod: "fast",
		NoSmooth:  true,
	})

	if !bytes.Equal(fastOutput, explicitOutput) {
		t.Fatal("--fast output differs from --dct fast --nosmooth output")
	}
}

func TestUpstreamCLIAliasesAreAccepted(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	cmd := djpegcli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetIn(bytes.NewReader(data))
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--bmp", "--pnm"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("executing --pnm alias: %v", err)
	}
	kind, _, _, components, _, err := parsePNMPixels(out.Bytes())
	if err != nil {
		t.Fatalf("parsing --pnm output: %v", err)
	}
	if kind != "P6" || components != 3 {
		t.Fatalf("--pnm output kind/components = %s/%d, want P6/3", kind, components)
	}

	cmd = djpegcli.NewRootCommand()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--debug", "--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("executing --debug alias with --version: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("djpeg-go")) {
		t.Fatalf("--version output = %q, want djpeg-go version", out.String())
	}
}

func TestGrayscaleOptionWritesPGMForColorInput(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	ppm := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:    output.FormatPPM,
		Grayscale: true,
	})

	kind, width, height, components, pixels, err := parsePNMPixels(ppm)
	if err != nil {
		t.Fatalf("parsing PGM output: %v", err)
	}
	if kind != "P5" {
		t.Fatalf("PNM kind = %q, want P5", kind)
	}
	if width <= 0 || height <= 0 {
		t.Fatalf("dimensions = %dx%d, want positive", width, height)
	}
	if components != 1 {
		t.Fatalf("components = %d, want 1", components)
	}
	if len(pixels) != width*height {
		t.Fatalf("pixel length = %d, want %d", len(pixels), width*height)
	}
}

func TestRGBOptionWritesPPMForGrayscaleInput(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/gray_8x8.jpg")
	if err != nil {
		t.Skip("gray_8x8.jpg not available:", err)
	}

	grayPNM := decompressForOptionTest(t, data, &djpegcli.Options{
		Format: output.FormatPPM,
	})
	rgbPNM := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:   output.FormatPPM,
		ForceRGB: true,
	})

	_, width, height, grayComponents, grayPixels, err := parsePNMPixels(grayPNM)
	if err != nil {
		t.Fatalf("parsing default PGM output: %v", err)
	}
	kind, rgbWidth, rgbHeight, rgbComponents, rgbPixels, err := parsePNMPixels(rgbPNM)
	if err != nil {
		t.Fatalf("parsing forced RGB PPM output: %v", err)
	}
	if kind != "P6" {
		t.Fatalf("PNM kind = %q, want P6", kind)
	}
	if grayComponents != 1 || rgbComponents != 3 {
		t.Fatalf("components gray=%d rgb=%d, want 1 and 3", grayComponents, rgbComponents)
	}
	if rgbWidth != width || rgbHeight != height {
		t.Fatalf("RGB dimensions = %dx%d, want %dx%d", rgbWidth, rgbHeight, width, height)
	}
	for i, gray := range grayPixels {
		j := i * 3
		if rgbPixels[j] != gray || rgbPixels[j+1] != gray || rgbPixels[j+2] != gray {
			t.Fatalf("RGB pixel %d = [%d %d %d], want [%d %d %d]",
				i, rgbPixels[j], rgbPixels[j+1], rgbPixels[j+2], gray, gray, gray)
		}
	}
}

func TestOutputColorSpaceOptionWritesPGMForColorInput(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	ppm := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:           output.FormatPPM,
		OutputColorSpace: "gray",
	})

	kind, width, height, components, pixels, err := parsePNMPixels(ppm)
	if err != nil {
		t.Fatalf("parsing PGM output: %v", err)
	}
	if kind != "P5" {
		t.Fatalf("PNM kind = %q, want P5", kind)
	}
	if components != 1 {
		t.Fatalf("components = %d, want 1", components)
	}
	if len(pixels) != width*height {
		t.Fatalf("pixel length = %d, want %d", len(pixels), width*height)
	}
}

func TestOutputColorSpaceOptionWritesPPMForGrayscaleInput(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/gray_8x8.jpg")
	if err != nil {
		t.Skip("gray_8x8.jpg not available:", err)
	}

	ppm := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:           output.FormatPPM,
		OutputColorSpace: "rgb",
	})

	kind, width, height, components, pixels, err := parsePNMPixels(ppm)
	if err != nil {
		t.Fatalf("parsing forced RGB PPM output: %v", err)
	}
	if kind != "P6" {
		t.Fatalf("PNM kind = %q, want P6", kind)
	}
	if width != 8 || height != 8 {
		t.Fatalf("dimensions = %dx%d, want 8x8", width, height)
	}
	if components != 3 {
		t.Fatalf("components = %d, want 3", components)
	}
	if len(pixels) != width*height*components {
		t.Fatalf("pixel length = %d, want %d", len(pixels), width*height*components)
	}
}

func TestMaxMemoryOptionAllowsDecodeWithSufficientLimit(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	out := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:    output.FormatPPM,
		MaxMemory: "1m",
	})
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func TestMaxMemoryOptionRejectsSmallLimit(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	var out bytes.Buffer
	err = djpegcli.Decompress(bytes.NewReader(data), &out, &djpegcli.Options{
		Format:    output.FormatPPM,
		MaxMemory: "1",
	})
	if err == nil {
		t.Fatal("expected memory limit error")
	}
	if !errors.Is(err, djpeg.ErrMemoryLimit) {
		t.Fatalf("error = %v, want ErrMemoryLimit", err)
	}
}

func TestMaxMemoryOptionRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := djpegcli.Decompress(bytes.NewReader(nil), &out, &djpegcli.Options{
		Format:    output.FormatPPM,
		MaxMemory: "bad",
	})
	if err == nil {
		t.Fatal("expected invalid option error")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func TestColorsOptionQuantizesPPM(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	ppm := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:    output.FormatPPM,
		NumColors: 8,
	})
	_, _, _, components, pixels, err := parsePNMPixels(ppm)
	if err != nil {
		t.Fatalf("parsing quantized PPM output: %v", err)
	}
	if components != 3 {
		t.Fatalf("components = %d, want RGB", components)
	}
	if got := countUniqueRGB(pixels); got > 8 {
		t.Fatalf("unique colors = %d, want <= 8", got)
	}
}

func TestColorsOptionRejectsTooFewRGBColors(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	var out bytes.Buffer
	err = djpegcli.Decompress(bytes.NewReader(data), &out, &djpegcli.Options{
		Format:    output.FormatPPM,
		NumColors: 7,
	})
	if err == nil {
		t.Fatal("expected invalid option error")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func TestDitherAndOnePassOptionsAllowQuantizedDecode(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	tests := []struct {
		name string
		opts djpegcli.Options
	}{
		{
			name: "onepass",
			opts: djpegcli.Options{Format: output.FormatPPM, NumColors: 8, OnePass: true},
		},
		{
			name: "dither_none",
			opts: djpegcli.Options{Format: output.FormatPPM, NumColors: 8, DitherMode: "none"},
		},
		{
			name: "dither_ordered",
			opts: djpegcli.Options{Format: output.FormatPPM, NumColors: 8, DitherMode: "ordered"},
		},
		{
			name: "dither_fs",
			opts: djpegcli.Options{Format: output.FormatPPM, NumColors: 8, DitherMode: "fs"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ppm := decompressForOptionTest(t, data, &tt.opts)
			_, _, _, components, pixels, err := parsePNMPixels(ppm)
			if err != nil {
				t.Fatalf("parsing quantized PPM output: %v", err)
			}
			if components != 3 {
				t.Fatalf("components = %d, want RGB", components)
			}
			if got := countUniqueRGB(pixels); got > 8 {
				t.Fatalf("unique colors = %d, want <= 8", got)
			}
		})
	}
}

func TestOnePassOptionUsesGeneratedPalette(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	twoPass := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:     output.FormatPPM,
		NumColors:  8,
		DitherMode: "none",
	})
	onePass := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:     output.FormatPPM,
		NumColors:  8,
		DitherMode: "none",
		OnePass:    true,
	})

	if bytes.Equal(twoPass, onePass) {
		t.Fatal("--onepass output matched default two-pass quantized output")
	}
}

func TestGIFOutputAutoQuantizesRGB(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	gifData := decompressForOptionTest(t, data, &djpegcli.Options{
		Format: output.FormatGIF,
	})
	if len(gifData) < 6 || string(gifData[:6]) != "GIF87a" {
		t.Fatalf("GIF header = %q, want GIF87a", gifData[:min(len(gifData), 6)])
	}
}

func TestGIF0OutputAutoQuantizesRGB(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	gifData := decompressForOptionTest(t, data, &djpegcli.Options{
		Format: output.FormatGIF0,
	})
	cfg, err := gif.DecodeConfig(bytes.NewReader(gifData))
	if err != nil {
		t.Fatalf("decoding GIF0 output: %v", err)
	}
	if cfg.Width != 256 || cfg.Height != 256 {
		t.Fatalf("GIF0 dimensions = %dx%d, want 256x256", cfg.Width, cfg.Height)
	}
	if len(gifData) < 6 || string(gifData[:6]) != "GIF87a" {
		t.Fatalf("GIF0 header = %q, want GIF87a", gifData[:min(len(gifData), 6)])
	}
}

func TestOS2BMPOutputWritesCoreHeader(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	bmp := decompressForOptionTest(t, data, &djpegcli.Options{
		Format: output.FormatBMPOS2,
	})
	if len(bmp) < 26 {
		t.Fatalf("OS/2 BMP output length = %d, want at least 26", len(bmp))
	}
	if string(bmp[:2]) != "BM" {
		t.Fatalf("OS/2 BMP signature = %q, want BM", bmp[:2])
	}
	if offset := binary.LittleEndian.Uint32(bmp[10:14]); offset != 26 {
		t.Fatalf("OS/2 BMP pixel offset = %d, want 26", offset)
	}
	if headerSize := binary.LittleEndian.Uint32(bmp[14:18]); headerSize != 12 {
		t.Fatalf("OS/2 BMP core header size = %d, want 12", headerSize)
	}
	if width := binary.LittleEndian.Uint16(bmp[18:20]); width != 256 {
		t.Fatalf("OS/2 BMP width = %d, want 256", width)
	}
	if bitCount := binary.LittleEndian.Uint16(bmp[24:26]); bitCount != 24 {
		t.Fatalf("OS/2 BMP bit count = %d, want 24", bitCount)
	}
}

func TestMapOptionUsesExternalPalette(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	palettePath := t.TempDir() + "/palette.ppm"
	palette := []byte("P6\n2 1\n255\n\x00\x00\x00\xff\xff\xff")
	if err := os.WriteFile(palettePath, palette, 0o600); err != nil {
		t.Fatalf("writing palette: %v", err)
	}

	ppm := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:  output.FormatPPM,
		MapFile: palettePath,
	})
	_, _, _, components, pixels, err := parsePNMPixels(ppm)
	if err != nil {
		t.Fatalf("parsing mapped PPM output: %v", err)
	}
	if components != 3 {
		t.Fatalf("components = %d, want RGB", components)
	}
	for i := 0; i < len(pixels); i += 3 {
		black := pixels[i] == 0 && pixels[i+1] == 0 && pixels[i+2] == 0
		white := pixels[i] == 255 && pixels[i+1] == 255 && pixels[i+2] == 255
		if !black && !white {
			t.Fatalf("pixel %d = [%d %d %d], want palette color", i/3, pixels[i], pixels[i+1], pixels[i+2])
		}
	}
}

func TestMapOptionAllowsSingleColorPalette(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	palettePath := t.TempDir() + "/palette.ppm"
	palette := []byte("P6\n1 1\n255\n\x11\x22\x33")
	if err := os.WriteFile(palettePath, palette, 0o600); err != nil {
		t.Fatalf("writing palette: %v", err)
	}

	ppm := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:     output.FormatPPM,
		MapFile:    palettePath,
		DitherMode: "none",
	})
	_, _, _, components, pixels, err := parsePNMPixels(ppm)
	if err != nil {
		t.Fatalf("parsing mapped PPM output: %v", err)
	}
	if components != 3 {
		t.Fatalf("components = %d, want RGB", components)
	}
	for i := 0; i < len(pixels); i += 3 {
		if pixels[i] != 0x11 || pixels[i+1] != 0x22 || pixels[i+2] != 0x33 {
			t.Fatalf("pixel %d = [%d %d %d], want single palette color", i/3, pixels[i], pixels[i+1], pixels[i+2])
		}
	}
}

func TestScaleOptionWritesScaledOutput(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/test_420.jpg")
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}

	ppm := decompressForOptionTest(t, data, &djpegcli.Options{
		Format: output.FormatPPM,
		Scale:  "1/2",
	})
	_, width, height, components, pixels, err := parsePNMPixels(ppm)
	if err != nil {
		t.Fatalf("parsing scaled PPM output: %v", err)
	}
	if width != 128 || height != 128 {
		t.Fatalf("scaled dimensions = %dx%d, want 128x128", width, height)
	}
	if components != 3 {
		t.Fatalf("components = %d, want RGB", components)
	}
	if len(pixels) != width*height*components {
		t.Fatalf("pixel length = %d, want %d", len(pixels), width*height*components)
	}
}

func TestScaleOptionRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := djpegcli.Decompress(bytes.NewReader(nil), &out, &djpegcli.Options{
		Format: output.FormatPPM,
		Scale:  "1/0",
	})
	if err == nil {
		t.Fatal("expected invalid option error")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func TestCompatibilityOptionMatchesTurboFancyAlias(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/pdf-reader-geotopo-p76-rgb-mismatch/input-geotopo-p76-rgb.jpg")
	if err != nil {
		t.Skip("Poppler fixture not available:", err)
	}

	compatOutput := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:        output.FormatPPM,
		Compatibility: "poppler-pdf",
	})
	aliasOutput := decompressForOptionTest(t, data, &djpegcli.Options{
		Format:     output.FormatPPM,
		TurboFancy: true,
	})

	if !bytes.Equal(compatOutput, aliasOutput) {
		t.Fatal("--compatibility poppler-pdf output differs from --turbo-fancy output")
	}
}

func TestCompatibilityOptionRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := djpegcli.Decompress(bytes.NewReader(nil), &out, &djpegcli.Options{
		Format:        output.FormatPPM,
		Compatibility: "bad",
	})
	if err == nil {
		t.Fatal("expected invalid option error")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func TestColorTransformOptionRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := djpegcli.Decompress(bytes.NewReader(nil), &out, &djpegcli.Options{
		Format:         output.FormatPPM,
		ColorTransform: "bad",
	})
	if err == nil {
		t.Fatal("expected invalid option error")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func TestOutputColorSpaceOptionRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := djpegcli.Decompress(bytes.NewReader(nil), &out, &djpegcli.Options{
		Format:           output.FormatPPM,
		OutputColorSpace: "bad",
	})
	if err == nil {
		t.Fatal("expected invalid option error")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func TestOutputColorSpaceOptionRejectsAliasConflict(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := djpegcli.Decompress(bytes.NewReader(nil), &out, &djpegcli.Options{
		Format:           output.FormatPPM,
		Grayscale:        true,
		OutputColorSpace: "rgb",
	})
	if err == nil {
		t.Fatal("expected invalid option error")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func TestConflictingOutputColorOptionsReturnError(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := djpegcli.Decompress(bytes.NewReader(nil), &out, &djpegcli.Options{
		Format:    output.FormatPPM,
		Grayscale: true,
		ForceRGB:  true,
	})
	if err == nil {
		t.Fatal("expected error for conflicting output color options")
	}
	if !errors.Is(err, djpeg.ErrInvalidOption) {
		t.Fatalf("error = %v, want ErrInvalidOption", err)
	}
}

func decompressForOptionTest(t *testing.T, data []byte, opts *djpegcli.Options) []byte {
	t.Helper()

	var out bytes.Buffer
	if err := djpegcli.Decompress(bytes.NewReader(data), &out, opts); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func countUniqueRGB(pixels []byte) int {
	seen := make(map[[3]byte]struct{})
	for i := 0; i+2 < len(pixels); i += 3 {
		seen[[3]byte{pixels[i], pixels[i+1], pixels[i+2]}] = struct{}{}
	}
	return len(seen)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
