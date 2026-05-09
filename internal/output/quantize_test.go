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
		DesiredColors: 8,
		Dither:        DitherNone,
	})
	if err != nil {
		t.Fatalf("QuantizeRows default failed: %v", err)
	}
	if cm.NumColors != 2 {
		t.Fatalf("default palette colors = %d, want the two image colors", cm.NumColors)
	}
	if !colormapHasRGB(cm, 0, 0, 0) || !colormapHasRGB(cm, 255, 0, 0) {
		t.Fatalf("default palette = %#v, want black and red from image", cm.Maps)
	}

	_, onePass, err := QuantizeRows(rows, info, QuantizeOptions{
		DesiredColors: 8,
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

func TestQuantizeRowsRejectsTooFewGeneratedRGBColors(t *testing.T) {
	t.Parallel()

	rows := [][]byte{{0, 0, 0, 255, 255, 255}}
	info := &ImageInfo{
		Width:         2,
		Height:        1,
		NumComponents: 3,
		ColorSpace:    ColorSpaceRGB,
	}
	if _, _, err := QuantizeRows(rows, info, QuantizeOptions{DesiredColors: 7}); err == nil {
		t.Fatal("QuantizeRows with seven RGB colors succeeded, want error")
	}
}

func TestOrderedDitherAdjustMatchesIJGMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		levels int
		x      int
		y      int
		want   int
	}{
		{name: "two_level_high", levels: 2, x: 0, y: 0, want: 127},
		{name: "two_level_low", levels: 2, x: 1, y: 0, want: -64},
		{name: "six_level_high", levels: 6, x: 0, y: 0, want: 25},
		{name: "six_level_repeat", levels: 6, x: 16, y: 16, want: 25},
		{name: "single_level", levels: 1, x: 0, y: 0, want: 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := orderedDitherAdjust(tt.levels, tt.x, tt.y); got != tt.want {
				t.Fatalf("orderedDitherAdjust(%d, %d, %d) = %d, want %d",
					tt.levels, tt.x, tt.y, got, tt.want)
			}
		})
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
