package huff

// Fast integer IDCT implementation.
// Ported from jidctfst.c (IJG libjpeg 9f).
//
// This is a fast, not-so-accurate integer implementation based on the
// Arai, Agui, and Nakajima (AA&N) algorithm for scaled DCT.
//
// The AA&N method leaves only 5 multiplies and 29 adds in the 1-D DCT.
// The primary disadvantage is that with fixed-point math, accuracy is lost
// due to imprecise representation of the scaled quantization values.
//
// Scaling constants for 8-bit JSAMPLE:
//   CONST_BITS  = 8
//   PASS1_BITS  = 2

const (
	ifastConstBits = 8
	ifastPass1Bits = 2

	// Pre-calculated constants for CONST_BITS = 8.
	fstFix1_082392200 int32 = 277  // FIX(1.082392200)
	fstFix1_414213562 int32 = 362  // FIX(1.414213562)
	fstFix1_847759065 int32 = 473  // FIX(1.847759065)
	fstFix2_613125930 int32 = 669  // FIX(2.613125930)
)

// ifastMultiply multiplies a DCTELEM variable by an INT32 constant,
// and immediately descales to yield a result.
// MULTIPLY(var,const) = DESCALE(var * const, CONST_BITS)
func ifastMultiply(var_, const_ int32) int32 {
	return (var_ * const_) >> ifastConstBits
}

