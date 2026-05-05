package huff

// Floating-point IDCT implementation.
// Ported from jidctflt.c (IJG libjpeg 9f).
//
// This implementation should be more accurate than either of the integer
// IDCT implementations. However, it may not give the same results on all
// machines because of differences in roundoff behavior.
//
// Based on the Arai, Agui, and Nakajima (AA&N) algorithm for scaled DCT.
// With floating-point arithmetic, the scaled quantization values are exact,
// so the AA&N method's primary disadvantage (imprecise fixed-point scaling)
// is avoided.

// IDCTFloatImpl performs dequantization and inverse DCT on one block
// of coefficients using the floating-point method.
// Ported from jpeg_idct_float in jidctflt.c.
func IDCTFloatImpl(coefBlock []JCOEF, quantptr *FloatMultTable,
	outputBuf []BlockRow, outputCol int, rangeLimit *RangeLimitTable) {

	var workspace [DCTSize2]float64

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
			dcval := float64(coefBlock[inptr+DCTSize*0]) * quantptr[DCTSize*0+ctr]

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
		tmp0 := float64(coefBlock[inptr+DCTSize*0]) * quantptr[DCTSize*0+ctr]
		tmp1 := float64(coefBlock[inptr+DCTSize*2]) * quantptr[DCTSize*2+ctr]
		tmp2 := float64(coefBlock[inptr+DCTSize*4]) * quantptr[DCTSize*4+ctr]
		tmp3 := float64(coefBlock[inptr+DCTSize*6]) * quantptr[DCTSize*6+ctr]

		tmp10 := tmp0 + tmp2 // phase 3
		tmp11 := tmp0 - tmp2

		tmp13 := tmp1 + tmp3 // phases 5-3
		tmp12 := (tmp1 - tmp3) * 1.414213562 - tmp13 // 2*c4

		tmp0 = tmp10 + tmp13 // phase 2
		tmp3 = tmp10 - tmp13
		tmp1 = tmp11 + tmp12
		tmp2 = tmp11 - tmp12

		// Odd part
		tmp4 := float64(coefBlock[inptr+DCTSize*1]) * quantptr[DCTSize*1+ctr]
		tmp5 := float64(coefBlock[inptr+DCTSize*3]) * quantptr[DCTSize*3+ctr]
		tmp6 := float64(coefBlock[inptr+DCTSize*5]) * quantptr[DCTSize*5+ctr]
		tmp7 := float64(coefBlock[inptr+DCTSize*7]) * quantptr[DCTSize*7+ctr]

		z13 := tmp6 + tmp5 // phase 6
		z10 := tmp6 - tmp5
		z11 := tmp4 + tmp7
		z12 := tmp4 - tmp7

		tmp7 = z11 + z13 // phase 5
		tmp11 = (z11 - z13) * 1.414213562 // 2*c4

		z5 := (z10 + z12) * 1.847759065           // 2*c2
		tmp10 = z5 - z12*1.082392200               // 2*(c2-c6)
		tmp12 = z5 - z10*2.613125930               // 2*(c2+c6)

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
	// The range_limit table handles the level shift (CENTERJSAMPLE)
	// internally, so we do NOT add RangeCenter explicitly.
	// C code uses: range_limit[(int)(value) & RANGE_MASK]
	// where range_limit is already offset by CENTERJSAMPLE.
	for ctr := 0; ctr < DCTSize; ctr++ {
		wsptr := ctr * DCTSize
		outptr := outputBuf[ctr]
		outIdx := outputCol

		// Even part
		ftmp10 := workspace[wsptr+0] + workspace[wsptr+4]
		ftmp11 := workspace[wsptr+0] - workspace[wsptr+4]

		ftmp13 := workspace[wsptr+2] + workspace[wsptr+6]
		ftmp12 := (workspace[wsptr+2]-workspace[wsptr+6])*1.414213562 - ftmp13 // 2*c4

		ftmp0 := ftmp10 + ftmp13
		ftmp3 := ftmp10 - ftmp13
		ftmp1 := ftmp11 + ftmp12
		ftmp2 := ftmp11 - ftmp12

		// Odd part
		fz13 := workspace[wsptr+5] + workspace[wsptr+3]
		fz10 := workspace[wsptr+5] - workspace[wsptr+3]
		fz11 := workspace[wsptr+1] + workspace[wsptr+7]
		fz12 := workspace[wsptr+1] - workspace[wsptr+7]

		ftmp7 := fz11 + fz13 // phase 5
		ftmp11 = (fz11 - fz13) * 1.414213562 // 2*c4

		fz5 := (fz10 + fz12) * 1.847759065            // 2*c2
		ftmp10 = fz5 - fz12*1.082392200               // 2*(c2-c6)
		ftmp12 = fz5 - fz10*2.613125930               // 2*(c2+c6)

		ftmp6 := ftmp12 - ftmp7 // phase 2
		ftmp5 := ftmp11 - ftmp6
		ftmp4 := ftmp10 - ftmp5

		// Final output stage: float->int conversion and range-limit
		// C code: range_limit[(int)(ftmp0 + ftmp7) & RANGE_MASK]
		// The range_limit table handles the level shift.
		// We add 0.5 for rounding (matching the C DESCALE behavior).
		outptr[outIdx+0] = rangeLimitGet(rangeLimit, int(ftmp0+ftmp7+0.5))
		outptr[outIdx+7] = rangeLimitGet(rangeLimit, int(ftmp0-ftmp7+0.5))
		outptr[outIdx+1] = rangeLimitGet(rangeLimit, int(ftmp1+ftmp6+0.5))
		outptr[outIdx+6] = rangeLimitGet(rangeLimit, int(ftmp1-ftmp6+0.5))
		outptr[outIdx+2] = rangeLimitGet(rangeLimit, int(ftmp2+ftmp5+0.5))
		outptr[outIdx+5] = rangeLimitGet(rangeLimit, int(ftmp2-ftmp5+0.5))
		outptr[outIdx+3] = rangeLimitGet(rangeLimit, int(ftmp3+ftmp4+0.5))
		outptr[outIdx+4] = rangeLimitGet(rangeLimit, int(ftmp3-ftmp4+0.5))
	}
}
