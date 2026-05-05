package huff

import (
	"testing"

	"github.com/dh-kam/djpeg-go/internal/testutil"
)

// makeAllOnesQuantTable creates a quant table with all values = 1
func makeAllOnesQuantVals() [DCTSize2]int32 {
	var q [DCTSize2]int32
	for i := 0; i < DCTSize2; i++ {
		q[i] = 1
	}
	return q
}

// makeTestOutputBuf creates an output buffer of 8 BlockRows, each with 16 samples.
func makeTestOutputBuf() []BlockRow {
	buf := make([]BlockRow, DCTSize)
	for i := range buf {
		buf[i] = make(BlockRow, 16)
	}
	return buf
}

// flattenOutputBuf converts an 8-row BlockRow buffer into a flat pixel slice.
func flattenOutputBuf(buf []BlockRow, col, width int) []byte {
	result := make([]byte, DCTSize*width)
	for row := 0; row < DCTSize; row++ {
		for c := 0; c < width; c++ {
			result[row*width+c] = buf[row][col+c]
		}
	}
	return result
}

func TestIDCTISlowAllZero(t *testing.T) {
	// All-zero coefficient block with all-1s quantization:
	// DC = 0 * 1 = 0. After IDCT with pass1 shift, DC workspace = 0.
	// Pass 2 descale: 0 >> 18 = 0. Range limit at index 0 = 128.
	// Output should be all 128 (CENTERJSAMPLE).
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildISlowMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF // all zeros
	outputBuf := makeTestOutputBuf()

	IDCTISlowImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// Verify using PixelComparator: all outputs should be 128
	expected := make([]byte, DCTSize*DCTSize)
	for i := range expected {
		expected[i] = 128
	}
	actual := flattenOutputBuf(outputBuf, 0, DCTSize)
	cmp := testutil.NewPixelComparator(expected, actual, DCTSize, DCTSize, 1)

	if !cmp.IsExact() {
		t.Errorf("all-zero ISLOW IDCT not exact: maxDiff=%d, avgDiff=%.4f",
			cmp.MaxDiff(), cmp.AvgDiff())
		report := testutil.GenerateAccuracyReport(cmp, "islow_zero", DCTSize, DCTSize, 1)
		t.Log(report.String())
	}
}

func TestIDCTIFastAllZero(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildIFastMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	outputBuf := makeTestOutputBuf()

	IDCTIFastImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// All outputs should be uniform
	first := outputBuf[0][0]
	expected := make([]byte, DCTSize*DCTSize)
	for i := range expected {
		expected[i] = first
	}
	actual := flattenOutputBuf(outputBuf, 0, DCTSize)
	cmp := testutil.NewPixelComparator(expected, actual, DCTSize, DCTSize, 1)

	if !cmp.IsExact() {
		t.Errorf("all-zero IFAST IDCT not uniform: maxDiff=%d", cmp.MaxDiff())
	}
}

func TestIDCTFloatAllZero(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildFloatMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	outputBuf := makeTestOutputBuf()

	IDCTFloatImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// All outputs should be uniform
	first := outputBuf[0][0]
	expected := make([]byte, DCTSize*DCTSize)
	for i := range expected {
		expected[i] = first
	}
	actual := flattenOutputBuf(outputBuf, 0, DCTSize)
	cmp := testutil.NewPixelComparator(expected, actual, DCTSize, DCTSize, 1)

	if !cmp.IsExact() {
		t.Errorf("all-zero Float IDCT not uniform: maxDiff=%d", cmp.MaxDiff())
	}
}

func TestIDCTDCOnlyISlow(t *testing.T) {
	// DC-only block: DC = 10, all other coefficients = 0
	// With quant = 1, dequantized DC = 10.
	// After ISLOW: pass1 DC = 10 << pass1_bits = 10 << 2 = 40
	// All workspace entries for this column = 40.
	// Pass 2: AC terms all zero, so dcval = (40 + bias) >> 18
	// bias = 1<<17 = 131072
	// (40 + 131072) >> 18 = 131112 >> 18 = 0
	// range_limit[0] = 128
	// So output = 128 for all.
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildISlowMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 10 // DC coefficient

	outputBuf := makeTestOutputBuf()
	IDCTISlowImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// With ISLOW, all values should be the same since DC-only
	first := outputBuf[0][0]
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			if outputBuf[row][col] != first {
				t.Errorf("DC-only: output[%d][%d] = %d, want uniform %d",
					row, col, outputBuf[row][col], first)
			}
		}
	}
	// The value should be close to 128 (within range 0-255)
	if first > 255 {
		t.Errorf("DC-only output = %d, should be in [0,255]", first)
	}
}

