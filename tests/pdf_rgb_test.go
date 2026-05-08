package djpeggo_test

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/djpegcli"
	"github.com/dh-kam/djpeg-go/internal/output"
)

func TestPDFRGBFixtureMatchesPopplerWithTurboFancy(t *testing.T) {
	root := testRepoRoot(t)
	fixtureDir := filepath.Join(root, "tests", "testdata", "pdf-reader-geotopo-p76-rgb-mismatch")
	jpegPath := filepath.Join(fixtureDir, "input-geotopo-p76-rgb.jpg")
	refPath := filepath.Join(fixtureDir, "reference-poppler-pdfimages.png")

	jpegData, err := os.ReadFile(jpegPath)
	if err != nil {
		t.Fatalf("reading fixture JPEG: %v", err)
	}

	var out bytes.Buffer
	opts := &djpegcli.Options{
		Format:     output.FormatPPM,
		DctMethod:  "int",
		TurboFancy: true,
	}
	if err := djpegcli.Decompress(bytes.NewReader(jpegData), &out, opts); err != nil {
		t.Fatalf("Poppler-compatible decode failed: %v", err)
	}

	_, width, height, components, pixels, err := parsePNMPixels(out.Bytes())
	if err != nil {
		t.Fatalf("parsing PPM output: %v", err)
	}
	if components != 3 {
		t.Fatalf("components = %d, want RGB", components)
	}

	refPixels, refWidth, refHeight, err := loadPNGRGB(refPath)
	if err != nil {
		t.Fatalf("loading Poppler reference: %v", err)
	}
	if width != refWidth || height != refHeight {
		t.Fatalf("dimensions = %dx%d, want %dx%d", width, height, refWidth, refHeight)
	}
	if !bytes.Equal(pixels, refPixels) {
		t.Fatalf("Poppler-compatible output differs from Poppler reference")
	}
}

func parsePNMPixels(data []byte) (kind string, width, height, components int, pixels []byte, err error) {
	pos := 0
	nextToken := func() (string, error) {
		for {
			for pos < len(data) && isPNMSpace(data[pos]) {
				pos++
			}
			if pos < len(data) && data[pos] == '#' {
				for pos < len(data) && data[pos] != '\n' {
					pos++
				}
				continue
			}
			break
		}
		if pos >= len(data) {
			return "", fmt.Errorf("unexpected end of PNM header")
		}
		start := pos
		for pos < len(data) && !isPNMSpace(data[pos]) && data[pos] != '#' {
			pos++
		}
		return string(data[start:pos]), nil
	}

	kind, err = nextToken()
	if err != nil {
		return "", 0, 0, 0, nil, err
	}
	if kind != "P5" && kind != "P6" {
		return "", 0, 0, 0, nil, fmt.Errorf("PNM kind = %q, want P5 or P6", kind)
	}
	wToken, err := nextToken()
	if err != nil {
		return "", 0, 0, 0, nil, err
	}
	hToken, err := nextToken()
	if err != nil {
		return "", 0, 0, 0, nil, err
	}
	maxToken, err := nextToken()
	if err != nil {
		return "", 0, 0, 0, nil, err
	}
	width, err = strconv.Atoi(wToken)
	if err != nil {
		return "", 0, 0, 0, nil, fmt.Errorf("invalid PNM width %q", wToken)
	}
	height, err = strconv.Atoi(hToken)
	if err != nil {
		return "", 0, 0, 0, nil, fmt.Errorf("invalid PNM height %q", hToken)
	}
	maxVal, err := strconv.Atoi(maxToken)
	if err != nil {
		return "", 0, 0, 0, nil, fmt.Errorf("invalid PNM max value %q", maxToken)
	}
	if width <= 0 || height <= 0 || maxVal != 255 {
		return "", 0, 0, 0, nil, fmt.Errorf("invalid PNM metadata width=%d height=%d max=%d", width, height, maxVal)
	}
	if pos >= len(data) || !isPNMSpace(data[pos]) {
		return "", 0, 0, 0, nil, fmt.Errorf("PNM header missing payload separator")
	}
	pos++

	components = 3
	if kind == "P5" {
		components = 1
	}
	wantLen := width * height * components
	if len(data)-pos != wantLen {
		return "", 0, 0, 0, nil, fmt.Errorf("PNM payload length = %d, want %d", len(data)-pos, wantLen)
	}
	return kind, width, height, components, data[pos:], nil
}

func loadPNGRGB(path string) ([]byte, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := make([]byte, width*height*3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			idx := (y*width + x) * 3
			pixels[idx] = byte(r >> 8)
			pixels[idx+1] = byte(g >> 8)
			pixels[idx+2] = byte(b >> 8)
		}
	}
	return pixels, width, height, nil
}
