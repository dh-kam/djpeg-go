package color

// This file ports jdmerge.c from IJG libjpeg 9f to pure Go.
// It implements merged upsampling + color conversion for the common cases:
//   - h2v1_merged_upsample: 2:1 horizontal, 1:1 vertical + YCbCr->RGB
//   - h2v2_merged_upsample: 2:1 horizontal, 2:1 vertical + YCbCr->RGB
//
// Merging these steps avoids redundant multiplications: the chroma
// contribution is computed once per chroma sample, then combined with
// multiple luma samples.

// MergedUpsampler combines upsampling and color conversion in one step.
type MergedUpsampler struct {
	// Lookup tables for YCbCr -> RGB conversion (same as in ColorConverter)
	CrRTab []int32
	CbBTab []int32
	CrGTab []int32
	CbGTab []int32

	// For 2:1 vertical: spare row buffer for odd-height images
	SpareRow  []byte
	SpareFull bool

	// Output dimensions
	OutRowWidth  int // bytes per output row (outputWidth * outColorComponents)
	RowsToGo     int
	OutputHeight int // saved for reset

	// Method pointer
	upsampleMethod func(info *DecompressInfo, mu *MergedUpsampler,
		inputBuf [][][]byte, inRowGroupCtr int, outputBuf [][]byte)

	// Whether using 2:1 vertical
	v2 bool

	// Range limit table
	rangeLimit []byte
}

// NewMergedUpsampler creates a merged upsampler for YCbCr->RGB output.
// This is an optimization that combines h2v1 or h2v2 upsampling with
// YCbCr->RGB color conversion in a single pass.
func NewMergedUpsampler(info *DecompressInfo, isBGYCC bool) *MergedUpsampler {
	mu := &MergedUpsampler{
		OutRowWidth:  info.OutputWidth * info.OutColorComponents,
		RowsToGo:     info.OutputHeight,
		OutputHeight: info.OutputHeight,
		rangeLimit:   BuildSampleRangeLimit(),
	}

	if info.MaxVSampFactor == 2 {
		mu.v2 = true
		mu.upsampleMethod = mergedH2V2Method
		mu.SpareRow = make([]byte, mu.OutRowWidth)
	} else {
		mu.v2 = false
		mu.upsampleMethod = mergedH2V1Method
	}

	// Build conversion tables
	if isBGYCC {
		mu.buildBgyccRGBTable()
	} else {
		mu.buildYccRGBTable()
	}

	return mu
}

// buildYccRGBTable builds standard YCbCr->RGB lookup tables.
func (mu *MergedUpsampler) buildYccRGBTable() {
	mu.CrRTab = make([]int32, MaxJSample+1)
	mu.CbBTab = make([]int32, MaxJSample+1)
	mu.CrGTab = make([]int32, MaxJSample+1)
	mu.CbGTab = make([]int32, MaxJSample+1)

	for i := 0; i <= MaxJSample; i++ {
		x := int32(i - CenterJSample)
		mu.CrRTab[i] = (fix(1.402)*x + OneHalf) >> ScaleBits
		mu.CbBTab[i] = (fix(1.772)*x + OneHalf) >> ScaleBits
		mu.CrGTab[i] = (-fix(0.714136286)) * x
		mu.CbGTab[i] = (-fix(0.344136286))*x + OneHalf
	}
}

// buildBgyccRGBTable builds wide-gamut BG_YCC->RGB lookup tables.
func (mu *MergedUpsampler) buildBgyccRGBTable() {
	mu.CrRTab = make([]int32, MaxJSample+1)
	mu.CbBTab = make([]int32, MaxJSample+1)
	mu.CrGTab = make([]int32, MaxJSample+1)
	mu.CbGTab = make([]int32, MaxJSample+1)

	for i := 0; i <= MaxJSample; i++ {
		x := int32(i - CenterJSample)
		mu.CrRTab[i] = (fix(2.804)*x + OneHalf) >> ScaleBits
		mu.CbBTab[i] = (fix(3.544)*x + OneHalf) >> ScaleBits
		mu.CrGTab[i] = (-fix(1.428272572)) * x
		mu.CbGTab[i] = (-fix(0.688272572))*x + OneHalf
	}
}

// StartPass resets state for a new pass.
func (mu *MergedUpsampler) StartPass() {
	mu.SpareFull = false
	mu.RowsToGo = mu.OutputHeight
}

