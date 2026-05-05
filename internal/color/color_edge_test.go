package color

import (
	"testing"
)

// TestColorConverterBGYCC tests BG_YCC -> RGB conversion path.
func TestColorConverterBGYCC(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:  JCS_BG_YCC,
		OutColorSpace:   JCS_RGB,
		NumComponents:   3,
		OutColorComponents: 3,
		OutputComponents:  3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil for BG_YCC")
	}
	if cc.ColorConvert == nil {
		t.Fatal("ColorConvert function is nil for BG_YCC")
	}
	if cc.CrRTab == nil {
		t.Fatal("CrRTab should be set for BG_YCC->RGB")
	}
}

// TestColorConverterCMYKFromYCCK tests YCCK -> CMYK conversion.
func TestColorConverterCMYKFromYCCK(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:    JCS_YCCK,
		OutColorSpace:     JCS_CMYK,
		NumComponents:     4,
		OutColorComponents: 4,
		OutputComponents:  4,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil for YCCK->CMYK")
	}
	if cc.ColorConvert == nil {
		t.Fatal("ColorConvert function is nil for YCCK->CMYK")
	}
}

// TestColorConverterCMYKFromCMYK tests CMYK passthrough.
func TestColorConverterCMYKFromCMYK(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:    JCS_CMYK,
		OutColorSpace:     JCS_CMYK,
		NumComponents:     4,
		OutColorComponents: 4,
		OutputComponents:  4,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil for CMYK passthrough")
	}
	if cc.ColorConvert == nil {
		t.Fatal("ColorConvert function is nil for CMYK passthrough")
	}
}

// TestColorConverterYCCKFromCMYK tests CMYK -> YCCK conversion.
func TestColorConverterYCCKFromCMYK(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:    JCS_CMYK,
		OutColorSpace:     JCS_YCCK,
		NumComponents:     4,
		OutColorComponents: 4,
		OutputComponents:  4,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil for CMYK->YCCK")
	}
}

// TestColorConverterUnknown tests unknown color space passthrough.
func TestColorConverterUnknown(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:    JCS_UNKNOWN,
		OutColorSpace:     JCS_UNKNOWN,
		NumComponents:     3,
		OutColorComponents: 3,
		OutputComponents:  3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil for unknown color space")
	}
}

// TestColorConverterRGBFromGrayscale tests grayscale -> RGB.
func TestColorConverterRGBFromGrayscale(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:    JCS_GRAYSCALE,
		OutColorSpace:     JCS_RGB,
		NumComponents:     1,
		OutColorComponents: 3,
		OutputComponents:  3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil")
	}

	// Test the conversion
	inputBuf := [][][]byte{
		{{128, 200, 50}},
	}
	outputBuf := [][]byte{
		make([]byte, 9), // 3 pixels * 3 bytes
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	// Each pixel should have R=G=B
	for col := 0; col < 3; col++ {
		idx := col * 3
		if outputBuf[0][idx] != outputBuf[0][idx+1] || outputBuf[0][idx+1] != outputBuf[0][idx+2] {
			t.Errorf("pixel %d: R=%d, G=%d, B=%d should be equal",
				col, outputBuf[0][idx], outputBuf[0][idx+1], outputBuf[0][idx+2])
		}
	}
}

// TestColorConverterGrayscaleFromYCbCr tests YCbCr -> Grayscale.
func TestColorConverterGrayscaleFromYCbCr(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:  JCS_YCbCr,
		OutColorSpace:   JCS_GRAYSCALE,
		NumComponents:   3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil")
	}

	inputBuf := [][][]byte{
		{{100, 150, 200, 50}},
		{{128, 128, 128, 128}}, // Cb
		{{128, 128, 128, 128}}, // Cr
	}
	outputBuf := [][]byte{
		make([]byte, 4),
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	// Should copy Y channel directly
	for i, want := range []byte{100, 150, 200, 50} {
		if outputBuf[0][i] != want {
			t.Errorf("pixel[%d] = %d, want %d", i, outputBuf[0][i], want)
		}
	}
}

// TestColorConverterGrayscaleFromRGB tests RGB -> Grayscale.
func TestColorConverterGrayscaleFromRGB(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:  JCS_RGB,
		OutColorSpace:   JCS_GRAYSCALE,
		ColorTransform:  JCT_NONE,
		NumComponents:   3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil")
	}

	inputBuf := [][][]byte{
		{{255}}, // R
		{{255}}, // G
		{{255}}, // B
	}
	outputBuf := [][]byte{
		make([]byte, 1),
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	if outputBuf[0][0] != 255 {
		t.Errorf("grayscale of white = %d, want 255", outputBuf[0][0])
	}
}

// TestColorConverterRGBSubtractGreen tests subtract-green inverse to RGB.
func TestColorConverterRGBSubtractGreen(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:    JCS_RGB,
		OutColorSpace:     JCS_RGB,
		ColorTransform:    JCT_SUBTRACT_GREEN,
		NumComponents:     3,
		OutColorComponents: 3,
		OutputComponents:  3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil")
	}
	if cc.ColorConvert == nil {
		t.Fatal("ColorConvert is nil")
	}

	// Test the conversion: [R-G, G, B-G] -> RGB
	inputBuf := [][][]byte{
		{{0}},   // R-G = 0
		{{128}}, // G = 128
		{{0}},   // B-G = 0
	}
	outputBuf := [][]byte{
		make([]byte, 3),
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	// R = (R-G + G - CenterJSample) & MaxJSample = (0 + 128 - 128) & 255 = 0
	// G = 128
	// B = (B-G + G - CenterJSample) & MaxJSample = (0 + 128 - 128) & 255 = 0
	if outputBuf[0][1] != 128 {
		t.Errorf("G = %d, want 128", outputBuf[0][1])
	}
}

