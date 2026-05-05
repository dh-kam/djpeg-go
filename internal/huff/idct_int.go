package huff

// Accurate integer IDCT implementation.
// Ported from jidctint.c (IJG libjpeg 9f).
//
// This is a slow-but-accurate integer implementation of the inverse DCT.
// It must also perform dequantization of the input coefficients.
//
// The algorithm is based on:
//   C. Loeffler, A. Ligtenberg and G. Moschytz, "Practical Fast 1-D DCT
//   Algorithms with 11 Multiplications", Proc. ICASSP '89, pp. 988-991.
//
// We use the alternate method with 12 multiplications and 32 additions.
//
// Fixed-point arithmetic constants (for 8-bit JSAMPLE):
//   CONST_BITS  = 13
//   PASS1_BITS  = 2
//
// Each 1-D IDCT step produces outputs that are a factor of sqrt(8) larger
// than the true IDCT outputs. The final outputs are 8x larger than desired,
// cured by a right shift at the end.

// Fixed-point constants for CONST_BITS = 13.
const (
	islowConstBits = 13
	islowPass1Bits = 2

	// ONE is 1.0 in fixed-point (1 << CONST_BITS).
	islowOne = 1 << islowConstBits

	// Pre-calculated constants for CONST_BITS = 13.
	fix0_298631336 int32 = 2446   // FIX(0.298631336)
	fix0_390180644 int32 = 3196   // FIX(0.390180644)
	fix0_541196100 int32 = 4433   // FIX(0.541196100)
	fix0_765366865 int32 = 6270   // FIX(0.765366865)
	fix0_899976223 int32 = 7373   // FIX(0.899976223)
	fix1_175875602 int32 = 9633   // FIX(1.175875602)
	fix1_501321110 int32 = 12299  // FIX(1.501321110)
	fix1_847759065 int32 = 15137  // FIX(1.847759065)
	fix1_961570560 int32 = 16069  // FIX(1.961570560)
	fix2_053119869 int32 = 16819  // FIX(2.053119869)
	fix2_562915447 int32 = 20995  // FIX(2.562915447)
	fix3_072711026 int32 = 25172  // FIX(3.072711026)
)

// multiply16c16 multiplies two 16-bit values (var * const) to yield int32.
// For 8-bit samples with recommended scaling, both values fit in 16 bits.
func multiply16c16(var_, const_ int32) int32 {
	return var_ * const_
}

