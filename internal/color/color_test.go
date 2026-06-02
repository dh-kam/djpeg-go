package color

import (
	"bytes"
	"testing"
)

func TestBuildSampleRangeLimit(t *testing.T) {
	t.Parallel()
	table := BuildSampleRangeLimit()

	// The table must be large enough for index range [-256..511] offset by 256.
	// Table size = 2*(MaxJSample+1) + CenterJSample = 640
	if len(table) != 640 {
		t.Fatalf("table size = %d, want 640", len(table))
	}

	// Negative index handling: entries [0..MaxJSample] (i.e. indices used for
	// negative values) should map to 0.
	// Index = v + MaxJSample + 1; for v = -256 => idx = 0
	// for v = -1 => idx = 255
	for i := 0; i <= MaxJSample; i++ {
		if table[i] != 0 {
			t.Errorf("table[%d] (negative clamp region) = %d, want 0", i, table[i])
		}
	}

	// Middle segment: values [0..MaxJSample] map to themselves.
	// These are at indices [MaxJSample+1 .. 2*MaxJSample+1]
	for i := 0; i <= MaxJSample; i++ {
		idx := MaxJSample + 1 + i
		if table[idx] != byte(i) {
			t.Errorf("table[%d] (value %d) = %d, want %d", idx, i, table[idx], i)
		}
	}

	// Overflow clamping: values > MaxJSample map to MaxJSample.
	// These are at indices [2*MaxJSample+2 .. 2*MaxJSample+1+CenterJSample]
	for i := MaxJSample + 1; i <= CenterJSample+MaxJSample; i++ {
		idx := MaxJSample + 1 + i
		if idx < len(table) && table[idx] != MaxJSample {
			t.Errorf("table[%d] (overflow clamp) = %d, want %d", idx, table[idx], MaxJSample)
		}
	}
}

func TestColorConverterYCbCrRGB(t *testing.T) {
	t.Parallel()

	// Build the YCbCr->RGB conversion tables
	cc := &ColorConverter{}
	cc.buildYccRGBTable()

	rl := BuildSampleRangeLimit()

	tests := []struct {
		name       string
		y, cb, cr  int
		wantR      byte
		wantG      byte
		wantB      byte
		tolerance  int // tolerance for rounding
	}{
		{"Black", 0, 128, 128, 0, 0, 0, 1},
		{"White", 255, 128, 128, 255, 255, 255, 1},
		{"MidGray", 128, 128, 128, 128, 128, 128, 1},
		{"PureRed", 82, 90, 240, 255, 0, 0, 20},
		{"PureGreen", 145, 54, 34, 0, 255, 0, 20},
		{"PureBlue", 41, 240, 110, 0, 0, 255, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Compute R, G, B the same way yccRGBConvert does
			y := tt.y
			cb := tt.cb
			cr := tt.cr

			r := y + int(cc.CrRTab[cr])
			g := y + int((cc.CbGTab[cb]+cc.CrGTab[cr])>>ScaleBits)
			b := y + int(cc.CbBTab[cb])

			outR := rl[r+MaxJSample+1]
			outG := rl[g+MaxJSample+1]
			outB := rl[b+MaxJSample+1]

			if diff := abs(int(outR) - int(tt.wantR)); diff > tt.tolerance {
				t.Errorf("R = %d, want %d (diff %d > tolerance %d)", outR, tt.wantR, diff, tt.tolerance)
			}
			if diff := abs(int(outG) - int(tt.wantG)); diff > tt.tolerance {
				t.Errorf("G = %d, want %d (diff %d > tolerance %d)", outG, tt.wantG, diff, tt.tolerance)
			}
			if diff := abs(int(outB) - int(tt.wantB)); diff > tt.tolerance {
				t.Errorf("B = %d, want %d (diff %d > tolerance %d)", outB, tt.wantB, diff, tt.tolerance)
			}
		})
	}
}

func TestColorConverterUsesRangeLimitOffset(t *testing.T) {
	t.Parallel()

	cc := &ColorConverter{}
	cc.buildYccRGBTable()

	rgbInput := [][][]byte{
		{[]byte{255, 128, 0}},
		{[]byte{128, 128, 128}},
		{[]byte{128, 128, 128}},
	}
	rgbOut := [][]byte{make([]byte, 9)}
	cc.yccRGBConvert(rgbInput, 0, rgbOut, 1)
	if got, want := rgbOut[0], []byte{255, 255, 255, 128, 128, 128, 0, 0, 0}; !bytes.Equal(got, want) {
		t.Fatalf("YCbCr neutral RGB = %v, want %v", got, want)
	}

	cmykInput := [][][]byte{
		{[]byte{255, 128, 0}},
		{[]byte{128, 128, 128}},
		{[]byte{128, 128, 128}},
		{[]byte{7, 8, 9}},
	}
	cmykOut := [][]byte{make([]byte, 12)}
	cc.ycckCMYKConvert(cmykInput, 0, cmykOut, 1)
	if got, want := cmykOut[0], []byte{0, 0, 0, 7, 127, 127, 127, 8, 255, 255, 255, 9}; !bytes.Equal(got, want) {
		t.Fatalf("YCCK neutral CMYK = %v, want %v", got, want)
	}
}