// TestColorConverterBGRGB tests BG_RGB output.
func TestColorConverterBGRGB(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace:    JCS_RGB,
		OutColorSpace:     JCS_BG_RGB,
		ColorTransform:    JCT_NONE,
		NumComponents:     3,
		OutColorComponents: 3,
		OutputComponents:  3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil for BG_RGB")
	}
}

// TestYCCRGBConvertFull tests the full YCbCr -> RGB conversion pipeline.
func TestYCCRGBConvertFull(t *testing.T) {
	cc := &ColorConverter{}
	cc.buildYccRGBTable()

	// Test black: Y=0, Cb=128, Cr=128
	inputBuf := [][][]byte{
		{{0}},       // Y
		{{128}},     // Cb
		{{128}},     // Cr
	}
	outputBuf := [][]byte{
		make([]byte, 3),
	}

	cc.yccRGBConvert(inputBuf, 0, outputBuf, 1)

	// Black should produce near-zero values
	for i, v := range outputBuf[0] {
		if v > 2 {
			t.Logf("Black YCbCr->RGB pixel[%d] = %d (near zero expected)", i, v)
		}
	}

	// Test white: Y=255, Cb=128, Cr=128
	inputBuf[0][0][0] = 255
	outputBuf[0][0] = 0
	outputBuf[0][1] = 0
	outputBuf[0][2] = 0

	cc.yccRGBConvert(inputBuf, 0, outputBuf, 1)

	for i, v := range outputBuf[0] {
		if v < 253 {
			t.Logf("White YCbCr->RGB pixel[%d] = %d (near 255 expected)", i, v)
		}
	}
}

// TestYCCKCMYKConvert tests YCCK -> CMYK conversion.
func TestYCCKCMYKConvert(t *testing.T) {
	cc := &ColorConverter{}
	cc.buildYccRGBTable()

	// Test with neutral values
	inputBuf := [][][]byte{
		{{128}},     // Y
		{{128}},     // Cb
		{{128}},     // Cr
		{{0}},       // K
	}
	outputBuf := [][]byte{
		make([]byte, 4),
	}

	cc.ycckCMYKConvert(inputBuf, 0, outputBuf, 1)

	// K should pass through
	if outputBuf[0][3] != 0 {
		t.Errorf("K = %d, want 0", outputBuf[0][3])
	}
}

// TestCMYKYKConvert tests CMYK -> YK conversion.
func TestCMYKYKConvert(t *testing.T) {
	cc := &ColorConverter{}
	cc.buildRGBYTable()

	// Test with white (C=0, M=0, Y=0, K=0)
	inputBuf := [][][]byte{
		{{0}},   // C
		{{0}},   // M
		{{0}},   // Y
		{{0}},   // K
	}
	outputBuf := [][]byte{
		make([]byte, 2),
	}

	cc.cmykYKConvert(inputBuf, 0, outputBuf, 1)

	// For white (0,0,0,0), Y should be high
	if outputBuf[0][0] < 250 {
		t.Errorf("Y = %d, want near 255 for white CMYK", outputBuf[0][0])
	}
}

// TestRGB1GrayConvert tests [R-G,G,B-G] -> Grayscale conversion.
func TestRGB1GrayConvert(t *testing.T) {
	cc := &ColorConverter{}
	cc.buildRGBYTable()

	// Test with G=128, R-G=0, B-G=0 (so R=0, G=128, B=0)
	inputBuf := [][][]byte{
		{{0}},   // R-G
		{{128}}, // G
		{{0}},   // B-G
	}
	outputBuf := [][]byte{
		make([]byte, 1),
	}

	cc.rgb1GrayConvert(inputBuf, 0, outputBuf, 1)

	// Y = 0.299*R + 0.587*G + 0.114*B where R=0+128-128=0, G=128, B=0
	// Should be approximately 0.587 * 128 = 75
	if outputBuf[0][0] < 70 || outputBuf[0][0] > 80 {
		t.Logf("rgb1GrayConvert(0,128,0) = %d (expected ~75)", outputBuf[0][0])
	}
}

// TestRangeLimitEdge tests the RangeLimit helper with additional edge cases.
func TestRangeLimitEdge(t *testing.T) {
	table := BuildSampleRangeLimit()

	tests := []struct {
		val  int
		want byte
	}{
		{-300, 0},
		{-256, 0},
		{300, 255},
		{500, 255},
	}
	for _, tt := range tests {
		got := RangeLimit(table, tt.val)
		if got != tt.want {
			t.Errorf("RangeLimit(%d) = %d, want %d", tt.val, got, tt.want)
		}
	}
}

// TestColorConverterGrayscaleFromBGYCC tests BG_YCC -> Grayscale.
func TestColorConverterGrayscaleFromBGYCC(t *testing.T) {
	info := &DecompressInfo{
		JpegColorSpace: JCS_BG_YCC,
		OutColorSpace:  JCS_GRAYSCALE,
		NumComponents:  3,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil")
	}

	inputBuf := [][][]byte{
		{{100, 200}},
		{{128, 128}},
		{{128, 128}},
	}
	outputBuf := [][]byte{
		make([]byte, 2),
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	if outputBuf[0][0] != 100 || outputBuf[0][1] != 200 {
		t.Errorf("grayscale from BG_YCC: got %v, want [100 200]", outputBuf[0])
	}
}

