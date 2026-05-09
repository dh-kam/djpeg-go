package huff

import "math"

const (
	fix16_0_071888074 int64 = 589
	fix16_0_138617169 int64 = 1136
	fix16_0_275899379 int64 = 2260
	fix16_0_410524528 int64 = 3363
	fix16_0_509795579 int64 = 4176
	fix16_0_541196100 int64 = 4433
	fix16_0_601344887 int64 = 4926
	fix16_0_666655658 int64 = 5461
	fix16_0_766367282 int64 = 6278
	fix16_0_897167586 int64 = 7350
	fix16_0_899976223 int64 = 7373
	fix16_1_065388962 int64 = 8728
	fix16_1_093201867 int64 = 8956
	fix16_1_125726048 int64 = 9222
	fix16_1_247225013 int64 = 10217
	fix16_1_306562965 int64 = 10703
	fix16_1_353318001 int64 = 11086
	fix16_1_387039845 int64 = 11363
	fix16_1_407403738 int64 = 11529
	fix16_1_835730603 int64 = 15038
	fix16_1_971951411 int64 = 16154
	fix16_2_286341144 int64 = 18730
	fix16_2_562915447 int64 = 20995
	fix16_3_141271809 int64 = 25733
)

// IDCTScaledISlowImpl performs a generic ISLOW dequantization plus scaled
// inverse DCT for block sizes that do not yet have a dedicated IJG kernel.
func IDCTScaledISlowImpl(coefBlock []JCOEF, quantptr *ISlowMultTable, outputBuf []BlockRow, outputCol, outputRows, outputCols int, rangeLimit *RangeLimitTable) {
	if outputRows <= 0 || outputCols <= 0 {
		return
	}
	var dequant [DCTSize2]float64
	for i := 0; i < DCTSize2; i++ {
		dequant[i] = float64(coefBlock[i]) * float64(quantptr[i])
	}

	const invSqrt2 = 0.7071067811865476
	rowDenom := 2 * float64(outputRows)
	colDenom := 2 * float64(outputCols)
	for outY := 0; outY < outputRows; outY++ {
		outptr := outputBuf[outY]
		for outX := 0; outX < outputCols; outX++ {
			sum := 0.0
			for v := 0; v < DCTSize; v++ {
				cv := 1.0
				if v == 0 {
					cv = invSqrt2
				}
				cosY := math.Cos(float64(2*outY+1) * float64(v) * math.Pi / rowDenom)
				for u := 0; u < DCTSize; u++ {
					cu := 1.0
					if u == 0 {
						cu = invSqrt2
					}
					cosX := math.Cos(float64(2*outX+1) * float64(u) * math.Pi / colDenom)
					sum += cu * cv * dequant[v*DCTSize+u] * cosX * cosY
				}
			}
			outptr[outputCol+outX] = rangeLimitGet(rangeLimit, int(math.Round(sum/4))+RangeCenter)
		}
	}
}