// IDCTIFastImpl performs dequantization and inverse DCT on one block
// of coefficients using the fast integer (AA&N) method.
// Ported from jpeg_idct_ifast in jidctfst.c.
func IDCTIFastImpl(coefBlock []JCOEF, quantptr *IFASTMultTable,
	outputBuf []BlockRow, outputCol int, rangeLimit *RangeLimitTable) {

	var workspace [DCTSize2]int32

	// Pass 1: process columns from input, store into work array.
	for ctr := 0; ctr < DCTSize; ctr++ {
		inptr := ctr
		wsptr := ctr

		// Check if all AC terms are zero.
		if coefBlock[inptr+DCTSize*1] == 0 &&
			coefBlock[inptr+DCTSize*2] == 0 &&
			coefBlock[inptr+DCTSize*3] == 0 &&
			coefBlock[inptr+DCTSize*4] == 0 &&
			coefBlock[inptr+DCTSize*5] == 0 &&
			coefBlock[inptr+DCTSize*6] == 0 &&
			coefBlock[inptr+DCTSize*7] == 0 {
			// AC terms all zero
			dcval := int32(coefBlock[inptr+DCTSize*0]) * quantptr[DCTSize*0+ctr]

			workspace[wsptr+DCTSize*0] = dcval
			workspace[wsptr+DCTSize*1] = dcval
			workspace[wsptr+DCTSize*2] = dcval
			workspace[wsptr+DCTSize*3] = dcval
			workspace[wsptr+DCTSize*4] = dcval
			workspace[wsptr+DCTSize*5] = dcval
			workspace[wsptr+DCTSize*6] = dcval
			workspace[wsptr+DCTSize*7] = dcval
			continue
		}

		// Even part
		tmp0 := int32(coefBlock[inptr+DCTSize*0]) * quantptr[DCTSize*0+ctr]
		tmp1 := int32(coefBlock[inptr+DCTSize*2]) * quantptr[DCTSize*2+ctr]
		tmp2 := int32(coefBlock[inptr+DCTSize*4]) * quantptr[DCTSize*4+ctr]
		tmp3 := int32(coefBlock[inptr+DCTSize*6]) * quantptr[DCTSize*6+ctr]

		tmp10 := tmp0 + tmp2 // phase 3
		tmp11 := tmp0 - tmp2

		tmp13 := tmp1 + tmp3 // phases 5-3
		tmp12 := ifastMultiply(tmp1-tmp3, fstFix1_414213562) - tmp13 // 2*c4

		tmp0 = tmp10 + tmp13 // phase 2
		tmp3 = tmp10 - tmp13
		tmp1 = tmp11 + tmp12
		tmp2 = tmp11 - tmp12

		// Odd part
		tmp4 := int32(coefBlock[inptr+DCTSize*1]) * quantptr[DCTSize*1+ctr]
		tmp5 := int32(coefBlock[inptr+DCTSize*3]) * quantptr[DCTSize*3+ctr]
		tmp6 := int32(coefBlock[inptr+DCTSize*5]) * quantptr[DCTSize*5+ctr]
		tmp7 := int32(coefBlock[inptr+DCTSize*7]) * quantptr[DCTSize*7+ctr]

		z13 := tmp6 + tmp5 // phase 6
		z10 := tmp6 - tmp5
		z11 := tmp4 + tmp7
		z12 := tmp4 - tmp7

		tmp7 = z11 + z13 // phase 5
		tmp11 = ifastMultiply(z11-z13, fstFix1_414213562) // 2*c4

		z5 := ifastMultiply(z10+z12, fstFix1_847759065)           // 2*c2
		tmp10 = z5 - ifastMultiply(z12, fstFix1_082392200)         // 2*(c2-c6)
		tmp12 = z5 - ifastMultiply(z10, fstFix2_613125930)         // 2*(c2+c6)

		tmp6 = tmp12 - tmp7 // phase 2
		tmp5 = tmp11 - tmp6
		tmp4 = tmp10 - tmp5

		workspace[wsptr+DCTSize*0] = tmp0 + tmp7
		workspace[wsptr+DCTSize*7] = tmp0 - tmp7
		workspace[wsptr+DCTSize*1] = tmp1 + tmp6
		workspace[wsptr+DCTSize*6] = tmp1 - tmp6
		workspace[wsptr+DCTSize*2] = tmp2 + tmp5
		workspace[wsptr+DCTSize*5] = tmp2 - tmp5
		workspace[wsptr+DCTSize*3] = tmp3 + tmp4
		workspace[wsptr+DCTSize*4] = tmp3 - tmp4
	}

	// Pass 2: process rows from work array, store into output array.
	// Descale by factor of 8 (2^3) and undo PASS1_BITS scaling.
	// The range_limit table handles the level shift (CENTERJSAMPLE)
	// internally, so we do NOT add RangeCenter here.
	// C code: range_limit[IDESCALE(wsptr[0], PASS1_BITS+3) & RANGE_MASK]
	shift := uint(ifastPass1Bits + 3) // 5

	for ctr := 0; ctr < DCTSize; ctr++ {
		wsptr := ctr * DCTSize
		outptr := outputBuf[ctr]
		outIdx := outputCol

		// Check if AC terms are all zero.
		if workspace[wsptr+1] == 0 && workspace[wsptr+2] == 0 &&
			workspace[wsptr+3] == 0 && workspace[wsptr+4] == 0 &&
			workspace[wsptr+5] == 0 && workspace[wsptr+6] == 0 &&
			workspace[wsptr+7] == 0 {
			// AC terms all zero: range_limit[IDESCALE(dcval, PASS1_BITS+3) & RANGE_MASK]
			dcval := rangeLimitGet(rangeLimit, int(workspace[wsptr+0]>>shift))
			outptr[outIdx+0] = dcval
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
		rtmp10 := workspace[wsptr+0] + workspace[wsptr+4]
		rtmp11 := workspace[wsptr+0] - workspace[wsptr+4]

		rtmp13 := workspace[wsptr+2] + workspace[wsptr+6]
		rtmp12 := ifastMultiply(workspace[wsptr+2]-workspace[wsptr+6],
			fstFix1_414213562) - rtmp13 // 2*c4

		rtmp0 := rtmp10 + rtmp13
		rtmp3 := rtmp10 - rtmp13
		rtmp1 := rtmp11 + rtmp12
		rtmp2 := rtmp11 - rtmp12

		// Odd part
		rz13 := workspace[wsptr+5] + workspace[wsptr+3]
		rz10 := workspace[wsptr+5] - workspace[wsptr+3]
		rz11 := workspace[wsptr+1] + workspace[wsptr+7]
		rz12 := workspace[wsptr+1] - workspace[wsptr+7]

		rtmp7 := rz11 + rz13 // phase 5
		rtmp11 = ifastMultiply(rz11-rz13, fstFix1_414213562) // 2*c4

		rz5 := ifastMultiply(rz10+rz12, fstFix1_847759065)          // 2*c2
		rtmp10 = rz5 - ifastMultiply(rz12, fstFix1_082392200)       // 2*(c2-c6)
		rtmp12 = rz5 - ifastMultiply(rz10, fstFix2_613125930)       // 2*(c2+c6)

		rtmp6 := rtmp12 - rtmp7 // phase 2
		rtmp5 := rtmp11 - rtmp6
		rtmp4 := rtmp10 - rtmp5

		// Final output stage: scale down by factor of 8 and range-limit
		// C code: range_limit[IDESCALE(tmp0+tmp7, PASS1_BITS+3) & RANGE_MASK]
		// The range_limit table handles the level shift internally.
		outptr[outIdx+0] = rangeLimitGet(rangeLimit, int((rtmp0+rtmp7)>>shift))
		outptr[outIdx+7] = rangeLimitGet(rangeLimit, int((rtmp0-rtmp7)>>shift))
		outptr[outIdx+1] = rangeLimitGet(rangeLimit, int((rtmp1+rtmp6)>>shift))
		outptr[outIdx+6] = rangeLimitGet(rangeLimit, int((rtmp1-rtmp6)>>shift))
		outptr[outIdx+2] = rangeLimitGet(rangeLimit, int((rtmp2+rtmp5)>>shift))
		outptr[outIdx+5] = rangeLimitGet(rangeLimit, int((rtmp2-rtmp5)>>shift))
		outptr[outIdx+3] = rangeLimitGet(rangeLimit, int((rtmp3+rtmp4)>>shift))
		outptr[outIdx+4] = rangeLimitGet(rangeLimit, int((rtmp3-rtmp4)>>shift))
	}
}
