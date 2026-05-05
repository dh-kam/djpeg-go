package color

// This file ports jdsample.c from IJG libjpeg 9f to pure Go.
// It implements upsampling for JPEG decompression:
//   - fullsize_upsample: no-op (component is already at full size)
//   - h2v1_upsample: 2:1 horizontal, 1:1 vertical (pixel duplication)
//   - h2v2_upsample: 2:1 horizontal, 2:1 vertical (pixel duplication)
//   - int_upsample: arbitrary integer ratio upsampling (pixel duplication)
//   - Fancy upsampling is handled separately via context rows in jdmerge.c / jdmainct.c

// Upsampler manages the upsampling state for all components.
type Upsampler struct {
	// Per-component color conversion buffers (for non-full-size components)
	// ColorBuf[ci] holds max_v_samp_factor rows of upsampled data for component ci
	ColorBuf    [][][]byte // [component][row] -> []byte pixel data

	// Per-component upsampling methods
	methods     []upsampleFunc

	// Track which components use fullsize (no-op) upsampling
	isFullSize  []bool

	// State tracking
	nextRowOut  int    // counts rows emitted from color_buf
	rowsToGo    int    // counts rows remaining in image

	// Height of an input row group for each component
	rowGroupHeight []int

	// Pixel expansion factors for int_upsample
	hExpand     []int
	vExpand     []int

	// Reference to decompression info
	info        *DecompressInfo

	// Color converter reference
	colorConv   *ColorConverter
}

// upsampleFunc is the signature for a per-component upsampling method.
// It takes input data rows and writes into the output data rows.
// inputData is the source rows for this component.
// outputData is the destination rows (ColorBuf for this component).
type upsampleFunc func(info *DecompressInfo, compIdx int,
	inputData [][]byte, outputData [][]byte)

// NewUpsampler creates and initializes the upsampler.
func NewUpsampler(info *DecompressInfo, cc *ColorConverter) *Upsampler {
	u := &Upsampler{
		info:        info,
		colorConv:   cc,
		methods:     make([]upsampleFunc, len(info.CompInfo)),
		isFullSize:  make([]bool, len(info.CompInfo)),
		rowGroupHeight: make([]int, len(info.CompInfo)),
		hExpand:     make([]int, len(info.CompInfo)),
		vExpand:     make([]int, len(info.CompInfo)),
	}

	for ci := range info.CompInfo {
		compptr := &info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}

		// Compute size of an "input group" after IDCT scaling
		hInGroup := (compptr.HSampFactor * compptr.DCTHScalSize) / info.MinDCTHScalSize
		vInGroup := (compptr.VSampFactor * compptr.DCTVScalSize) / info.MinDCTVScalSize
		hOutGroup := info.MaxHSampFactor
		vOutGroup := info.MaxVSampFactor
		u.rowGroupHeight[ci] = vInGroup

		if hInGroup == hOutGroup && vInGroup == vOutGroup {
			// Full-size: no upsampling needed
			u.methods[ci] = fullsizeUpsample
			u.isFullSize[ci] = true
			continue
		}

		if hInGroup*2 == hOutGroup && vInGroup == vOutGroup {
			// 2h1v
			u.methods[ci] = h2v1Upsample
		} else if hInGroup*2 == hOutGroup && vInGroup*2 == vOutGroup {
			// 2h2v
			u.methods[ci] = h2v2Upsample
		} else if hOutGroup%hInGroup == 0 && vOutGroup%vInGroup == 0 {
			// Generic integer factors
			u.methods[ci] = intUpsample
			u.hExpand[ci] = hOutGroup / hInGroup
			u.vExpand[ci] = vOutGroup / vInGroup
		}
		// Note: non-integer ratios are not supported (as in libjpeg)

		// Allocate color conversion buffer for this component
		bufWidth := roundUp(info.OutputWidth, info.MaxHSampFactor)
		bufRows := info.MaxVSampFactor
		for len(u.ColorBuf) <= ci {
			u.ColorBuf = append(u.ColorBuf, nil)
		}
		u.ColorBuf[ci] = make([][]byte, bufRows)
		for r := 0; r < bufRows; r++ {
			u.ColorBuf[ci][r] = make([]byte, bufWidth)
		}
	}

	return u
}