func TestGrayscaleConvert(t *testing.T) {
	t.Parallel()

	cc := &ColorConverter{}
	cc.ColorConvert = cc.grayscaleConvert

	// Create input: one component, one row of 4 pixels
	inputBuf := [][][]byte{
		{{10, 20, 30, 40}}, // Y component
	}
	outputBuf := [][]byte{
		make([]byte, 4),
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	for i, want := range []byte{10, 20, 30, 40} {
		if outputBuf[0][i] != want {
			t.Errorf("pixel[%d] = %d, want %d", i, outputBuf[0][i], want)
		}
	}
}

func TestNullConvert(t *testing.T) {
	t.Parallel()

	cc := &ColorConverter{}
	cc.ColorConvert = cc.nullConvert

	// 3 components, 1 row of 2 pixels, interleaved output
	inputBuf := [][][]byte{
		{{100, 200}}, // R
		{{50, 60}},   // G
		{{25, 35}},   // B
	}
	outputBuf := [][]byte{
		make([]byte, 6), // 2 pixels * 3 components
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	// nullConvert interleaves: for each column, puts comp0 then comp1 then comp2
	// So output should be: R0, G0, B0, R1, G1, B1 = 100, 50, 25, 200, 60, 35
	want := []byte{100, 50, 25, 200, 60, 35}
	for i, w := range want {
		if outputBuf[0][i] != w {
			t.Errorf("output[%d] = %d, want %d", i, outputBuf[0][i], w)
		}
	}
}

func TestNewColorConverterGrayscale(t *testing.T) {
	t.Parallel()

	info := &DecompressInfo{
		JpegColorSpace:  JCS_GRAYSCALE,
		OutColorSpace:   JCS_GRAYSCALE,
		NumComponents:   1,
		CompInfo:        []ComponentInfo{{ComponentNeeded: true}},
	}

	cc := NewColorConverter(info)
	if cc == nil {
		t.Fatal("NewColorConverter returned nil")
	}
	if cc.ColorConvert == nil {
		t.Fatal("ColorConvert function is nil")
	}
}

func TestNewColorConverterRGB(t *testing.T) {
	t.Parallel()

	info := &DecompressInfo{
		JpegColorSpace:    JCS_YCbCr,
		OutColorSpace:     JCS_RGB,
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
		t.Fatal("ColorConvert function is nil")
	}
	if cc.CrRTab == nil || cc.CbBTab == nil {
		t.Fatal("YCbCr->RGB lookup tables not built")
	}
}

func TestGrayRGBConvert(t *testing.T) {
	t.Parallel()

	cc := &ColorConverter{}
	cc.ColorConvert = cc.grayRGBConvert

	// Input: one grayscale component with 3 pixels
	inputBuf := [][][]byte{
		{{50, 128, 200}},
	}
	outputBuf := [][]byte{
		make([]byte, 9), // 3 pixels * 3 bytes
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	// Each pixel should be duplicated to R=G=B
	tests := []struct {
		offset int
		value  byte
	}{
		{0, 50}, {1, 50}, {2, 50},
		{3, 128}, {4, 128}, {5, 128},
		{6, 200}, {7, 200}, {8, 200},
	}
	for _, tt := range tests {
		if outputBuf[0][tt.offset] != tt.value {
			t.Errorf("output[%d] = %d, want %d", tt.offset, outputBuf[0][tt.offset], tt.value)
		}
	}
}

func TestRGBConvert(t *testing.T) {
	t.Parallel()

	cc := &ColorConverter{}
	cc.ColorConvert = cc.rgbConvert

	inputBuf := [][][]byte{
		{{255, 0}},   // R
		{{0, 255}},   // G
		{{0, 0}},     // B
	}
	outputBuf := [][]byte{
		make([]byte, 6), // 2 pixels * 3 bytes
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	// Pixel 0: R=255, G=0, B=0
	// Pixel 1: R=0, G=255, B=0
	want := []byte{255, 0, 0, 0, 255, 0}
	for i, w := range want {
		if outputBuf[0][i] != w {
			t.Errorf("output[%d] = %d, want %d", i, outputBuf[0][i], w)
		}
	}
}

func TestRGBGrayConvert(t *testing.T) {
	t.Parallel()

	cc := &ColorConverter{}
	cc.buildRGBYTable()
	cc.ColorConvert = cc.rgbGrayConvert

	// For pure white (255,255,255), Y = 0.299*255 + 0.587*255 + 0.114*255 = 255
	inputBuf := [][][]byte{
		{{255}},
		{{255}},
		{{255}},
	}
	outputBuf := [][]byte{
		make([]byte, 1),
	}

	cc.ColorConvert(inputBuf, 0, outputBuf, 1)

	if outputBuf[0][0] != 255 {
		t.Errorf("grayscale of white = %d, want 255", outputBuf[0][0])
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