// IDCT16x16Impl performs IJG's 16x16 scaled ISLOW IDCT.
func IDCT16x16Impl(coefBlock []JCOEF, quantptr *ISlowMultTable, outputBuf []BlockRow, outputCol int, rangeLimit *RangeLimitTable) {
	var workspace [8 * 16]int64

	const (
		pass1Shift = islowConstBits - islowPass1Bits
		pass2Shift = islowConstBits + islowPass1Bits + 3
	)

	for ctr := 0; ctr < 8; ctr++ {
		inptr := ctr
		wsptr := ctr

		tmp0 := int64(coefBlock[inptr+DCTSize*0]) * int64(quantptr[DCTSize*0+ctr])
		tmp0 <<= islowConstBits
		tmp0 += int64(1) << (islowConstBits - islowPass1Bits - 1)

		z1 := int64(coefBlock[inptr+DCTSize*4]) * int64(quantptr[DCTSize*4+ctr])
		tmp1 := z1 * fix16_1_306562965
		tmp2 := z1 * fix16_0_541196100

		tmp10 := tmp0 + tmp1
		tmp11 := tmp0 - tmp1
		tmp12 := tmp0 + tmp2
		tmp13 := tmp0 - tmp2

		z1 = int64(coefBlock[inptr+DCTSize*2]) * int64(quantptr[DCTSize*2+ctr])
		z2 := int64(coefBlock[inptr+DCTSize*6]) * int64(quantptr[DCTSize*6+ctr])
		z3 := z1 - z2
		z4 := z3 * fix16_0_275899379
		z3 = z3 * fix16_1_387039845

		tmp0 = z3 + z2*fix16_2_562915447
		tmp1 = z4 + z1*fix16_0_899976223
		tmp2 = z3 - z1*fix16_0_601344887
		tmp3 := z4 - z2*fix16_0_509795579

		tmp20 := tmp10 + tmp0
		tmp27 := tmp10 - tmp0
		tmp21 := tmp12 + tmp1
		tmp26 := tmp12 - tmp1
		tmp22 := tmp13 + tmp2
		tmp25 := tmp13 - tmp2
		tmp23 := tmp11 + tmp3
		tmp24 := tmp11 - tmp3

		z1 = int64(coefBlock[inptr+DCTSize*1]) * int64(quantptr[DCTSize*1+ctr])
		z2 = int64(coefBlock[inptr+DCTSize*3]) * int64(quantptr[DCTSize*3+ctr])
		z3 = int64(coefBlock[inptr+DCTSize*5]) * int64(quantptr[DCTSize*5+ctr])
		z4 = int64(coefBlock[inptr+DCTSize*7]) * int64(quantptr[DCTSize*7+ctr])

		tmp11 = z1 + z3

		tmp1 = (z1 + z2) * fix16_1_353318001
		tmp2 = tmp11 * fix16_1_247225013
		tmp3 = (z1 + z4) * fix16_1_093201867
		tmp10 = (z1 - z4) * fix16_0_897167586
		tmp11 = tmp11 * fix16_0_666655658
		tmp12 = (z1 - z2) * fix16_0_410524528
		tmp0 = tmp1 + tmp2 + tmp3 - z1*fix16_2_286341144
		tmp13 = tmp10 + tmp11 + tmp12 - z1*fix16_1_835730603
		z1 = (z2 + z3) * fix16_0_138617169
		tmp1 += z1 + z2*fix16_0_071888074
		tmp2 += z1 - z3*fix16_1_125726048
		z1 = (z3 - z2) * fix16_1_407403738
		tmp11 += z1 - z3*fix16_0_766367282
		tmp12 += z1 + z2*fix16_1_971951411
		z2 += z4
		z1 = z2 * (-fix16_0_666655658)
		tmp1 += z1
		tmp3 += z1 + z4*fix16_1_065388962
		z2 = z2 * (-fix16_1_247225013)
		tmp10 += z2 + z4*fix16_3_141271809
		tmp12 += z2
		z2 = (z3 + z4) * (-fix16_1_353318001)
		tmp2 += z2
		tmp3 += z2
		z2 = (z4 - z3) * fix16_0_410524528
		tmp10 += z2
		tmp11 += z2

		workspace[wsptr+8*0] = (tmp20 + tmp0) >> pass1Shift
		workspace[wsptr+8*15] = (tmp20 - tmp0) >> pass1Shift
		workspace[wsptr+8*1] = (tmp21 + tmp1) >> pass1Shift
		workspace[wsptr+8*14] = (tmp21 - tmp1) >> pass1Shift
		workspace[wsptr+8*2] = (tmp22 + tmp2) >> pass1Shift
		workspace[wsptr+8*13] = (tmp22 - tmp2) >> pass1Shift
		workspace[wsptr+8*3] = (tmp23 + tmp3) >> pass1Shift
		workspace[wsptr+8*12] = (tmp23 - tmp3) >> pass1Shift
		workspace[wsptr+8*4] = (tmp24 + tmp10) >> pass1Shift
		workspace[wsptr+8*11] = (tmp24 - tmp10) >> pass1Shift
		workspace[wsptr+8*5] = (tmp25 + tmp11) >> pass1Shift
		workspace[wsptr+8*10] = (tmp25 - tmp11) >> pass1Shift
		workspace[wsptr+8*6] = (tmp26 + tmp12) >> pass1Shift
		workspace[wsptr+8*9] = (tmp26 - tmp12) >> pass1Shift
		workspace[wsptr+8*7] = (tmp27 + tmp13) >> pass1Shift
		workspace[wsptr+8*8] = (tmp27 - tmp13) >> pass1Shift
	}

	rl := (*[5 * 256]JSAMPLE)(rangeLimit)

	for ctr := 0; ctr < 16; ctr++ {
		wsptr := ctr * 8
		outptr := outputBuf[ctr]
		outIdx := outputCol

		tmp0 := workspace[wsptr] +
			((int64(RangeCenter) << (islowPass1Bits + 3)) +
				(int64(1) << (islowPass1Bits + 2)))
		tmp0 <<= islowConstBits

		z1 := workspace[wsptr+4]
		tmp1 := z1 * fix16_1_306562965
		tmp2 := z1 * fix16_0_541196100

		tmp10 := tmp0 + tmp1
		tmp11 := tmp0 - tmp1
		tmp12 := tmp0 + tmp2
		tmp13 := tmp0 - tmp2

		z1 = workspace[wsptr+2]
		z2 := workspace[wsptr+6]
		z3 := z1 - z2
		z4 := z3 * fix16_0_275899379
		z3 = z3 * fix16_1_387039845

		tmp0 = z3 + z2*fix16_2_562915447
		tmp1 = z4 + z1*fix16_0_899976223
		tmp2 = z3 - z1*fix16_0_601344887
		tmp3 := z4 - z2*fix16_0_509795579

		tmp20 := tmp10 + tmp0
		tmp27 := tmp10 - tmp0
		tmp21 := tmp12 + tmp1
		tmp26 := tmp12 - tmp1
		tmp22 := tmp13 + tmp2
		tmp25 := tmp13 - tmp2
		tmp23 := tmp11 + tmp3
		tmp24 := tmp11 - tmp3

		z1 = workspace[wsptr+1]
		z2 = workspace[wsptr+3]
		z3 = workspace[wsptr+5]
		z4 = workspace[wsptr+7]

		tmp11 = z1 + z3

		tmp1 = (z1 + z2) * fix16_1_353318001
		tmp2 = tmp11 * fix16_1_247225013
		tmp3 = (z1 + z4) * fix16_1_093201867
		tmp10 = (z1 - z4) * fix16_0_897167586
		tmp11 = tmp11 * fix16_0_666655658
		tmp12 = (z1 - z2) * fix16_0_410524528
		tmp0 = tmp1 + tmp2 + tmp3 - z1*fix16_2_286341144
		tmp13 = tmp10 + tmp11 + tmp12 - z1*fix16_1_835730603
		z1 = (z2 + z3) * fix16_0_138617169
		tmp1 += z1 + z2*fix16_0_071888074
		tmp2 += z1 - z3*fix16_1_125726048
		z1 = (z3 - z2) * fix16_1_407403738
		tmp11 += z1 - z3*fix16_0_766367282
		tmp12 += z1 + z2*fix16_1_971951411
		z2 += z4
		z1 = z2 * (-fix16_0_666655658)
		tmp1 += z1
		tmp3 += z1 + z4*fix16_1_065388962
		z2 = z2 * (-fix16_1_247225013)
		tmp10 += z2 + z4*fix16_3_141271809
		tmp12 += z2
		z2 = (z3 + z4) * (-fix16_1_353318001)
		tmp2 += z2
		tmp3 += z2
		z2 = (z4 - z3) * fix16_0_410524528
		tmp10 += z2
		tmp11 += z2

		outptr[outIdx+0] = rl[((tmp20+tmp0)>>pass2Shift)&RangeMask]
		outptr[outIdx+15] = rl[((tmp20-tmp0)>>pass2Shift)&RangeMask]
		outptr[outIdx+1] = rl[((tmp21+tmp1)>>pass2Shift)&RangeMask]
		outptr[outIdx+14] = rl[((tmp21-tmp1)>>pass2Shift)&RangeMask]
		outptr[outIdx+2] = rl[((tmp22+tmp2)>>pass2Shift)&RangeMask]
		outptr[outIdx+13] = rl[((tmp22-tmp2)>>pass2Shift)&RangeMask]
		outptr[outIdx+3] = rl[((tmp23+tmp3)>>pass2Shift)&RangeMask]
		outptr[outIdx+12] = rl[((tmp23-tmp3)>>pass2Shift)&RangeMask]
		outptr[outIdx+4] = rl[((tmp24+tmp10)>>pass2Shift)&RangeMask]
		outptr[outIdx+11] = rl[((tmp24-tmp10)>>pass2Shift)&RangeMask]
		outptr[outIdx+5] = rl[((tmp25+tmp11)>>pass2Shift)&RangeMask]
		outptr[outIdx+10] = rl[((tmp25-tmp11)>>pass2Shift)&RangeMask]
		outptr[outIdx+6] = rl[((tmp26+tmp12)>>pass2Shift)&RangeMask]
		outptr[outIdx+9] = rl[((tmp26-tmp12)>>pass2Shift)&RangeMask]
		outptr[outIdx+7] = rl[((tmp27+tmp13)>>pass2Shift)&RangeMask]
		outptr[outIdx+8] = rl[((tmp27-tmp13)>>pass2Shift)&RangeMask]
	}
}