// IDCTISlowImpl performs dequantization and inverse DCT on one block of
// coefficients using the accurate integer (ISLOW) method.
// This is the most important IDCT implementation.
//
// Ported from jpeg_idct_islow in jidctint.c.
// Optimized: all arithmetic uses int32 consistently, range-limit inlined.
func IDCTISlowImpl(coefBlock []JCOEF, quantptr *ISlowMultTable,
	outputBuf []BlockRow, outputCol int, rangeLimit *RangeLimitTable) {

	var workspace [DCTSize2]int32

	// Precompute shifts as constants for the compiler.
	const (
		shift1 = islowConstBits - islowPass1Bits // 11
		shift2 = islowPass1Bits + 3              // 5
		shift3 = islowConstBits + islowPass1Bits + 3 // 18
	)

	// Pass 1: process columns from input, store into work array.
	for ctr := 0; ctr < DCTSize; ctr++ {
		inptr := ctr
		wsptr := ctr

		// Check if all AC terms are zero (common case).
		if coefBlock[inptr+DCTSize*1] == 0 &&
			coefBlock[inptr+DCTSize*2] == 0 &&
			coefBlock[inptr+DCTSize*3] == 0 &&
			coefBlock[inptr+DCTSize*4] == 0 &&
			coefBlock[inptr+DCTSize*5] == 0 &&
			coefBlock[inptr+DCTSize*6] == 0 &&
			coefBlock[inptr+DCTSize*7] == 0 {
			dcval := int32(coefBlock[inptr]) * quantptr[ctr]
			dcval <<= islowPass1Bits
			workspace[wsptr] = dcval
			workspace[wsptr+DCTSize] = dcval
			workspace[wsptr+DCTSize*2] = dcval
			workspace[wsptr+DCTSize*3] = dcval
			workspace[wsptr+DCTSize*4] = dcval
			workspace[wsptr+DCTSize*5] = dcval
			workspace[wsptr+DCTSize*6] = dcval
			workspace[wsptr+DCTSize*7] = dcval
			continue
		}

		// Even part
		z2 := int32(coefBlock[inptr]) * quantptr[ctr]
		z3 := int32(coefBlock[inptr+DCTSize*4]) * quantptr[DCTSize*4+ctr]
		z2 <<= islowConstBits
		z3 <<= islowConstBits
		z2 += int32(1) << (islowConstBits - islowPass1Bits - 1)

		tmp0 := z2 + z3
		tmp1 := z2 - z3

		z2 = int32(coefBlock[inptr+DCTSize*2]) * quantptr[DCTSize*2+ctr]
		z3 = int32(coefBlock[inptr+DCTSize*6]) * quantptr[DCTSize*6+ctr]

		z1 := (z2 + z3) * fix0_541196100
		tmp2 := z1 + z2*fix0_765366865
		tmp3 := z1 - z3*fix1_847759065

		tmp10 := tmp0 + tmp2
		tmp13 := tmp0 - tmp2
		tmp11 := tmp1 + tmp3
		tmp12 := tmp1 - tmp3

		// Odd part
		tmp0 = int32(coefBlock[inptr+DCTSize*7]) * quantptr[DCTSize*7+ctr]
		tmp1 = int32(coefBlock[inptr+DCTSize*5]) * quantptr[DCTSize*5+ctr]
		tmp2 = int32(coefBlock[inptr+DCTSize*3]) * quantptr[DCTSize*3+ctr]
		tmp3 = int32(coefBlock[inptr+DCTSize*1]) * quantptr[DCTSize*1+ctr]

		z2 = tmp0 + tmp2
		z3 = tmp1 + tmp3

		z1 = (z2 + z3) * fix1_175875602
		z2 = z2 * (-fix1_961570560)
		z3 = z3 * (-fix0_390180644)
		z2 += z1
		z3 += z1

		z1 = (tmp0 + tmp3) * (-fix0_899976223)
		tmp0 = tmp0*fix0_298631336 + z1 + z2
		tmp3 = tmp3*fix1_501321110 + z1 + z3

		z1 = (tmp1 + tmp2) * (-fix2_562915447)
		tmp1 = tmp1*fix2_053119869 + z1 + z3
		tmp2 = tmp2*fix3_072711026 + z1 + z2

		workspace[wsptr] = (tmp10 + tmp3) >> shift1
		workspace[wsptr+DCTSize*7] = (tmp10 - tmp3) >> shift1
		workspace[wsptr+DCTSize] = (tmp11 + tmp2) >> shift1
		workspace[wsptr+DCTSize*6] = (tmp11 - tmp2) >> shift1
		workspace[wsptr+DCTSize*2] = (tmp12 + tmp1) >> shift1
		workspace[wsptr+DCTSize*5] = (tmp12 - tmp1) >> shift1
		workspace[wsptr+DCTSize*3] = (tmp13 + tmp0) >> shift1
		workspace[wsptr+DCTSize*4] = (tmp13 - tmp0) >> shift1
	}

	// Pass 2: process rows from work array, store into output array.
	// Inline range-limit: rl[value & RangeMask]
	rl := (*[5 * 256]JSAMPLE)(rangeLimit)

	for ctr := 0; ctr < DCTSize; ctr++ {
		wsptr := ctr * DCTSize
		outptr := outputBuf[ctr]
		outIdx := outputCol

		// Add range center and fudge factor.
		z2 := workspace[wsptr] +
			(int32(RangeCenter)<<shift2)+
			(int32(1)<<(shift2-1))

		// Check if AC terms are all zero.
		if workspace[wsptr+1] == 0 && workspace[wsptr+2] == 0 &&
			workspace[wsptr+3] == 0 && workspace[wsptr+4] == 0 &&
			workspace[wsptr+5] == 0 && workspace[wsptr+6] == 0 &&
			workspace[wsptr+7] == 0 {
			dcval := rl[z2>>shift2&RangeMask]
			outptr[outIdx] = dcval
			outptr[outIdx+1] = dcval
			outptr[outIdx+2] = dcval
			outptr[outIdx+3] = dcval
			outptr[outIdx+4] = dcval
			outptr[outIdx+5] = dcval
			outptr[outIdx+6] = dcval
			outptr[outIdx+7] = dcval
			continue
		}

		// Even part
		z3 := workspace[wsptr+4]
		tmp0 := (z2 + z3) << islowConstBits
		tmp1 := (z2 - z3) << islowConstBits

		z2 = workspace[wsptr+2]
		z3 = workspace[wsptr+6]

		z1 := (z2 + z3) * fix0_541196100
		tmp2 := z1 + z2*fix0_765366865
		tmp3 := z1 - z3*fix1_847759065

		tmp10 := tmp0 + tmp2
		tmp13 := tmp0 - tmp2
		tmp11 := tmp1 + tmp3
		tmp12 := tmp1 - tmp3

		// Odd part
		tmp0 = workspace[wsptr+7]
		tmp1 = workspace[wsptr+5]
		tmp2 = workspace[wsptr+3]
		tmp3 = workspace[wsptr+1]

		z2 = tmp0 + tmp2
		z3 = tmp1 + tmp3

		z1 = (z2 + z3) * fix1_175875602
		z2 = z2 * (-fix1_961570560)
		z3 = z3 * (-fix0_390180644)
		z2 += z1
		z3 += z1

		z1 = (tmp0 + tmp3) * (-fix0_899976223)
		tmp0 = tmp0*fix0_298631336 + z1 + z2
		tmp3 = tmp3*fix1_501321110 + z1 + z3

		z1 = (tmp1 + tmp2) * (-fix2_562915447)
		tmp1 = tmp1*fix2_053119869 + z1 + z3
		tmp2 = tmp2*fix3_072711026 + z1 + z2

		// Final output with inlined range-limit
		outptr[outIdx] = rl[((tmp10 + tmp3) >> shift3) & RangeMask]
		outptr[outIdx+7] = rl[((tmp10 - tmp3) >> shift3) & RangeMask]
		outptr[outIdx+1] = rl[((tmp11 + tmp2) >> shift3) & RangeMask]
		outptr[outIdx+6] = rl[((tmp11 - tmp2) >> shift3) & RangeMask]
		outptr[outIdx+2] = rl[((tmp12 + tmp1) >> shift3) & RangeMask]
		outptr[outIdx+5] = rl[((tmp12 - tmp1) >> shift3) & RangeMask]
		outptr[outIdx+3] = rl[((tmp13 + tmp0) >> shift3) & RangeMask]
		outptr[outIdx+4] = rl[((tmp13 - tmp0) >> shift3) & RangeMask]
	}
}

// rangeLimitGet performs the range-limiting function.
// The C code uses: range_limit[(value) & RANGE_MASK]
// where RANGE_MASK = 1023 and the table is the post-IDCT portion of
// sample_range_limit (pre-offset by CENTERJSAMPLE).
func rangeLimitGet(rl *RangeLimitTable, value int) JSAMPLE {
	idx := value & RangeMask
	return rl[idx]
}
