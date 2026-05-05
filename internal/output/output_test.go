package output

import (
	"strings"
	"testing"
)

func TestNewWriter(t *testing.T) {
	t.Parallel()

	formats := []Format{
		FormatPPM,
		FormatBMP,
		FormatGIF,
		FormatTarga,
		FormatRLE,
	}

	for _, f := range formats {
		f := f
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()
			w, err := NewWriter(f)
			if err != nil {
				t.Errorf("NewWriter(%q) returned error: %v", f, err)
			}
			if w == nil {
				t.Errorf("NewWriter(%q) returned nil writer", f)
			}
		})
	}
}

func TestNewWriterInvalid(t *testing.T) {
	t.Parallel()

	w, err := NewWriter("invalid_format")
	if err == nil {
		t.Error("expected error for unknown format, got nil")
	}
	if w != nil {
		t.Error("expected nil writer for unknown format")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error message = %q, want it to contain 'unsupported'", err.Error())
	}
}

func TestImageInfo(t *testing.T) {
	t.Parallel()

	info := &ImageInfo{
		Width:         640,
		Height:        480,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
		XDensity:      72,
		YDensity:      72,
		DensityUnit:   1,
	}

	if info.Width != 640 {
		t.Errorf("Width = %d, want 640", info.Width)
	}
	if info.Height != 480 {
		t.Errorf("Height = %d, want 480", info.Height)
	}
	if info.ColorSpace != ColorSpaceRGB {
		t.Errorf("ColorSpace = %d, want %d", info.ColorSpace, ColorSpaceRGB)
	}
}

func TestColormapStruct(t *testing.T) {
	t.Parallel()

	cm := &Colormap{
		Maps: [][]uint8{
			{255, 0, 0},   // R
			{0, 255, 0},   // G
			{0, 0, 255},   // B
		},
		NumColors: 3,
	}

	if cm.NumColors != 3 {
		t.Errorf("NumColors = %d, want 3", cm.NumColors)
	}
	if len(cm.Maps) != 3 {
		t.Errorf("len(Maps) = %d, want 3", len(cm.Maps))
	}
	if cm.Maps[0][0] != 255 {
		t.Errorf("Maps[0][0] = %d, want 255", cm.Maps[0][0])
	}
}