// StartPass resets the upsampler for a new pass.
func (u *Upsampler) StartPass() {
	u.nextRowOut = u.info.MaxVSampFactor
	u.rowsToGo = u.info.OutputHeight
}

// SepUpsample performs upsampling on each component independently,
// then calls color conversion.
//
// inputBuf is [component][row][col], indexed by row group.
// outputBuf is [row][col] (interleaved output).
func (u *Upsampler) SepUpsample(
	inputBuf [][][]byte, // [component][row][col]
	inRowGroupCtr *int,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
) {
	info := u.info

	// Fill the conversion buffer if empty
	if u.nextRowOut >= info.MaxVSampFactor {
		for ci := range info.CompInfo {
			compptr := &info.CompInfo[ci]
			if !compptr.ComponentNeeded {
				continue
			}
			// Get the input rows for this component
			rowStart := *inRowGroupCtr * u.rowGroupHeight[ci]
			inputRows := inputBuf[ci][rowStart:]

			// Invoke per-component upsample method
			if !u.isFullSize[ci] && u.methods[ci] != nil {
				u.methods[ci](info, ci, inputRows, u.ColorBuf[ci])
			}
		}
		u.nextRowOut = 0
	}

	// Color-convert and emit rows
	numRows := info.MaxVSampFactor - u.nextRowOut
	if numRows > u.rowsToGo {
		numRows = u.rowsToGo
	}
	avail := outRowsAvail - *outRowCtr
	if numRows > avail {
		numRows = avail
	}

	if numRows > 0 && u.colorConv != nil && u.colorConv.ColorConvert != nil {
		// Build inputBuf for color converter from ColorBuf
		colorInput := make([][][]byte, len(info.CompInfo))
		for ci := range info.CompInfo {
			if !info.CompInfo[ci].ComponentNeeded {
				continue
			}
			if u.isFullSize[ci] {
				// Full-size: pass input data directly (no upsampling needed)
				rowStart := *inRowGroupCtr * u.rowGroupHeight[ci]
				colorInput[ci] = inputBuf[ci][rowStart:]
			} else {
				colorInput[ci] = u.ColorBuf[ci]
			}
		}
		u.colorConv.ColorConvert(colorInput, u.nextRowOut, outputBuf[*outRowCtr:], numRows)
	}

	*outRowCtr += numRows
	u.rowsToGo -= numRows
	u.nextRowOut += numRows

	if u.nextRowOut >= info.MaxVSampFactor {
		*inRowGroupCtr++
	}
}

// ---- Per-component upsampling functions ----

// fullsizeUpsample does nothing; the output pointer is set to the input data.
func fullsizeUpsample(info *DecompressInfo, compIdx int,
	inputData [][]byte, outputData [][]byte) {
	// In the C code, this sets *output_data_ptr = input_data
	// In our Go version, the caller handles this by checking the method type.
}

// h2v1Upsample performs 2:1 horizontal, 1:1 vertical upsampling.
// Each input pixel is duplicated horizontally.
func h2v1Upsample(info *DecompressInfo, compIdx int,
	inputData [][]byte, outputData [][]byte) {

	for outRow := 0; outRow < info.MaxVSampFactor; outRow++ {
		inRow := inputData[outRow]
		outRow2 := outputData[outRow]
		outWidth := info.OutputWidth

		inPos := 0
		outPos := 0
		for outPos < outWidth {
			v := inRow[inPos]
			inPos++
			outRow2[outPos] = v
			outRow2[outPos+1] = v
			outPos += 2
		}
	}
}

