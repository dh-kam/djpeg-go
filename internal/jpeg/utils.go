package jpeg

// Utility functions ported from IJG libjpeg 9f (jutils.c).

// JDivRoundUp computes a/b rounded up to next integer, i.e., ceil(a/b).
// Assumes a >= 0, b > 0.
func JDivRoundUp(a, b int64) int64 {
	return (a + b - 1) / b
}

// JRoundUp computes a rounded up to next multiple of b, i.e., ceil(a/b)*b.
// Assumes a >= 0, b > 0.
func JRoundUp(a, b int64) int64 {
	a += b - 1
	return a - (a % b)
}

// JZeroFar zeroes out a slice of bytes. This replaces the C jzero_far function.
func JZeroFar(target []byte) {
	for i := range target {
		target[i] = 0
	}
}

// ZeroSamples zeroes out a slice of JSAMPLE.
func ZeroSamples(target []JSAMPLE) {
	for i := range target {
		target[i] = 0
	}
}

// ZeroCoeffs zeroes out a slice of JCOEF.
func ZeroCoeffs(target []JCOEF) {
	for i := range target {
		target[i] = 0
	}
}

// JCopySampleRows copies some rows of samples from one place to another.
// numRows rows are copied from inputArray to outputArray;
// these areas may overlap for duplication.
// The source and destination arrays must be at least as wide as numCols.
func JCopySampleRows(inputArray, outputArray JSAMPARRAY, numRows, numCols int) {
	for row := 0; row < numRows; row++ {
		if row < len(inputArray) && row < len(outputArray) {
			src := inputArray[row]
			dst := outputArray[row]
			n := numCols
			if n > len(src) {
				n = len(src)
			}
			if n > len(dst) {
				n = len(dst)
			}
			copy(dst[:n], src[:n])
		}
	}
}

// JCopyBlockRow copies a row of coefficient blocks from one place to another.
func JCopyBlockRow(inputRow, outputRow JBLOCKROW, numBlocks int) {
	n := numBlocks
	if n > len(inputRow) {
		n = len(inputRow)
	}
	if n > len(outputRow) {
		n = len(outputRow)
	}
	copy(outputRow[:n], inputRow[:n])
}

// MinInt returns the minimum of two ints.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxInt returns the maximum of two ints.
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ClampInt clamps v to the range [lo, hi].
func ClampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
