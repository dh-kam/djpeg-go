package huff

// Accurate integer IDCT implementation constants and helpers.
// Ported from jidctint.c (IJG libjpeg 9f).

const (
	islowConstBits = 13
	islowPass1Bits = 2
)

var (
	// ONE is 1.0 in fixed-point (1 << CONST_BITS).
	islowOne = 1 << islowConstBits

	// Pre-calculated constants for CONST_BITS = 13.
	// Formula: FIX(x) = int(x * 8192 + 0.5)

	fix0_275899379 int32 = 2260  // FIX(0.275899379)
	fix0_298631336 int32 = 2446  // FIX(0.298631336)
	fix0_344136286 int32 = 2818  // FIX(0.344136286)
	fix0_390180644 int32 = 3196  // FIX(0.390180644)
	fix0_410524528 int32 = 3363  // FIX(0.410524528)
	fix0_509795579 int32 = 4176  // FIX(0.509795579)
	fix0_541196100 int32 = 4433  // FIX(0.541196100)
	fix0_601344887 int32 = 4926  // FIX(0.601344887)
	fix0_666655658 int32 = 5461  // FIX(0.666655658)
	fix0_714136286 int32 = 5850  // FIX(0.714136286)
	fix0_765366865 int32 = 6270  // FIX(0.765366865)
	fix0_766367282 int32 = 6278  // FIX(0.766367282)
	fix0_897167586 int32 = 7349  // FIX(0.897167586)
	fix0_899976223 int32 = 7373  // FIX(0.899976223)
	fix1_065388962 int32 = 8728  // FIX(1.065388962)
	fix1_093201867 int32 = 8956  // FIX(1.093201867)
	fix1_125726048 int32 = 9222  // FIX(1.125726048)
	fix1_175875602 int32 = 9633  // FIX(1.175875602)
	fix1_247225013 int32 = 10217 // FIX(1.247225013)
	fix1_306562965 int32 = 10703 // FIX(1.306562965)
	fix1_353318001 int32 = 11086 // FIX(1.353318001)
	fix1_387039845 int32 = 11363 // FIX(1.387039845)
	fix1_407403738 int32 = 11530 // FIX(1.407403738)
	fix1_501321110 int32 = 12299 // FIX(1.501321110)
	fix1_835730603 int32 = 15038 // FIX(1.835730603)
	fix1_847759065 int32 = 15137 // FIX(1.847759065)
	fix1_961570560 int32 = 16069 // FIX(1.961570560)
	fix1_971951411 int32 = 16154 // FIX(1.971951411)
	fix2_053119869 int32 = 16819 // FIX(2.053119869)
	fix2_286341144 int32 = 18730 // FIX(2.286341144)
	fix2_562915447 int32 = 20995 // FIX(2.562915447)
	fix3_072711026 int32 = 25172 // FIX(3.072711026)
	fix3_141271809 int32 = 25733 // FIX(3.141271809)

	fix0_138617169 int32 = 1136 // FIX(0.138617169)
	fix0_071888074 int32 = 589  // FIX(0.071888074)
)

// rangeLimitGet is a helper to access the range limit table safely.
func rangeLimitGet(rl *RangeLimitTable, val int) JSAMPLE {
	return rl[val&RangeMask]
}

