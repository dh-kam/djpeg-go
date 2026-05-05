package color

// This file ports jdpostct.c from IJG libjpeg 9f to pure Go.
// It implements the decompression post-processing controller.
//
// The post-processing controller sits between the main buffer controller
// and the output. It manages:
//   - Pass-through mode (no quantization): just calls the upsampler directly
//   - One-pass quantization mode: buffers output for color quantization
//   - Two-pass quantization mode: full-image buffering for 2-pass quantization
//
// For typical usage without color quantization, this is essentially a pass-through.

// PostProcessor manages the post-processing step in the decompression pipeline.
type PostProcessor struct {
	// Strip buffer for one-pass color quantization
	Buffer       [][]byte // strip buffer for color quantization
	StripHeight  int      // buffer size in rows

	// Full image buffer for two-pass quantization
	WholeImage   [][]byte // full image buffer (for 2-pass)
	StartingRow  int
	NextRow      int

	// Whether we need full image buffering
	NeedFullBuffer bool

	// Method to call for processing
	postProcessData func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int)

	// Upsampler reference (for pass-through mode)
	upsampler UpsampleFunc
}

// UpsampleFunc is the signature for the upsampler callback.
type UpsampleFunc func(info *DecompressInfo,
	inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
	outputBuf [][]byte, outRowCtr *int, outRowsAvail int)

// NewPostProcessor creates a post-processing controller.
func NewPostProcessor(info *DecompressInfo, needFullBuffer bool,
	upsamplerFn UpsampleFunc) *PostProcessor {

	pp := &PostProcessor{
		upsampler: upsamplerFn,
	}

	if info.QuantizeColors {
		pp.StripHeight = info.MaxVSampFactor

		if needFullBuffer {
			// Two-pass color quantization: need full image storage
			pp.NeedFullBuffer = true
			totalRows := roundUp(info.OutputHeight, pp.StripHeight)
			rowWidth := info.OutputWidth * info.OutColorComponents
			pp.WholeImage = make([][]byte, totalRows)
			for r := range pp.WholeImage {
				pp.WholeImage[r] = make([]byte, rowWidth)
			}
		} else {
			// One-pass: just allocate a strip buffer
			rowWidth := info.OutputWidth * info.OutColorComponents
			pp.Buffer = make([][]byte, pp.StripHeight)
			for r := range pp.Buffer {
				pp.Buffer[r] = make([]byte, rowWidth)
			}
		}
	}

	return pp
}

// StartPass initializes for a processing pass.
func (pp *PostProcessor) StartPass(passMode int) {
	pp.StartingRow = 0
	pp.NextRow = 0

	switch passMode {
	case JBufPassThru:
		if pp.postProcessData == nil {
			// Default: pass-through to upsampler
			pp.postProcessData = pp.passThruProcess
		}
	}
}

// PostProcessData performs the post-processing step.
// In pass-through mode (no quantization), this just calls the upsampler directly.
func (pp *PostProcessor) PostProcessData(
	info *DecompressInfo,
	inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
	outputBuf [][]byte, outRowCtr *int, outRowsAvail int,
) {
	if pp.postProcessData != nil {
		pp.postProcessData(info, inputBuf, inRowGroupCtr, inRowGroupsAvail,
			outputBuf, outRowCtr, outRowsAvail)
	}
}

// passThruProcess is the pass-through mode: call upsampler directly.
// This is used when no color quantization is required.
func (pp *PostProcessor) passThruProcess(
	info *DecompressInfo,
	inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
	outputBuf [][]byte, outRowCtr *int, outRowsAvail int,
) {
	if pp.upsampler != nil {
		pp.upsampler(info, inputBuf, inRowGroupCtr, inRowGroupsAvail,
			outputBuf, outRowCtr, outRowsAvail)
	}
}

// postProcess1Pass handles one-pass color quantization mode.
// Upsamples into a strip buffer, then quantizes.
func (pp *PostProcessor) postProcess1Pass(
	info *DecompressInfo,
	inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
	outputBuf [][]byte, outRowCtr *int, outRowsAvail int,
	colorQuantize func(input [][]byte, output [][]byte, numRows int),
) {
	maxRows := outRowsAvail - *outRowCtr
	if maxRows > pp.StripHeight {
		maxRows = pp.StripHeight
	}

	numRows := 0
	// Use a temporary counter for the upsampler
	tmpCtr := *inRowGroupCtr
	_ = inputBuf
	_ = inRowGroupsAvail

	// The upsampler writes into our strip buffer
	if pp.upsampler != nil {
		pp.upsampler(info, inputBuf, &tmpCtr, inRowGroupsAvail,
			pp.Buffer, &numRows, maxRows)
	}
	*inRowGroupCtr = tmpCtr

	// Quantize and emit
	if colorQuantize != nil && numRows > 0 {
		colorQuantize(pp.Buffer, outputBuf[*outRowCtr:], numRows)
	}
	*outRowCtr += numRows
}
