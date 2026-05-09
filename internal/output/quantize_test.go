package output

import (
	"bytes"
	"testing"
)

func TestQuantizeRowsDefaultBuildsImagePalette(t *testing.T) {
	t.Parallel()

	rows := [][]byte{
		{0, 0, 0, 255, 0, 0},
		{0, 0, 0, 255, 0, 0},
	}
	info := &ImageInfo{
		Width:         2,
		Height:        2,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}

	_, cm, err := QuantizeRows(rows, info, QuantizeOptions{
		DesiredColors: 2,
		Dither:        DitherNone,
	})
	if err != nil {
		t.Fatalf("QuantizeRows default failed: %v", err)
	}
	if cm.NumColors != 2 {
		t.Fatalf("default palette colors = %d, want 2", cm.NumColors)
	}
	if !colormapHasRGB(cm, 0, 0, 0) || !colormapHasRGB(cm, 255, 0, 0) {
		t.Fatalf("default palette = %#v, want black and red from image", cm.Maps)
	}

	_, onePass, err := QuantizeRows(rows, info, QuantizeOptions{
		DesiredColors: 2,
		Dither:        DitherNone,
		OnePass:       true,
	})
	if err != nil {
		t.Fatalf("QuantizeRows one-pass failed: %v", err)
	}
	if colormapsEqual(cm, onePass) {
		t.Fatal("one-pass palette unexpectedly matched two-pass image palette")
	}
}

func colormapHasRGB(cm *Colormap, r, g, b byte) bool {
	if cm == nil || len(cm.Maps) < 3 {
		return false
	}
	for i := 0; i < cm.NumColors; i++ {
		if cm.Maps[0][i] == r && cm.Maps[1][i] == g && cm.Maps[2][i] == b {
			return true
		}
	}
	return false
}

func colormapsEqual(a, b *Colormap) bool {
	if a == nil || b == nil || a.NumColors != b.NumColors || len(a.Maps) != len(b.Maps) {
		return false
	}
	for i := range a.Maps {
		if !bytes.Equal(a.Maps[i][:a.NumColors], b.Maps[i][:b.NumColors]) {
			return false
		}
	}
	return true
}