// IDCTISlowImpl performs dequantization and inverse DCT on one block of
// coefficients using the accurate integer (ISLOW) method.
func IDCTISlowImpl(coefBlock []JCOEF, quantptr *ISlowMultTable,
	outputBuf []BlockRow, outputCol int, rangeLimit *RangeLimitTable) {

	var workspace [DCTSize2]int64

	const (
		shift1 = islowConstBits - islowPass1Bits
		shift3 = islowConstBits + islowPass1Bits + 3
	)

	// Pass 1: process columns from input, store into work array.
	for ctr := 0; ctr < DCTSize; ctr++ {
		inptr := ctr
		wsptr := ctr

		// shortcut
		if coefBlock[inptr+DCTSize*1] == 0 &&
			coefBlock[inptr+DCTSize*2] == 0 &&
			coefBlock[inptr+DCTSize*3] == 0 &&
			coefBlock[inptr+DCTSize*4] == 0 &&
			coefBlock[inptr+DCTSize*5] == 0 &&
			coefBlock[inptr+DCTSize*6] == 0 &&
			coefBlock[inptr+DCTSize*7] == 0 {
			dcval := int64(coefBlock[inptr]) * int64(quantptr[ctr]) << islowPass1Bits
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
		z2 := int64(coefBlock[inptr]) * int64(quantptr[ctr])
		z3 := int64(coefBlock[inptr+DCTSize*4]) * int64(quantptr[DCTSize*4+ctr])
		z2 <<= islowConstBits
		z3 <<= islowConstBits
		z2 += 1 << (islowConstBits - islowPass1Bits - 1)
		tmp0 := z2 + z3
		tmp1 := z2 - z3
		z2 = int64(coefBlock[inptr+DCTSize*2]) * int64(quantptr[DCTSize*2+ctr])
		z3 = int64(coefBlock[inptr+DCTSize*6]) * int64(quantptr[DCTSize*6+ctr])
		z1 := (z2 + z3) * int64(fix0_541196100)
		tmp2 := z1 + z2*int64(fix0_765366865)
		tmp3 := z1 - z3*int64(fix1_847759065)
		tmp10 := tmp0 + tmp2
		tmp13 := tmp0 - tmp2
		tmp11 := tmp1 + tmp3
		tmp12 := tmp1 - tmp3

		// Odd part
		t0 := int64(coefBlock[inptr+DCTSize*7]) * int64(quantptr[DCTSize*7+ctr])
		t1 := int64(coefBlock[inptr+DCTSize*5]) * int64(quantptr[DCTSize*5+ctr])
		t2 := int64(coefBlock[inptr+DCTSize*3]) * int64(quantptr[DCTSize*3+ctr])
		t3 := int64(coefBlock[inptr+DCTSize*1]) * int64(quantptr[DCTSize*1+ctr])
		z2 = t0 + t2
		z3 = t1 + t3
		z1 = (z2 + z3) * int64(fix1_175875602)
		z2 = z2 * int64(-fix1_961570560)
		z3 = z3 * int64(-fix0_390180644)
		z2 += z1
		z3 += z1
		z1 = (t0 + t3) * int64(-fix0_899976223)
		t0 = t0*int64(fix0_298631336) + z1 + z2
		t3 = t3*int64(fix1_501321110) + z1 + z3
		z1 = (t1 + t2) * int64(-fix2_562915447)
		t1 = t1*int64(fix2_053119869) + z1 + z3
		t2 = t2*int64(fix3_072711026) + z1 + z2

		workspace[wsptr] = (tmp10 + t3) >> shift1
		workspace[wsptr+DCTSize*7] = (tmp10 - t3) >> shift1
		workspace[wsptr+DCTSize] = (tmp11 + t2) >> shift1
		workspace[wsptr+DCTSize*6] = (tmp11 - t2) >> shift1
		workspace[wsptr+DCTSize*2] = (tmp12 + t1) >> shift1
		workspace[wsptr+DCTSize*5] = (tmp12 - t1) >> shift1
		workspace[wsptr+DCTSize*3] = (tmp13 + t0) >> shift1
		workspace[wsptr+DCTSize*4] = (tmp13 - t0) >> shift1
	}

	// Pass 2: process rows from work array, store into output array.
	for ctr := 0; ctr < DCTSize; ctr++ {
		outptr := outputBuf[ctr][outputCol:]
		wsptr := ctr * DCTSize

		// Add range center and fudge factor.
		z2 := workspace[wsptr] + (int64(RangeCenter) << 5) + (1 << 4)

		if workspace[wsptr+1] == 0 && workspace[wsptr+2] == 0 &&
			workspace[wsptr+3] == 0 && workspace[wsptr+4] == 0 &&
			workspace[wsptr+5] == 0 && workspace[wsptr+6] == 0 &&
			workspace[wsptr+7] == 0 {
			dcval := rangeLimitGet(rangeLimit, int(z2>>5))
			outptr[0] = dcval
			outptr[1] = dcval
			outptr[2] = dcval
			outptr[3] = dcval
			outptr[4] = dcval
			outptr[5] = dcval
			outptr[6] = dcval
			outptr[7] = dcval
			continue
		}

		// Even part
		z2 <<= islowConstBits
		z3 := workspace[wsptr+4] << islowConstBits
		tmp0 := z2 + z3
		tmp1 := z2 - z3
		z2 = workspace[wsptr+2]
		z3 = workspace[wsptr+6]
		z1 := (z2 + z3) * int64(fix0_541196100)
		tmp2 := z1 + z2*int64(fix0_765366865)
		tmp3 := z1 - z3*int64(fix1_847759065)
		tmp10 := tmp0 + tmp2
		tmp13 := tmp0 - tmp2
		tmp11 := tmp1 + tmp3
		tmp12 := tmp1 - tmp3

		// Odd part
		t0 := workspace[wsptr+7]
		t1 := workspace[wsptr+5]
		t2 := workspace[wsptr+3]
		t3 := workspace[wsptr+1]
		z2 = t0 + t2
		z3 = t1 + t3
		z1 = (z2 + z3) * int64(fix1_175875602)
		z2 = z2 * int64(-fix1_961570560)
		z3 = z3 * int64(-fix0_390180644)
		z2 += z1
		z3 += z1
		z1 = (t0 + t3) * int64(-fix0_899976223)
		t0 = t0*int64(fix0_298631336) + z1 + z2
		t3 = t3*int64(fix1_501321110) + z1 + z3
		z1 = (t1 + t2) * int64(-fix2_562915447)
		t1 = t1*int64(fix2_053119869) + z1 + z3
		t2 = t2*int64(fix3_072711026) + z1 + z2

		outptr[0] = rangeLimitGet(rangeLimit, int((tmp10+t3)>>shift3))
		outptr[7] = rangeLimitGet(rangeLimit, int((tmp10-t3)>>shift3))
		outptr[1] = rangeLimitGet(rangeLimit, int((tmp11+t2)>>shift3))
		outptr[6] = rangeLimitGet(rangeLimit, int((tmp11-t2)>>shift3))
		outptr[2] = rangeLimitGet(rangeLimit, int((tmp12+t1)>>shift3))
		outptr[5] = rangeLimitGet(rangeLimit, int((tmp12-t1)>>shift3))
		outptr[3] = rangeLimitGet(rangeLimit, int((tmp13+t0)>>shift3))
		outptr[4] = rangeLimitGet(rangeLimit, int((tmp13-t0)>>shift3))
	}
}