// Upsample performs merged upsampling + color conversion.
// For the 2:1 vertical case, it handles the spare row buffer management.
func (mu *MergedUpsampler) Upsample(
	info *DecompressInfo,
	inputBuf [][][]byte, // [component][row][col]
	inRowGroupCtr *int,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
) {
	if mu.v2 {
		mu.merged2VUpsample(info, inputBuf, inRowGroupCtr, outputBuf, outRowCtr, outRowsAvail)
	} else {
		mu.merged1VUpsample(info, inputBuf, inRowGroupCtr, outputBuf, outRowCtr, outRowsAvail)
	}
}

// merged2VUpsample handles the 2:1 vertical sampling case with spare row management.
func (mu *MergedUpsampler) merged2VUpsample(
	info *DecompressInfo,
	inputBuf [][][]byte,
	inRowGroupCtr *int,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
) {
	var numRows int

	if mu.SpareFull {
		// Return the spare row saved from previous cycle
		copy(outputBuf[*outRowCtr][:mu.OutRowWidth], mu.SpareRow)
		numRows = 1
		mu.SpareFull = false
	} else {
		numRows = 2
		if numRows > mu.RowsToGo {
			numRows = mu.RowsToGo
		}
		avail := outRowsAvail - *outRowCtr
		if numRows > avail {
			numRows = avail
		}

		// Create output pointer array
		workPtrs := make([][]byte, 2)
		workPtrs[0] = outputBuf[*outRowCtr]
		if numRows > 1 {
			workPtrs[1] = outputBuf[*outRowCtr+1]
		} else {
			workPtrs[1] = mu.SpareRow
			mu.SpareFull = true
		}

		// Do the merged upsampling
		mu.h2v2MergedUpsample(info, inputBuf, *inRowGroupCtr, workPtrs)
	}

	*outRowCtr += numRows
	mu.RowsToGo -= numRows
	if !mu.SpareFull {
		*inRowGroupCtr++
	}
}

// merged1VUpsample handles the 1:1 vertical sampling case (no spare row needed).
func (mu *MergedUpsampler) merged1VUpsample(
	info *DecompressInfo,
	inputBuf [][][]byte,
	inRowGroupCtr *int,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
) {
	// Just do the upsampling directly
	workPtrs := [][]byte{outputBuf[*outRowCtr]}
	mu.h2v1MergedUpsample(info, inputBuf, *inRowGroupCtr, workPtrs)
	*outRowCtr++
	*inRowGroupCtr++
}

