package jpeg

import "testing"

func TestJDivRoundUp(t *testing.T) {
	tests := []struct {
		a, b, want int64
	}{
		{7, 8, 1},
		{8, 8, 1},
		{9, 8, 2},
		{0, 1, 0},
		{1, 1, 1},
		{1, 2, 1},
		{2, 2, 1},
		{3, 2, 2},
		{15, 16, 1},
		{16, 16, 1},
		{17, 16, 2},
		{100, 7, 15},
		{0, 8, 0},
	}
	for _, tt := range tests {
		got := JDivRoundUp(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("JDivRoundUp(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestJRoundUp(t *testing.T) {
	tests := []struct {
		a, b, want int64
	}{
		{0, 8, 0},
		{1, 8, 8},
		{7, 8, 8},
		{8, 8, 8},
		{9, 8, 16},
		{15, 16, 16},
		{16, 16, 16},
		{17, 16, 32},
		{0, 1, 0},
		{5, 1, 5},
		{5, 3, 6},
		{6, 3, 6},
		{7, 3, 9},
	}
	for _, tt := range tests {
		got := JRoundUp(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("JRoundUp(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestJZeroFar(t *testing.T) {
	// Test zeroing a non-empty slice
	buf := make([]byte, 10)
	for i := range buf {
		buf[i] = 0xFF
	}
	JZeroFar(buf)
	for i, v := range buf {
		if v != 0 {
			t.Errorf("JZeroFar: buf[%d] = %d, want 0", i, v)
		}
	}

	// Test zeroing an empty slice (should not panic)
	JZeroFar(nil)
	JZeroFar([]byte{})
}

func TestZeroSamples(t *testing.T) {
	samples := make(JSAMPROW, 8)
	for i := range samples {
		samples[i] = 255
	}
	ZeroSamples(samples)
	for i, v := range samples {
		if v != 0 {
			t.Errorf("ZeroSamples: samples[%d] = %d, want 0", i, v)
		}
	}
}

func TestZeroCoeffs(t *testing.T) {
	coeffs := make([]JCOEF, 64)
	for i := range coeffs {
		coeffs[i] = JCOEF(i * 100)
	}
	ZeroCoeffs(coeffs)
	for i, v := range coeffs {
		if v != 0 {
			t.Errorf("ZeroCoeffs: coeffs[%d] = %d, want 0", i, v)
		}
	}
}

func TestJCopySampleRows(t *testing.T) {
	// Create source and destination arrays
	src := JSAMPARRAY{
		JSAMPROW{1, 2, 3, 4},
		JSAMPROW{5, 6, 7, 8},
	}
	dst := JSAMPARRAY{
		make(JSAMPROW, 4),
		make(JSAMPROW, 4),
	}

	JCopySampleRows(src, dst, 2, 4)

	for row := 0; row < 2; row++ {
		for col := 0; col < 4; col++ {
			if dst[row][col] != src[row][col] {
				t.Errorf("JCopySampleRows: dst[%d][%d] = %d, want %d",
					row, col, dst[row][col], src[row][col])
			}
		}
	}
}

func TestJCopySampleRowsPartial(t *testing.T) {
	src := JSAMPARRAY{
		JSAMPROW{10, 20, 30, 40, 50},
	}
	dst := JSAMPARRAY{
		make(JSAMPROW, 5),
	}

	// Copy only 3 columns
	JCopySampleRows(src, dst, 1, 3)

	if dst[0][0] != 10 || dst[0][1] != 20 || dst[0][2] != 30 {
		t.Errorf("JCopySampleRows partial: got %v, want [10 20 30]", dst[0][:3])
	}
	// Remaining should be zero
	if dst[0][3] != 0 || dst[0][4] != 0 {
		t.Errorf("JCopySampleRows partial: untouched values should be 0, got %d, %d",
			dst[0][3], dst[0][4])
	}
}

func TestJCopyBlockRow(t *testing.T) {
	src := JBLOCKROW{
		{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
			17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
			33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
			49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64},
		{64, 63, 62, 61, 60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49,
			48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33,
			32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17,
			16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
	}
	dst := make(JBLOCKROW, 2)

	JCopyBlockRow(src, dst, 2)

	for i := 0; i < 2; i++ {
		for j := 0; j < DCTSize2; j++ {
			if dst[i][j] != src[i][j] {
				t.Errorf("JCopyBlockRow: dst[%d][%d] = %d, want %d",
					i, j, dst[i][j], src[i][j])
			}
		}
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{-1, 1, -1},
		{0, 0, 0},
		{-5, -3, -5},
	}
	for _, tt := range tests {
		got := MinInt(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("MinInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMaxInt(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 2},
		{2, 1, 2},
		{-1, 1, 1},
		{0, 0, 0},
		{-5, -3, -3},
	}
	for _, tt := range tests {
		got := MaxInt(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("MaxInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestClampInt(t *testing.T) {
	tests := []struct {
		v, lo, hi, want int
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 0, 0},
		{-100, -50, 50, -50},
		{100, -50, 50, 50},
		{0, -10, -5, -5}, // v below hi range
	}
	for _, tt := range tests {
		got := ClampInt(tt.v, tt.lo, tt.hi)
		if got != tt.want {
			t.Errorf("ClampInt(%d, %d, %d) = %d, want %d",
				tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}