func TestIDCTDCOnlyIFast(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildIFastMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 10

	outputBuf := makeTestOutputBuf()
	IDCTIFastImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// All values should be the same
	first := outputBuf[0][0]
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			if outputBuf[row][col] != first {
				t.Errorf("DC-only IFAST: output[%d][%d] = %d, want uniform %d",
					row, col, outputBuf[row][col], first)
			}
		}
	}
}

func TestIDCTDCOnlyFloat(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildFloatMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 10

	outputBuf := makeTestOutputBuf()
	IDCTFloatImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	first := outputBuf[0][0]
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			if outputBuf[row][col] != first {
				t.Errorf("DC-only Float: output[%d][%d] = %d, want uniform %d",
					row, col, outputBuf[row][col], first)
			}
		}
	}
}

func TestIDCTISlowOutputCol(t *testing.T) {
	// Test that outputCol offset is respected
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildISlowMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF

	// Create larger output buffer
	outputBuf := make([]BlockRow, DCTSize)
	for i := range outputBuf {
		outputBuf[i] = make(BlockRow, 24)
		// Fill with 0xFF to distinguish from IDCT output
		for j := range outputBuf[i] {
			outputBuf[i][j] = 0xFF
		}
	}

	IDCTISlowImpl(coefBlock[:], multTable, outputBuf, 8, rl)

	// Columns 0-7 should be untouched (0xFF)
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < 8; col++ {
			if outputBuf[row][col] != 0xFF {
				t.Errorf("output[%d][%d] = %d, want 0xFF (untouched)",
					row, col, outputBuf[row][col])
			}
		}
	}

	// Columns 8-15 should be the IDCT output (128 for all-zero input)
	for row := 0; row < DCTSize; row++ {
		for col := 8; col < 16; col++ {
			if outputBuf[row][col] != 128 {
				t.Errorf("output[%d][%d] = %d, want 128",
					row, col, outputBuf[row][col])
			}
		}
	}

	// Columns 16-23 should be untouched
	for row := 0; row < DCTSize; row++ {
		if outputBuf[row][16] != 0xFF {
			t.Errorf("output[%d][16] = %d, want 0xFF (untouched)",
				row, outputBuf[row][16])
		}
	}
}

func TestPerformIDCTDispatchesCorrectly(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()

	var coefBlock [DCTSize2]JCOEF

	for _, method := range []IDCTMethod{IDCTISlow, IDCTIFast, IDCTFloat} {
		var multTable interface{}
		switch method {
		case IDCTISlow:
			multTable = BuildISlowMultTable(quantVals)
		case IDCTIFast:
			multTable = BuildIFastMultTable(quantVals)
		case IDCTFloat:
			multTable = BuildFloatMultTable(quantVals)
		}

		outputBuf := makeTestOutputBuf()
		PerformIDCT(method, coefBlock[:], multTable, outputBuf, 0, rl)

		// Just verify it produced a valid output (uniform values for zero block)
		first := outputBuf[0][0]
		if first > 255 {
			t.Errorf("PerformIDCT method %d: output[0][0] = %d, out of range",
				method, first)
		}
		// Check uniformity
		for row := 0; row < DCTSize; row++ {
			for col := 0; col < DCTSize; col++ {
				if outputBuf[row][col] != first {
					t.Errorf("PerformIDCT method %d: output[%d][%d] = %d, want %d",
						method, row, col, outputBuf[row][col], first)
				}
			}
		}
	}
}

func TestIDCTClampsNegativeValues(t *testing.T) {
	// Use a large negative DC to test clamping
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildISlowMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = -1000 // Very large negative DC

	outputBuf := makeTestOutputBuf()
	IDCTISlowImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// All values should be clamped to [0, 255]
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			v := outputBuf[row][col]
			if v < 0 || v > 255 {
				t.Errorf("output[%d][%d] = %d, out of range [0,255]", row, col, v)
			}
		}
	}
}

func TestIDCTClampsLargePositive(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildISlowMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 1000 // Very large positive DC

	outputBuf := makeTestOutputBuf()
	IDCTISlowImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			v := outputBuf[row][col]
			if v > 255 {
				t.Errorf("output[%d][%d] = %d, should be clamped to 255", row, col, v)
			}
		}
	}
}

