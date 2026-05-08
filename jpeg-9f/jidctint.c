#include "jinclude.h"
#include "jpeglib.h"
#include "jdct.h"

#define CONST_BITS  13
#define PASS1_BITS  2

#define ONE	((INT32) 1)
#define CONST_SCALE (ONE << CONST_BITS)
#define FIX(x)	((INT32) ((x) * CONST_SCALE + 0.5))

#define DESCALE(x,n)  RIGHT_SHIFT((x) + (ONE << ((n)-1)), n)

#define MULTIPLY(var,const)  ((var) * (const))
#define DEQUANTIZE(coef,quantval)  (((ISLOW_MULT_TYPE) (coef)) * (quantval))

#define FIX_0_298631336  FIX(0.298631336)
#define FIX_0_390180644  FIX(0.390180644)
#define FIX_0_541196100  FIX(0.541196100)
#define FIX_0_765366865  FIX(0.765366865)
#define FIX_0_899976223  FIX(0.899976223)
#define FIX_1_175875602  FIX(1.175875602)
#define FIX_1_501321110  FIX(1.501321110)
#define FIX_1_847759065  FIX(1.847759065)
#define FIX_1_961570560  FIX(1.961570560)
#define FIX_2_053119869  FIX(2.053119869)
#define FIX_2_562915447  FIX(2.562915447)
#define FIX_3_072711026  FIX(3.072711026)