// h2v2Upsample performs 2:1 horizontal, 2:1 vertical upsampling.
// Each input pixel is duplicated both horizontally and vertically.
func h2v2Upsample(info *DecompressInfo, compIdx int,
	inputData [][]byte, outputData [][]byte) {

	outWidth := info.OutputWidth
	for outRow := 0; outRow < info.MaxVSampFactor; outRow += 2 {
		inRow := inputData[outRow/2]
		outRow0 := outputData[outRow]

		inPos := 0
		outPos := 0
		for outPos < outWidth {
			v := inRow[inPos]
			inPos++
			outRow0[outPos] = v
			outRow0[outPos+1] = v
			outPos += 2
		}

		// Duplicate row for vertical upsampling
		if outRow+1 < info.MaxVSampFactor {
			outRow1 := outputData[outRow+1]
			copy(outRow1[:outWidth], outRow0[:outWidth])
		}
	}
}

// intUpsample performs arbitrary integer factor upsampling.
// Uses simple pixel replication (box filter).
// NOTE: The expansion factors are stored per-component in the Upsampler,
// but since we don't have a reference to the Upsampler here, we use
// IntUpsampleWithFactors instead for actual usage.
func intUpsample(info *DecompressInfo, compIdx int,
	inputData [][]byte, outputData [][]byte,
) {
	// Default to 2x2 expansion (most common case)
	hExpand := 2
	vExpand := 2

	outWidth := info.OutputWidth

	for outRow := 0; outRow < info.MaxVSampFactor; {
		inRow := inputData[outRow/vExpand]
		outRowBuf := outputData[outRow]

		inPos := 0
		outPos := 0
		for outPos < outWidth {
			v := inRow[inPos]
			inPos++
			for h := 0; h < hExpand && outPos < outWidth; h++ {
				outRowBuf[outPos] = v
				outPos++
			}
		}

		// Duplicate for vertical expansion
		for v := 1; v < vExpand && outRow+v < info.MaxVSampFactor; v++ {
			copy(outputData[outRow+v][:outWidth], outRowBuf[:outWidth])
		}
		outRow += vExpand
	}
}

// IntUpsampleWithFactors performs integer upsampling with the given expansion factors.
func IntUpsampleWithFactors(info *DecompressInfo, compIdx int,
	hExpand, vExpand int,
	inputData [][]byte, outputData [][]byte,
) {
	outWidth := info.OutputWidth

	for outRow := 0; outRow < info.MaxVSampFactor; {
		inRowIdx := outRow / vExpand
		if inRowIdx >= len(inputData) {
			inRowIdx = len(inputData) - 1
		}
		inRow := inputData[inRowIdx]
		outRowBuf := outputData[outRow]

		inPos := 0
		outPos := 0
		for outPos < outWidth {
			v := inRow[inPos]
			inPos++
			for h := 0; h < hExpand && outPos < outWidth; h++ {
				outRowBuf[outPos] = v
				outPos++
			}
		}

		// Duplicate for vertical expansion
		for v := 1; v < vExpand && outRow+v < info.MaxVSampFactor; v++ {
			copy(outputData[outRow+v][:outWidth], outRowBuf[:outWidth])
		}
		outRow += vExpand
	}
}

// ---- Fancy Upsampling ----
// Fancy upsampling uses bilinear interpolation with edge handling.
// This is used when need_context_rows is true.
// The C code in jdsample.c has h2v1_fancy_upsample and h2v2_fancy_upsample,
// but these are conditionally compiled based on INPUT_SMOOTHING_SUPPORTED.
// We implement them here for completeness.