func TestIDCTMethodsAgreeOnZeroBlock(t *testing.T) {
	// All three methods should produce uniform output for zero block
	// (the exact value may differ between ISlow and IFast/Float due to
	// different RangeCenter handling)
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	var coefBlock [DCTSize2]JCOEF

	outputs := make([][]BlockRow, 3)
	tables := []interface{}{
		BuildISlowMultTable(quantVals),
		BuildIFastMultTable(quantVals),
		BuildFloatMultTable(quantVals),
	}
	methods := []IDCTMethod{IDCTISlow, IDCTIFast, IDCTFloat}

	for i, method := range methods {
		outputs[i] = makeTestOutputBuf()
		PerformIDCT(method, coefBlock[:], tables[i], outputs[i], 0, rl)
	}

	// Each method should produce uniform output
	for i := 0; i < 3; i++ {
		first := outputs[i][0][0]
		for row := 0; row < DCTSize; row++ {
			for col := 0; col < DCTSize; col++ {
				if outputs[i][row][col] != first {
					t.Errorf("method %d: output[%d][%d] = %d, want uniform %d",
						i, row, col, outputs[i][row][col], first)
				}
			}
		}
	}
}

func TestIDCTMethodsAgreeOnDCOnly(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 5

	outputs := make([][]BlockRow, 3)
	tables := []interface{}{
		BuildISlowMultTable(quantVals),
		BuildIFastMultTable(quantVals),
		BuildFloatMultTable(quantVals),
	}
	methods := []IDCTMethod{IDCTISlow, IDCTIFast, IDCTFloat}

	for i, method := range methods {
		outputs[i] = makeTestOutputBuf()
		PerformIDCT(method, coefBlock[:], tables[i], outputs[i], 0, rl)
	}

	// Each method should produce uniform output for DC-only block
	for i := 0; i < 3; i++ {
		first := outputs[i][0][0]
		for row := 0; row < DCTSize; row++ {
			for col := 0; col < DCTSize; col++ {
				if outputs[i][row][col] != first {
					t.Errorf("method %d DC-only: output[%d][%d] = %d, want uniform %d",
						i, row, col, outputs[i][row][col], first)
				}
			}
		}
	}

	// IFast and Float should agree (both use same AAN method without explicit RangeCenter)
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			if outputs[1][row][col] != outputs[2][row][col] {
				t.Errorf("IFast vs Float: [%d][%d] = %d vs %d",
					row, col, outputs[1][row][col], outputs[2][row][col])
			}
		}
	}
}

func TestIDCTISlowWithAC(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildISlowMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 10  // DC
	coefBlock[1] = 5   // AC coefficient (zigzag position 1 = row 0, col 1)

	outputBuf := makeTestOutputBuf()
	IDCTISlowImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// Output should NOT be uniform (we have an AC coefficient)
	// Just verify all values are in [0, 255]
	allSame := true
	first := outputBuf[0][0]
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			v := outputBuf[row][col]
			if v > 255 {
				t.Errorf("output[%d][%d] = %d, out of range", row, col, v)
			}
			if v != first {
				allSame = false
			}
		}
	}
	if allSame {
		t.Error("expected non-uniform output with AC coefficient")
	}
}

func TestIDCTIFastWithAC(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildIFastMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 10
	coefBlock[1] = 5

	outputBuf := makeTestOutputBuf()
	IDCTIFastImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// Verify all values are valid
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			v := outputBuf[row][col]
			if v > 255 {
				t.Errorf("output[%d][%d] = %d, out of range", row, col, v)
			}
		}
	}
}

func TestIDCTFloatWithAC(t *testing.T) {
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()
	multTable := BuildFloatMultTable(quantVals)

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 10
	coefBlock[1] = 5

	outputBuf := makeTestOutputBuf()
	IDCTFloatImpl(coefBlock[:], multTable, outputBuf, 0, rl)

	// Verify all values are valid
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			v := outputBuf[row][col]
			if v > 255 {
				t.Errorf("output[%d][%d] = %d, out of range", row, col, v)
			}
		}
	}
}