GLOBAL(void)
jpeg_idct_islow (j_decompress_ptr cinfo, jpeg_component_info * compptr,
		 JCOEFPTR coef_block,
		 JSAMPARRAY output_buf, JDIMENSION output_col)
{
  INT32 tmp0, tmp1, tmp2, tmp3, tmp10, tmp11, tmp12, tmp13;
  INT32 z1, z2, z3;
  JCOEFPTR inptr;
  ISLOW_MULT_TYPE * quantptr;
  int * wsptr;
  JSAMPROW outptr;
  JSAMPLE *range_limit = IDCT_range_limit(cinfo);
  int ctr;
  int workspace[DCTSIZE2];
  SHIFT_TEMPS

  inptr = coef_block;
  quantptr = (ISLOW_MULT_TYPE *) compptr->dct_table;
  wsptr = workspace;
  for (ctr = DCTSIZE; ctr > 0; ctr--) {
    if (inptr[DCTSIZE*1] == 0 && inptr[DCTSIZE*2] == 0 &&
	inptr[DCTSIZE*3] == 0 && inptr[DCTSIZE*4] == 0 &&
	inptr[DCTSIZE*5] == 0 && inptr[DCTSIZE*6] == 0 &&
	inptr[DCTSIZE*7] == 0) {
      tmp0 = DEQUANTIZE(inptr[DCTSIZE*0], quantptr[DCTSIZE*0]) << PASS1_BITS;
      wsptr[DCTSIZE*0] = (int) tmp0;
      wsptr[DCTSIZE*1] = (int) tmp0;
      wsptr[DCTSIZE*2] = (int) tmp0;
      wsptr[DCTSIZE*3] = (int) tmp0;
      wsptr[DCTSIZE*4] = (int) tmp0;
      wsptr[DCTSIZE*5] = (int) tmp0;
      wsptr[DCTSIZE*6] = (int) tmp0;
      wsptr[DCTSIZE*7] = (int) tmp0;
      inptr++; quantptr++; wsptr++;
      continue;
    }
    z2 = DEQUANTIZE(inptr[DCTSIZE*0], quantptr[DCTSIZE*0]);
    z3 = DEQUANTIZE(inptr[DCTSIZE*4], quantptr[DCTSIZE*4]);
    z2 <<= CONST_BITS;
    z3 <<= CONST_BITS;
    z2 += ONE << (CONST_BITS-PASS1_BITS-1);
    tmp0 = z2 + z3;
    tmp1 = z2 - z3;
    z2 = DEQUANTIZE(inptr[DCTSIZE*2], quantptr[DCTSIZE*2]);
    z3 = DEQUANTIZE(inptr[DCTSIZE*6], quantptr[DCTSIZE*6]);
    z1 = MULTIPLY(z2 + z3, FIX_0_541196100);
    tmp2 = z1 + MULTIPLY(z2, FIX_0_765366865);
    tmp3 = z1 - MULTIPLY(z3, FIX_1_847759065);
    tmp10 = tmp0 + tmp2;
    tmp13 = tmp0 - tmp2;
    tmp11 = tmp1 + tmp3;
    tmp12 = tmp1 - tmp3;
    tmp0 = DEQUANTIZE(inptr[DCTSIZE*7], quantptr[DCTSIZE*7]);
    tmp1 = DEQUANTIZE(inptr[DCTSIZE*5], quantptr[DCTSIZE*5]);
    tmp2 = DEQUANTIZE(inptr[DCTSIZE*3], quantptr[DCTSIZE*3]);
    tmp3 = DEQUANTIZE(inptr[DCTSIZE*1], quantptr[DCTSIZE*1]);
    z2 = tmp0 + tmp2;
    z3 = tmp1 + tmp3;
    z1 = MULTIPLY(z2 + z3, FIX_1_175875602);
    z2 = MULTIPLY(z2, - FIX_1_961570560);
    z3 = MULTIPLY(z3, - FIX_0_390180644);
    z2 += z1;
    z3 += z1;
    z1 = MULTIPLY(tmp0 + tmp3, - FIX_0_899976223);
    tmp0 = MULTIPLY(tmp0, FIX_0_298631336);
    tmp3 = MULTIPLY(tmp3, FIX_1_501321110);
    tmp0 += z1 + z2;
    tmp3 += z1 + z3;
    z1 = MULTIPLY(tmp1 + tmp2, - FIX_2_562915447);
    tmp1 = MULTIPLY(tmp1, FIX_2_053119869);
    tmp2 = MULTIPLY(tmp2, FIX_3_072711026);
    tmp1 += z1 + z3;
    tmp2 += z1 + z2;
    wsptr[DCTSIZE*0] = (int) RIGHT_SHIFT(tmp10 + tmp3, CONST_BITS-PASS1_BITS);
    wsptr[DCTSIZE*7] = (int) RIGHT_SHIFT(tmp10 - tmp3, CONST_BITS-PASS1_BITS);
    wsptr[DCTSIZE*1] = (int) RIGHT_SHIFT(tmp11 + tmp2, CONST_BITS-PASS1_BITS);
    wsptr[DCTSIZE*6] = (int) RIGHT_SHIFT(tmp11 - tmp2, CONST_BITS-PASS1_BITS);
    wsptr[DCTSIZE*2] = (int) RIGHT_SHIFT(tmp12 + tmp1, CONST_BITS-PASS1_BITS);
    wsptr[DCTSIZE*5] = (int) RIGHT_SHIFT(tmp12 - tmp1, CONST_BITS-PASS1_BITS);
    wsptr[DCTSIZE*3] = (int) RIGHT_SHIFT(tmp13 + tmp0, CONST_BITS-PASS1_BITS);
    wsptr[DCTSIZE*4] = (int) RIGHT_SHIFT(tmp13 - tmp0, CONST_BITS-PASS1_BITS);
    inptr++; quantptr++; wsptr++;
  }
  wsptr = workspace;
  for (ctr = 0; ctr < DCTSIZE; ctr++) {
    outptr = output_buf[ctr] + output_col;
    if (wsptr[1] == 0 && wsptr[2] == 0 && wsptr[3] == 0 && wsptr[4] == 0 &&
	wsptr[5] == 0 && wsptr[6] == 0 && wsptr[7] == 0) {
      JSAMPLE dcval = range_limit[(int) DESCALE((INT32) wsptr[0], PASS1_BITS+3) & RANGE_MASK];
      outptr[0] = dcval; outptr[1] = dcval; outptr[2] = dcval; outptr[3] = dcval;
      outptr[4] = dcval; outptr[5] = dcval; outptr[6] = dcval; outptr[7] = dcval;
      wsptr += DCTSIZE; continue;
    }
    z2 = (INT32) wsptr[0] + ((((INT32) RANGE_CENTER) << (PASS1_BITS+3)) + (ONE << (PASS1_BITS+2)));
    z2 <<= CONST_BITS;
    z3 = (INT32) wsptr[4]; z3 <<= CONST_BITS;
    tmp0 = z2 + z3; tmp1 = z2 - z3;
    z2 = (INT32) wsptr[2]; z3 = (INT32) wsptr[6];
    z1 = MULTIPLY(z2 + z3, FIX_0_541196100);
    tmp2 = z1 + MULTIPLY(z2, FIX_0_765366865);
    tmp3 = z1 - MULTIPLY(z3, FIX_1_847759065);
    tmp10 = tmp0 + tmp2; tmp13 = tmp0 - tmp2;
    tmp11 = tmp1 + tmp3; tmp12 = tmp1 - tmp3;
    tmp0 = (INT32) wsptr[7]; tmp1 = (INT32) wsptr[5];
    tmp2 = (INT32) wsptr[3]; tmp3 = (INT32) wsptr[1];
    z2 = tmp0 + tmp2; z3 = tmp1 + tmp3;
    z1 = MULTIPLY(z2 + z3, FIX_1_175875602);
    z2 = MULTIPLY(z2, - FIX_1_961570560);
    z3 = MULTIPLY(z3, - FIX_0_390180644);
    z2 += z1; z3 += z1;
    z1 = MULTIPLY(tmp0 + tmp3, - FIX_0_899976223);
    tmp0 = MULTIPLY(tmp0, FIX_0_298631336);
    tmp3 = MULTIPLY(tmp3, FIX_1_501321110);
    tmp0 += z1 + z2; tmp3 += z1 + z3;
    z1 = MULTIPLY(tmp1 + tmp2, - FIX_2_562915447);
    tmp1 = MULTIPLY(tmp1, FIX_2_053119869);
    tmp2 = MULTIPLY(tmp2, FIX_3_072711026);
    tmp1 += z1 + z3; tmp2 += z1 + z2;
    outptr[0] = range_limit[(int) RIGHT_SHIFT(tmp10 + tmp3, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    outptr[7] = range_limit[(int) RIGHT_SHIFT(tmp10 - tmp3, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    outptr[1] = range_limit[(int) RIGHT_SHIFT(tmp11 + tmp2, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    outptr[6] = range_limit[(int) RIGHT_SHIFT(tmp11 - tmp2, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    outptr[2] = range_limit[(int) RIGHT_SHIFT(tmp12 + tmp1, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    outptr[5] = range_limit[(int) RIGHT_SHIFT(tmp12 - tmp1, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    outptr[3] = range_limit[(int) RIGHT_SHIFT(tmp13 + tmp0, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    outptr[4] = range_limit[(int) RIGHT_SHIFT(tmp13 - tmp0, CONST_BITS+PASS1_BITS+3) & RANGE_MASK];
    wsptr += DCTSIZE;
  }
}

GLOBAL(void) jpeg_idct_16x16 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_16x8 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_8x16 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_14x14 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_12x12 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_10x10 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_9x9 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_7x7 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_6x6 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_5x5 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_4x4 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_3x3 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_2x2 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_1x1 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_11x11 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_13x13 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_15x15 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_16x16_p (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_12x6 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_6x12 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_10x5 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_5x10 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_8x4 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_4x8 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_6x3 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_3x6 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_4x2 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
GLOBAL(void) jpeg_idct_2x4 (j_decompress_ptr cinfo, jpeg_component_info * compptr, JCOEFPTR coef_block, JSAMPARRAY output_buf, JDIMENSION output_col) {}
