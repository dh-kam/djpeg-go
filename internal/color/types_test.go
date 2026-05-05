package color

import (
	"testing"
)

func TestFix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input float64
		want  int32
	}{
		{0.0, 0},
		{1.0, 65536},     // 1.0 * 65536 = 65536
		{0.5, 32768},     // 0.5 * 65536 = 32768
		{1.402, 91881},   // standard YCbCr coefficient
		{1.772, 116130},  // standard YCbCr coefficient
	}

	for _, tt := range tests {
		got := fix(tt.input)
		// Allow tolerance of 1 due to rounding
		diff := got - tt.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Errorf("fix(%v) = %d, want approximately %d", tt.input, got, tt.want)
		}
	}
}

func TestClampByte(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input int
		want  byte
	}{
		{-1, 0},
		{-256, 0},
		{0, 0},
		{1, 1},
		{128, 128},
		{254, 254},
		{255, 255},
		{256, 255},
		{512, 255},
	}

	for _, tt := range tests {
		got := clampByte(tt.input)
		if got != tt.want {
			t.Errorf("clampByte(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestRoundUp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n, m int
		want int
	}{
		{0, 8, 0},
		{1, 8, 8},
		{7, 8, 8},
		{8, 8, 8},
		{9, 8, 16},
		{15, 16, 16},
		{16, 16, 16},
		{17, 16, 32},
		{10, 3, 12},
		{9, 3, 9},
	}

	for _, tt := range tests {
		got := roundUp(tt.n, tt.m)
		if got != tt.want {
			t.Errorf("roundUp(%d, %d) = %d, want %d", tt.n, tt.m, got, tt.want)
		}
	}
}

func TestCopyRow(t *testing.T) {
	t.Parallel()

	src := []byte{1, 2, 3, 4, 5}
	dst := make([]byte, 5)

	copyRow(dst, src)

	for i, v := range src {
		if dst[i] != v {
			t.Errorf("dst[%d] = %d, want %d", i, dst[i], v)
		}
	}
}

func TestCopySampleRows(t *testing.T) {
	t.Parallel()

	src := [][]byte{
		{1, 2, 3, 4},
		{5, 6, 7, 8},
		{9, 10, 11, 12},
	}
	dst := make([][]byte, 4)
	for i := range dst {
		dst[i] = make([]byte, 4)
	}

	// Copy 2 rows starting from src[1] into dst starting at dst[0]
	copySampleRows(src, dst, 1, 0, 2, 4)

	// dst[0] should be {5,6,7,8}
	for i, w := range []byte{5, 6, 7, 8} {
		if dst[0][i] != w {
			t.Errorf("dst[0][%d] = %d, want %d", i, dst[0][i], w)
		}
	}
	// dst[1] should be {9,10,11,12}
	for i, w := range []byte{9, 10, 11, 12} {
		if dst[1][i] != w {
			t.Errorf("dst[1][%d] = %d, want %d", i, dst[1][i], w)
		}
	}
}

func TestRangeLimit(t *testing.T) {
	t.Parallel()

	table := BuildSampleRangeLimit()

	tests := []struct {
		value int
		want  byte
	}{
		{-256, 0},   // negative clamp
		{-1, 0},     // negative clamp
		{0, 0},      // identity
		{128, 128},  // identity
		{255, 255},  // identity
		{256, 255},  // overflow clamp
		{512, 255},  // overflow clamp
	}

	for _, tt := range tests {
		got := RangeLimit(table, tt.value)
		if got != tt.want {
			t.Errorf("rangeLimit(%d) = %d, want %d", tt.value, got, tt.want)
		}
	}
}