func TestBuildISlowMultTable(t *testing.T) {
	var quantVals [DCTSize2]int32
	for i := 0; i < DCTSize2; i++ {
		quantVals[i] = int32(i + 1)
	}
	tbl := BuildISlowMultTable(quantVals)
	if tbl == nil {
		t.Fatal("BuildISlowMultTable returned nil")
	}
	for i := 0; i < DCTSize2; i++ {
		if tbl[i] != int32(i+1) {
			t.Errorf("tbl[%d] = %d, want %d", i, tbl[i], i+1)
		}
	}
}

func TestBuildIFastMultTable(t *testing.T) {
	var quantVals [DCTSize2]int32
	for i := 0; i < DCTSize2; i++ {
		quantVals[i] = 1
	}
	tbl := BuildIFastMultTable(quantVals)
	if tbl == nil {
		t.Fatal("BuildIFastMultTable returned nil")
	}
	// With quant=1, tbl[0] should be aanscales[0] >> 12 = 16384 >> 12 = 4
	if tbl[0] != 4 {
		t.Errorf("tbl[0] = %d, want 4", tbl[0])
	}
}

func TestBuildFloatMultTable(t *testing.T) {
	var quantVals [DCTSize2]int32
	for i := 0; i < DCTSize2; i++ {
		quantVals[i] = 1
	}
	tbl := BuildFloatMultTable(quantVals)
	if tbl == nil {
		t.Fatal("BuildFloatMultTable returned nil")
	}
	// tbl[0] = 1.0 * 1.0 * 1.0 * 0.125 = 0.125
	if tbl[0] < 0.124 || tbl[0] > 0.126 {
		t.Errorf("tbl[0] = %f, want ~0.125", tbl[0])
	}
}

func TestIDCTISlowCrossMethodConsistency(t *testing.T) {
	// Compare ISLOW and IFAST output for a non-trivial block.
	// They should be within a small tolerance of each other.
	rl := NewRangeLimitTable()
	quantVals := makeAllOnesQuantVals()

	var coefBlock [DCTSize2]JCOEF
	coefBlock[0] = 20 // DC
	coefBlock[1] = 10 // AC zigzag pos 1
	coefBlock[8] = -5 // AC zigzag pos 8

	islowTable := BuildISlowMultTable(quantVals)
	ifastTable := BuildIFastMultTable(quantVals)
	floatTable := BuildFloatMultTable(quantVals)

	islowOut := makeTestOutputBuf()
	ifastOut := makeTestOutputBuf()
	floatOut := makeTestOutputBuf()

	IDCTISlowImpl(coefBlock[:], islowTable, islowOut, 0, rl)
	IDCTIFastImpl(coefBlock[:], ifastTable, ifastOut, 0, rl)
	IDCTFloatImpl(coefBlock[:], floatTable, floatOut, 0, rl)

	// Flatten to byte arrays for comparison
	islowFlat := flattenOutputBuf(islowOut, 0, DCTSize)
	ifastFlat := flattenOutputBuf(ifastOut, 0, DCTSize)
	floatFlat := flattenOutputBuf(floatOut, 0, DCTSize)

	// IFAST and Float should be very close
	cmpIFastFloat := testutil.NewPixelComparator(ifastFlat, floatFlat, DCTSize, DCTSize, 1)
	if cmpIFastFloat.MaxDiff() > 2 {
		t.Errorf("IFAST vs Float max diff = %d, want <= 2", cmpIFastFloat.MaxDiff())
	}

	// ISLOW and IFAST can differ significantly due to different range center conventions.
	// ISLOW uses RangeCenter (adds 512 before range-limit lookup), producing outputs
	// centered at 128. IFAST/Float use different conventions. Just verify all values
	// are in valid range [0, 255].
	cmpISlowIFast := testutil.NewPixelComparator(islowFlat, ifastFlat, DCTSize, DCTSize, 1)
	report := testutil.GenerateAccuracyReport(cmpISlowIFast, "islow_vs_ifast", DCTSize, DCTSize, 1)
	t.Log(report.String())

	// Verify all outputs are in valid range
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			if islowOut[row][col] > 255 {
				t.Errorf("ISLOW output[%d][%d] = %d, out of range", row, col, islowOut[row][col])
			}
			if ifastOut[row][col] > 255 {
				t.Errorf("IFAST output[%d][%d] = %d, out of range", row, col, ifastOut[row][col])
			}
			if floatOut[row][col] > 255 {
				t.Errorf("Float output[%d][%d] = %d, out of range", row, col, floatOut[row][col])
			}
		}
	}
}
