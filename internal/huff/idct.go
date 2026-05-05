package huff

// IDCT manager ported from jddctmgr.c.
//
// This module selects a particular IDCT implementation and builds
// the multiplier table from quantization table values.
// The IDCT routines are responsible for performing coefficient
// dequantization as well as the IDCT proper.

// IDCTMethod identifies the IDCT algorithm to use.
type IDCTMethod int

const (
	IDCTISlow IDCTMethod = iota // accurate integer (default, most important)
	IDCTIFast                   // fast integer (AA&N)
	IDCTFloat                   // floating-point
)

// AAN scaling factors for the fast integer IDCT multiplier table.
// Precomputed values scaled up by 14 bits (CONST_BITS = 14).
// From jddctmgr.c.
var aanscales = [DCTSize2]int16{
	16384, 22725, 21407, 19266, 16384, 12873, 8867, 4520,
	22725, 31521, 29692, 26722, 22725, 17855, 12299, 6270,
	21407, 29692, 27969, 25172, 21407, 16819, 11585, 5906,
	19266, 26722, 25172, 22654, 19266, 15137, 10426, 5315,
	16384, 22725, 21407, 19266, 16384, 12873, 8867, 4520,
	12873, 17855, 16819, 15137, 12873, 10114, 6967, 3552,
	8867, 12299, 11585, 10426, 8867, 6967, 4799, 2446,
	4520, 6270, 5906, 5315, 4520, 3552, 2446, 1247,
}

// AAN scaling factors for the floating-point IDCT.
var aanscalefactor = [DCTSize]float64{
	1.0, 1.387039845, 1.306562965, 1.175875602,
	1.0, 0.785694958, 0.541196100, 0.275899379,
}

// BuildISlowMultTable creates the multiplier table for the accurate
// integer IDCT. For the ISLOW method, multipliers are equal to raw
// quantization coefficients.
func BuildISlowMultTable(quantVals [DCTSize2]int32) *ISlowMultTable {
	var tbl ISlowMultTable
	copy(tbl[:], quantVals[:])
	return &tbl
}

// BuildIFastMultTable creates the multiplier table for the fast integer IDCT.
// The AA&N method uses scaled quantization coefficients.
func BuildIFastMultTable(quantVals [DCTSize2]int32) *IFASTMultTable {
	var tbl IFASTMultTable
	const constBits = 14
	const ifastScaleBits = 2
	for i := 0; i < DCTSize2; i++ {
		// DESCALE(MULTIPLY16V16(q, aanscales[i]), CONST_BITS-IFAST_SCALE_BITS)
		// = (q * aanscales[i]) >> (14 - 2) = (q * aanscales[i]) >> 12
		product := int32(quantVals[i]) * int32(aanscales[i])
		tbl[i] = int32(product >> (constBits - ifastScaleBits))
	}
	return &tbl
}

// BuildFloatMultTable creates the multiplier table for the floating-point IDCT.
func BuildFloatMultTable(quantVals [DCTSize2]int32) *FloatMultTable {
	var tbl FloatMultTable
	i := 0
	for row := 0; row < DCTSize; row++ {
		for col := 0; col < DCTSize; col++ {
			tbl[i] = float64(quantVals[i]) *
				aanscalefactor[row] * aanscalefactor[col] * 0.125
			i++
		}
	}
	return &tbl
}

// PerformIDCT dispatches to the appropriate IDCT implementation based on
// the method, performs dequantization and IDCT on the coefficient block,
// and writes the result to the output buffer.
//
// coefBlock: 64 DCT coefficients in zigzag order
// outputBuf: rows of output samples
// outputCol: starting column in output rows
// rangeLimit: range-limiting lookup table
func PerformIDCT(method IDCTMethod, coefBlock []JCOEF, multTable interface{},
	outputBuf []BlockRow, outputCol int, rangeLimit *RangeLimitTable) {

	switch method {
	case IDCTISlow:
		IDCTISlowImpl(coefBlock, multTable.(*ISlowMultTable), outputBuf, outputCol, rangeLimit)
	case IDCTIFast:
		IDCTIFastImpl(coefBlock, multTable.(*IFASTMultTable), outputBuf, outputCol, rangeLimit)
	case IDCTFloat:
		IDCTFloatImpl(coefBlock, multTable.(*FloatMultTable), outputBuf, outputCol, rangeLimit)
	}
}
