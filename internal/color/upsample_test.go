package color

import (
	"testing"
)

func TestH2V1Upsample(t *testing.T) {
	t.Parallel()

	// Set up a decompression info for 6-pixel-wide output with 2:1 horizontal upsampling
	info := &DecompressInfo{
		OutputWidth:    6, // output width (after upsampling)
		MaxVSampFactor: 1,
		MaxHSampFactor: 2,
		CompInfo: []ComponentInfo{
			{
				ComponentNeeded: true,
				HSampFactor:     1,
				VSampFactor:     1,
				DCTHScalSize:    8,
				DCTVScalSize:    8,
			},
		},
		MinDCTHScalSize: 8,
		MinDCTVScalSize: 8,
	}

	// Input: 3 pixels [A=10, B=20, C=30], output should be 6 pixels [10,10,20,20,30,30]
	inputData := [][]byte{
		{10, 20, 30},
	}
	outputData := make([][]byte, 1)
	outputData[0] = make([]byte, 6)

	h2v1Upsample(info, 0, inputData, outputData)

	want := []byte{10, 10, 20, 20, 30, 30}
	for i, w := range want {
		if outputData[0][i] != w {
			t.Errorf("output[%d] = %d, want %d", i, outputData[0][i], w)
		}
	}
}

func TestH2V2Upsample(t *testing.T) {
	t.Parallel()

	// 2:1 horizontal + 2:1 vertical upsampling
	info := &DecompressInfo{
		OutputWidth:    6,
		MaxVSampFactor: 2,
		MaxHSampFactor: 2,
		CompInfo: []ComponentInfo{
			{
				ComponentNeeded: true,
				HSampFactor:     1,
				VSampFactor:     1,
				DCTHScalSize:    8,
				DCTVScalSize:    8,
			},
		},
		MinDCTHScalSize: 8,
		MinDCTVScalSize: 8,
	}

	// Input: 1 row with 3 pixels [A=10, B=20, C=30]
	// Output: 2 rows, each with 6 pixels [10,10,20,20,30,30]
	inputData := [][]byte{
		{10, 20, 30},
	}
	outputData := make([][]byte, 2)
	outputData[0] = make([]byte, 6)
	outputData[1] = make([]byte, 6)

	h2v2Upsample(info, 0, inputData, outputData)

	want := []byte{10, 10, 20, 20, 30, 30}
	for i, w := range want {
		if outputData[0][i] != w {
			t.Errorf("output row 0[%d] = %d, want %d", i, outputData[0][i], w)
		}
		if outputData[1][i] != w {
			t.Errorf("output row 1[%d] = %d, want %d", i, outputData[1][i], w)
		}
	}
}

func TestIntUpsample(t *testing.T) {
	t.Parallel()

	// Test IntUpsampleWithFactors with 3:1 horizontal and 2:1 vertical expansion
	info := &DecompressInfo{
		OutputWidth:    6, // output 6 pixels from 2 input pixels
		MaxVSampFactor: 2,
		CompInfo: []ComponentInfo{
			{
				ComponentNeeded: true,
				HSampFactor:     1,
				VSampFactor:     1,
				DCTHScalSize:    8,
				DCTVScalSize:    8,
			},
		},
		MinDCTHScalSize: 8,
		MinDCTVScalSize: 8,
	}

	// Input: 2 pixels [A=50, B=100]
	// With hExpand=3: each pixel becomes 3 pixels: [50,50,50,100,100,100]
	inputData := [][]byte{
		{50, 100},
	}
	outputData := make([][]byte, 2)
	outputData[0] = make([]byte, 6)
	outputData[1] = make([]byte, 6)

	IntUpsampleWithFactors(info, 0, 3, 2, inputData, outputData)

	wantRow := []byte{50, 50, 50, 100, 100, 100}
	for i, w := range wantRow {
		if outputData[0][i] != w {
			t.Errorf("output row 0[%d] = %d, want %d", i, outputData[0][i], w)
		}
		if outputData[1][i] != w {
			t.Errorf("output row 1[%d] = %d, want %d", i, outputData[1][i], w)
		}
	}
}

func TestNewUpsampler(t *testing.T) {
	t.Parallel()

	// Full-size (no upsampling needed)
	info := &DecompressInfo{
		OutputWidth:      8,
		OutputHeight:     8,
		MaxHSampFactor:   1,
		MaxVSampFactor:   1,
		MinDCTHScalSize:  8,
		MinDCTVScalSize:  8,
		NumComponents:    1,
		OutColorComponents: 1,
		OutputComponents: 1,
		CompInfo: []ComponentInfo{
			{
				ComponentNeeded: true,
				HSampFactor:     1,
				VSampFactor:     1,
				DCTHScalSize:    8,
				DCTVScalSize:    8,
			},
		},
	}

	cc := NewColorConverter(info)
	u := NewUpsampler(info, cc)
	if u == nil {
		t.Fatal("NewUpsampler returned nil")
	}
	if !u.isFullSize[0] {
		t.Error("expected full-size upsampling for 1:1 sampling")
	}
}

func TestUpsamplerStartPass(t *testing.T) {
	t.Parallel()

	info := &DecompressInfo{
		OutputWidth:      8,
		OutputHeight:     8,
		MaxHSampFactor:   1,
		MaxVSampFactor:   1,
		MinDCTHScalSize:  8,
		MinDCTVScalSize:  8,
		NumComponents:    1,
		OutColorComponents: 1,
		OutputComponents: 1,
		CompInfo: []ComponentInfo{
			{
				ComponentNeeded: true,
				HSampFactor:     1,
				VSampFactor:     1,
				DCTHScalSize:    8,
				DCTVScalSize:    8,
			},
		},
	}

	cc := NewColorConverter(info)
	u := NewUpsampler(info, cc)
	u.StartPass()

	if u.nextRowOut != 1 {
		t.Errorf("nextRowOut = %d, want 1", u.nextRowOut)
	}
	if u.rowsToGo != 8 {
		t.Errorf("rowsToGo = %d, want 8", u.rowsToGo)
	}
}

func TestNeedContextRows(t *testing.T) {
	t.Parallel()

	info := &DecompressInfo{
		OutputWidth:      8,
		OutputHeight:     8,
		MaxHSampFactor:   1,
		MaxVSampFactor:   1,
		MinDCTHScalSize:  8,
		MinDCTVScalSize:  8,
		NumComponents:    1,
		OutColorComponents: 1,
		OutputComponents: 1,
		CompInfo: []ComponentInfo{
			{
				ComponentNeeded: true,
				HSampFactor:     1,
				VSampFactor:     1,
				DCTHScalSize:    8,
				DCTVScalSize:    8,
			},
		},
	}

	cc := NewColorConverter(info)
	u := NewUpsampler(info, cc)

	// The separate upsampler uses simple box-filter replication,
	// which does not require context rows.
	if u.NeedContextRows() {
		t.Error("NeedContextRows() = true, want false for simple upsampler")
	}
}