// h2v1MergedUpsample performs merged 2:1 horizontal, 1:1 vertical
// upsampling with YCbCr->RGB color conversion.
//
// For each pair of output pixels, we compute the chroma contribution once
// (Cr->R, Cb->B, Cb+Cr->G) and then fetch two Y values.
func (mu *MergedUpsampler) h2v1MergedUpsample(
	info *DecompressInfo,
	inputBuf [][][]byte,
	inRowGroupCtr int,
	outputBuf [][]byte,
) {
	inY := inputBuf[0][inRowGroupCtr]
	inCb := inputBuf[1][inRowGroupCtr]
	inCr := inputBuf[2][inRowGroupCtr]
	out := outputBuf[0]
	rl := mu.rangeLimit

	numPairs := info.OutputWidth >> 1
	cbPos := 0
	crPos := 0
	yPos := 0
	outPos := 0

	for col := 0; col < numPairs; col++ {
		// Compute chroma contributions once per pair
		cb := int(inCb[cbPos])
		cr := int(inCr[crPos])
		cbPos++
		crPos++

		cGreen := int((mu.CbGTab[cb] + mu.CrGTab[cr]) >> ScaleBits)
		cBlue := int(mu.CbBTab[cb])
		cRed := int(mu.CrRTab[cr])

		// First pixel
		y := int(inY[yPos])
		yPos++
		out[outPos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out[outPos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out[outPos+RGBBlue] = rl[y+cBlue+MaxJSample+1]
		outPos += RGBPixelSize

		// Second pixel (same chroma, different luma)
		y = int(inY[yPos])
		yPos++
		out[outPos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out[outPos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out[outPos+RGBBlue] = rl[y+cBlue+MaxJSample+1]
		outPos += RGBPixelSize
	}

	// Handle odd width
	if info.OutputWidth&1 != 0 {
		y := int(inY[yPos])
		cb := int(inCb[cbPos])
		cr := int(inCr[crPos])
		out[outPos+RGBRed] = rl[y+int(mu.CrRTab[cr])+MaxJSample+1]
		g := int((mu.CbGTab[cb]+mu.CrGTab[cr])>>ScaleBits) + MaxJSample + 1
		out[outPos+RGBGreen] = rl[g]
		out[outPos+RGBBlue] = rl[y+int(mu.CbBTab[cb])+MaxJSample+1]
	}
}

// h2v2MergedUpsample performs merged 2:1 horizontal, 2:1 vertical
// upsampling with YCbCr->RGB color conversion.
//
// For each group of 4 output pixels (2x2), we compute chroma contributions
// once and then fetch 4 Y values.
func (mu *MergedUpsampler) h2v2MergedUpsample(
	info *DecompressInfo,
	inputBuf [][][]byte,
	inRowGroupCtr int,
	outputBuf [][]byte,
) {
	// Y component has 2 rows per row group for h2v2
	inYRow0 := inputBuf[0][inRowGroupCtr*2]
	inYRow1 := inputBuf[0][inRowGroupCtr*2+1]
	inCb := inputBuf[1][inRowGroupCtr]
	inCr := inputBuf[2][inRowGroupCtr]

	out0 := outputBuf[0]
	out1 := outputBuf[1]
	rl := mu.rangeLimit

	numPairs := info.OutputWidth >> 1
	y0Pos := 0
	y1Pos := 0
	cbPos := 0
	crPos := 0
	out0Pos := 0
	out1Pos := 0

	for col := 0; col < numPairs; col++ {
		// Compute chroma contributions once per 2x2 group
		cb := int(inCb[cbPos])
		cr := int(inCr[crPos])
		cbPos++
		crPos++

		cGreen := int((mu.CbGTab[cb] + mu.CrGTab[cr]) >> ScaleBits)
		cBlue := int(mu.CbBTab[cb])
		cRed := int(mu.CrRTab[cr])

		// Pixel (0,0)
		y := int(inYRow0[y0Pos])
		y0Pos++
		out0[out0Pos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out0[out0Pos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out0[out0Pos+RGBBlue] = rl[y+cBlue+MaxJSample+1]
		out0Pos += RGBPixelSize

		// Pixel (1,0)
		y = int(inYRow0[y0Pos])
		y0Pos++
		out0[out0Pos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out0[out0Pos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out0[out0Pos+RGBBlue] = rl[y+cBlue+MaxJSample+1]
		out0Pos += RGBPixelSize

		// Pixel (0,1)
		y = int(inYRow1[y1Pos])
		y1Pos++
		out1[out1Pos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out1[out1Pos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out1[out1Pos+RGBBlue] = rl[y+cBlue+MaxJSample+1]
		out1Pos += RGBPixelSize

		// Pixel (1,1)
		y = int(inYRow1[y1Pos])
		y1Pos++
		out1[out1Pos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out1[out1Pos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out1[out1Pos+RGBBlue] = rl[y+cBlue+MaxJSample+1]
		out1Pos += RGBPixelSize
	}

	// Handle odd width
	if info.OutputWidth&1 != 0 {
		cb := int(inCb[cbPos])
		cr := int(inCr[crPos])
		cGreen := int((mu.CbGTab[cb] + mu.CrGTab[cr]) >> ScaleBits)
		cBlue := int(mu.CbBTab[cb])
		cRed := int(mu.CrRTab[cr])

		y := int(inYRow0[y0Pos])
		out0[out0Pos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out0[out0Pos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out0[out0Pos+RGBBlue] = rl[y+cBlue+MaxJSample+1]

		y = int(inYRow1[y1Pos])
		out1[out1Pos+RGBRed] = rl[y+cRed+MaxJSample+1]
		out1[out1Pos+RGBGreen] = rl[y+cGreen+MaxJSample+1]
		out1[out1Pos+RGBBlue] = rl[y+cBlue+MaxJSample+1]
	}
}

// mergedH2V1Method is the method wrapper for h2v1 merged upsampling.
func mergedH2V1Method(info *DecompressInfo, mu *MergedUpsampler,
	inputBuf [][][]byte, inRowGroupCtr int, outputBuf [][]byte) {
	mu.h2v1MergedUpsample(info, inputBuf, inRowGroupCtr, outputBuf)
}

// mergedH2V2Method is the method wrapper for h2v2 merged upsampling.
func mergedH2V2Method(info *DecompressInfo, mu *MergedUpsampler,
	inputBuf [][][]byte, inRowGroupCtr int, outputBuf [][]byte) {
	mu.h2v2MergedUpsample(info, inputBuf, inRowGroupCtr, outputBuf)
}