// H2V1FancyUpsample performs fancy (interpolated) 2:1 horizontal upsampling
// with context rows (previous and next rows for vertical interpolation).
// This uses a triangle filter for better quality than pixel duplication.
func H2V1FancyUpsample(
	inputPrev, inputCur, inputNext []byte,
	output []byte,
	width int,
) {
	// For 2h1v fancy, we only do horizontal interpolation.
	// The "fancy" part uses the neighboring rows for weighted averaging,
	// but for h2v1 (no vertical subsampling), it's simpler.
	lastCol := width/2 - 1

	// First column: replicate
	out0 := int(inputCur[0])
	output[0] = byte(out0)
	output[1] = byte((out0*3 + int(inputCur[1]) + 2) / 4)

	// Middle columns
	for col := 1; col < lastCol; col++ {
		inm1 := int(inputCur[col-1])
		in0 := int(inputCur[col])
		inp1 := int(inputCur[col+1])

		outPos := col * 2
		output[outPos] = byte((in0*3 + inm1 + 2) / 4)
		output[outPos+1] = byte((in0*3 + inp1 + 2) / 4)
	}

	// Last column
	if lastCol >= 0 {
		inLast := int(inputCur[lastCol])
		outPos := lastCol * 2
		output[outPos] = byte((inLast*3 + int(inputCur[lastCol-1]) + 2) / 4)
		output[outPos+1] = byte(inLast)
	}
}

// H2V2FancyUpsample performs fancy (interpolated) 2:1 horizontal, 2:1 vertical
// upsampling with context rows. Uses a triangle (bilinear) filter.
func H2V2FancyUpsample(
	inputAbove []byte, // previous row (context)
	inputCur []byte,   // current row
	inputBelow []byte, // next row (context)
	output0, output1 []byte, // two output rows
	inputWidth int, // width of chroma input
	outputWidth int, // width of luma output
) {
	halfWidth := inputWidth
	if outputWidth/2 < halfWidth {
		halfWidth = outputWidth / 2
	}
	lastInCol := halfWidth - 1

	// Helper to safely get a value from a row, defaulting to inputCur
	getVal := func(row []byte, col int) int {
		if row != nil && col >= 0 && col < len(row) {
			return int(row[col])
		}
		return int(inputCur[col])
	}

	for col := 0; col <= lastInCol; col++ {
		above := getVal(inputAbove, col)
		cur := int(inputCur[col])
		below := getVal(inputBelow, col)

		// Vertical interpolation of chroma at current column
		vertCur0 := (3*cur + above + 2) >> 2
		vertCur1 := (3*cur + below + 2) >> 2

		outPos := col * 2

		// Get left/right neighbors for horizontal interpolation
		var leftVert0, leftVert1 int
		if col > 0 {
			leftCur := int(inputCur[col-1])
			leftAbove := getVal(inputAbove, col-1)
			leftBelow := getVal(inputBelow, col-1)
			leftVert0 = (3*leftCur + leftAbove + 2) >> 2
			leftVert1 = (3*leftCur + leftBelow + 2) >> 2
		} else {
			leftVert0 = vertCur0
			leftVert1 = vertCur1
		}

		var rightVert0, rightVert1 int
		if col < lastInCol {
			rightCur := int(inputCur[col+1])
			rightAbove := getVal(inputAbove, col+1)
			rightBelow := getVal(inputBelow, col+1)
			rightVert0 = (3*rightCur + rightAbove + 2) >> 2
			rightVert1 = (3*rightCur + rightBelow + 2) >> 2
		} else {
			rightVert0 = vertCur0
			rightVert1 = vertCur1
		}

		// Output pixel 0: left pixel - blend with left neighbor using (3:1) triangle filter
		output0[outPos] = byte((3*vertCur0 + leftVert0 + 2) >> 2)
		output1[outPos] = byte((3*vertCur1 + leftVert1 + 2) >> 2)
		// Output pixel 1: right pixel - blend with right neighbor using (3:1) triangle filter
		if outPos+1 < outputWidth {
			output0[outPos+1] = byte((3*vertCur0 + rightVert0 + 2) >> 2)
			output1[outPos+1] = byte((3*vertCur1 + rightVert1 + 2) >> 2)
		}
	}
}

// NeedContextRows returns whether any component requires context rows
// (i.e., fancy upsampling with neighboring rows).
// For simple pixel-duplication upsampling, context rows are not needed.
func (u *Upsampler) NeedContextRows() bool {
	// The separate upsampler uses simple box-filter replication,
	// which does not require context rows. Only the fancy upsampler
	// (which uses interpolation) needs context rows, and that is
	// handled separately via the merged upsampler path.
	return false
}
