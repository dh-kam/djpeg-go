package color

// This file ports jdmainct.c from IJG libjpeg 9f to pure Go.
// It implements the main buffer controller for decompression.
//
// The main controller sits between the coefficient controller and the
// post-processor. Its primary responsibility is to provide context rows
// for fancy upsampling: it ensures that the upsampler always has access
// to one row group above and below the current processing position.
//
// Buffer management strategies:
//   1. Simple mode (no context rows): just pass iMCU rows through
//   2. Context mode: uses "funny pointer" technique to avoid copying data
//      - Maintains two alternating pointer sets (xbuffer[0] and xbuffer[1])
//      - The last two row groups of the previous iMCU row are preserved
//      - At image boundaries, rows are duplicated for context
//
// The pointer structure for context mode (M = min_DCT_v_scaled_size):
//
//   Physical rows:  0, 1, ..., M-3, M-2, M-1, M, M+1
//
//   xbuffer[0]:     0, 1, ..., M-3, M-2, M-1, M, M+1, (wrap to 0)
//   xbuffer[1]:     0, 1, ..., M-3, M, M+1, M-2, M-1, (wrap to 0)
//
// The swapped last entries in xbuffer[1] preserve the previous iMCU row's
// data while the new iMCU row is being read into the other entries.

// Context state machine values
const (
	CTXPrepareForIMCU = 0 // need to prepare for MCU row
	CTXProcessIMCU    = 1 // feeding iMCU to postprocessor
	CTXPostponedRow   = 2 // feeding postponed row group
)

// MainController manages the main decompression buffer pipeline.
type MainController struct {
	// Per-component sample row buffers
	// buffer[ci] holds rgroup*ngroups rows of sample data for component ci
	Buffer [][][]byte // [component][row] -> []byte row data

	// Row group tracking (simple mode)
	RowGroupCtr    int // counts row groups output to postprocessor
	RowGroupsAvail int // row groups available to postprocessor

	// Context mode state
	BufferFull bool
	WhichPtr   int // 0 or 1, alternates between xbuffer sets
	ContextState int
	IMCURowCtr  int // counts iMCU rows processed

	// Funny pointer lists for context mode
	// xbuffer[0][ci] and xbuffer[1][ci] are the two alternating pointer sets
	// Each is [][]byte, a list of row pointers
	// Indexed with an offset of rgroup, so the "logical" row i is at xbuf[i+offset]
	// The offset allows negative indexing for "above" context rows
	XBuffer       [2][][][]byte // [which][component][row] -> []byte
	xbufOffset    []int         // per-component offset for xbuffer indexing

	// Info reference
	info *DecompressInfo
}

// NewMainController creates and initializes the main buffer controller.
func NewMainController(info *DecompressInfo, needFullBuffer bool) *MainController {
	if needFullBuffer {
		// shouldn't happen in normal decompression
		return nil
	}

	mc := &MainController{
		info: info,
	}

	// Determine number of row groups needed
	needContextRows := false // will be set based on upsampler requirements
	ngroups := info.MinDCTVScalSize
	mc.RowGroupsAvail = ngroups

	// If context rows are needed (fancy upsampling), we need M+2 row groups
	// and the funny pointer setup
	_ = needContextRows // will be properly set by SetNeedContextRows

	// Allocate per-component sample row buffers
	mc.Buffer = make([][][]byte, info.NumComponents)
	for ci := range info.CompInfo {
		compptr := &info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}
		rgroup := (compptr.VSampFactor * compptr.DCTVScalSize) / info.MinDCTVScalSize
		rowWidth := compptr.WidthInBlocks * compptr.DCTHScalSize

		mc.Buffer[ci] = make([][]byte, rgroup*ngroups)
		for r := 0; r < rgroup*ngroups; r++ {
			mc.Buffer[ci][r] = make([]byte, rowWidth)
		}
	}

	return mc
}

// NewMainControllerWithContext creates a main controller with context row support.
// This is used when the upsampler requires context rows (fancy upsampling).
func NewMainControllerWithContext(info *DecompressInfo) *MainController {
	mc := &MainController{
		info: info,
	}

	M := info.MinDCTVScalSize
	if M < 2 {
		// Context rows not supported for M < 2
		return nil
	}

	ngroups := M + 2

	// Allocate funny pointer lists
	mc.XBuffer[0] = make([][][]byte, info.NumComponents)
	mc.XBuffer[1] = make([][][]byte, info.NumComponents)
	mc.xbufOffset = make([]int, info.NumComponents)

	// Allocate the actual sample buffers and set up pointer lists
	mc.Buffer = make([][][]byte, info.NumComponents)
	for ci := range info.CompInfo {
		compptr := &info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}
		rgroup := (compptr.VSampFactor * compptr.DCTVScalSize) / info.MinDCTVScalSize
		rowWidth := compptr.WidthInBlocks * compptr.DCTHScalSize
		totalRows := rgroup * ngroups

		// Allocate physical buffer
		mc.Buffer[ci] = make([][]byte, totalRows)
		for r := 0; r < totalRows; r++ {
			mc.Buffer[ci][r] = make([]byte, rowWidth)
		}

		// Offset: rgroup entries before logical row 0 for "above" context
		mc.xbufOffset[ci] = rgroup

		// Allocate pointer lists for both xbuffers
		// Need rgroup*(M+2) logical entries, plus rgroup offset at each end
		// Total: rgroup + rgroup*(M+2) + rgroup = rgroup*(M+4)
		ptrListSize := rgroup * (M + 4)
		mc.XBuffer[0][ci] = make([][]byte, ptrListSize)
		mc.XBuffer[1][ci] = make([][]byte, ptrListSize)
	}

	return mc
}

// StartPass initializes for a processing pass.
func (mc *MainController) StartPass(passMode int, needContextRows bool) {
	switch passMode {
	case JBufPassThru:
		if needContextRows {
			mc.initContextMode()
		} else {
			mc.RowGroupCtr = mc.RowGroupsAvail // mark buffer empty
		}
	case JBufCrankDest:
		// Just crank the postprocessor (for 2-pass quantization)
	}
}

// initContextMode sets up the context row processing state.
func (mc *MainController) initContextMode() {
	mc.makeFunnyPointers()
	mc.WhichPtr = 0
	mc.ContextState = CTXPrepareForIMCU
	mc.IMCURowCtr = 0
	mc.BufferFull = false
}

// makeFunnyPointers creates the alternating pointer lists for context mode.
// The physical buffer rows are mapped into a specific ordering that
// preserves the last two row groups across iMCU row boundaries.
//
// Logical index i maps to physical index i + offset (offset = rgroup).
func (mc *MainController) makeFunnyPointers() {
	M := mc.info.MinDCTVScalSize

	for ci := range mc.info.CompInfo {
		compptr := &mc.info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}
		rgroup := (compptr.VSampFactor * compptr.DCTVScalSize) / mc.info.MinDCTVScalSize
		off := mc.xbufOffset[ci]

		xbuf0 := mc.XBuffer[0][ci]
		xbuf1 := mc.XBuffer[1][ci]
		buf := mc.Buffer[ci]

		// First copy the workspace pointers as-is (to logical positions 0..M+1)
		for i := 0; i < rgroup*(M+2); i++ {
			xbuf0[off+i] = buf[i]
			xbuf1[off+i] = buf[i]
		}

		// In xbuf1, swap the last two row groups with the second-to-last pair
		for i := 0; i < rgroup*2; i++ {
			xbuf1[off+rgroup*(M-2)+i] = buf[rgroup*M+i]
			xbuf1[off+rgroup*M+i] = buf[rgroup*(M-2)+i]
		}

		// Set "above" pointers to duplicate the first row (top of image)
		// These are at logical positions -rgroup..-1, i.e., physical 0..rgroup-1
		for i := 0; i < rgroup; i++ {
			xbuf0[off-rgroup+i] = xbuf0[off]
		}
	}
}

// setWraparoundPointers sets up wraparound at top and bottom.
// This transitions from top-of-image state to normal state.
func (mc *MainController) setWraparoundPointers() {
	M := mc.info.MinDCTVScalSize

	for ci := range mc.info.CompInfo {
		compptr := &mc.info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}
		rgroup := (compptr.VSampFactor * compptr.DCTVScalSize) / mc.info.MinDCTVScalSize
		off := mc.xbufOffset[ci]

		xbuf0 := mc.XBuffer[0][ci]
		xbuf1 := mc.XBuffer[1][ci]

		for i := 0; i < rgroup; i++ {
			// "Above" context points to bottom of the buffer (wrap up)
			xbuf0[off-rgroup+i] = xbuf0[off+rgroup*(M+1)+i]
			xbuf1[off-rgroup+i] = xbuf1[off+rgroup*(M+1)+i]
			// "Below" context points to top of the buffer (wrap down)
			xbuf0[off+rgroup*(M+2)+i] = xbuf0[off+i]
			xbuf1[off+rgroup*(M+2)+i] = xbuf1[off+i]
		}
	}
}

// setBottomPointers adjusts pointer lists at the bottom of the image.
// Duplicates the last real sample row to pad out context rows.
func (mc *MainController) setBottomPointers() {
	for ci := range mc.info.CompInfo {
		compptr := &mc.info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}

		iMCUHeight := compptr.VSampFactor * compptr.DCTVScalSize
		rgroup := iMCUHeight / mc.info.MinDCTVScalSize
		off := mc.xbufOffset[ci]

		// Count nondummy sample rows remaining
		rowsLeft := compptr.DownsampledHeight % iMCUHeight
		if rowsLeft == 0 {
			rowsLeft = iMCUHeight
		}

		// Count nondummy row groups (only need to do once)
		if ci == 0 {
			mc.RowGroupsAvail = (rowsLeft-1)/rgroup + 1
		}

		// Duplicate last real row for context padding
		xbuf := mc.XBuffer[mc.WhichPtr][ci]
		for i := 0; i < rgroup*2; i++ {
			idx := off + rowsLeft + i
			if idx < len(xbuf) {
				xbuf[idx] = xbuf[off+rowsLeft-1]
			}
		}
	}
}

// ProcessDataSimple handles the simple (no-context) case.
// Reads one iMCU row from the coefficient controller and feeds it
// to the postprocessor as row groups.
func (mc *MainController) ProcessDataSimple(
	info *DecompressInfo,
	coefController *CoefController,
	postProcessor *PostProcessor,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
	upsampleFunc func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int),
) {
	// Read input data if buffer is empty
	if mc.RowGroupCtr >= mc.RowGroupsAvail {
		result := coefController.DecompressData(info, mc.Buffer)
		if result == JPEGSuspended {
			return
		}
		mc.RowGroupCtr = 0
	}

	// Feed the postprocessor
	if postProcessor != nil {
		postProcessor.PostProcessData(info,
			mc.Buffer, &mc.RowGroupCtr, mc.RowGroupsAvail,
			outputBuf, outRowCtr, outRowsAvail)
	} else if upsampleFunc != nil {
		upsampleFunc(info,
			mc.Buffer, &mc.RowGroupCtr, mc.RowGroupsAvail,
			outputBuf, outRowCtr, outRowsAvail)
	}
}

// ProcessDataContext handles the context row case.
// Uses the funny pointer technique to provide context for fancy upsampling.
func (mc *MainController) ProcessDataContext(
	info *DecompressInfo,
	coefController *CoefController,
	postProcessor *PostProcessor,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
	upsampleFunc func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int),
) {
	// Read input data if buffer not full
	if !mc.BufferFull {
		result := coefController.DecompressData(info, mc.XBuffer[mc.WhichPtr])
		if result == JPEGSuspended {
			return
		}
		mc.BufferFull = true
		mc.IMCURowCtr++
	}

	// State machine for processing
	switch mc.ContextState {
	case CTXPostponedRow:
		// Process the postponed row group from previous iMCU
		mc.feedPostProcessor(postProcessor, upsampleFunc, info,
			mc.XBuffer[mc.WhichPtr], outputBuf, outRowCtr, outRowsAvail)
		if mc.RowGroupCtr < mc.RowGroupsAvail {
			return // need to suspend
		}
		mc.ContextState = CTXPrepareForIMCU
		if *outRowCtr >= outRowsAvail {
			return
		}
		fallthrough

	case CTXPrepareForIMCU:
		// Prepare to process first M-1 row groups of this iMCU row
		mc.RowGroupCtr = 0
		mc.RowGroupsAvail = info.MinDCTVScalSize - 1
		if mc.IMCURowCtr == info.TotalIMCURows {
			mc.setBottomPointers()
		}
		mc.ContextState = CTXProcessIMCU
		fallthrough

	case CTXProcessIMCU:
		mc.feedPostProcessor(postProcessor, upsampleFunc, info,
			mc.XBuffer[mc.WhichPtr], outputBuf, outRowCtr, outRowsAvail)
		if mc.RowGroupCtr < mc.RowGroupsAvail {
			return // need to suspend
		}
		// After first iMCU, set wraparound pointers
		if mc.IMCURowCtr == 1 {
			mc.setWraparoundPointers()
		}
		// Switch to other xbuffer for next iMCU row
		mc.WhichPtr ^= 1
		mc.BufferFull = false
		// Process postponed last row group
		mc.RowGroupCtr = info.MinDCTVScalSize + 1
		mc.RowGroupsAvail = info.MinDCTVScalSize + 2
		mc.ContextState = CTXPostponedRow
	}
}

// feedPostProcessor feeds data to the postprocessor or upsampler.
func (mc *MainController) feedPostProcessor(
	postProcessor *PostProcessor,
	upsampleFunc func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int),
	info *DecompressInfo,
	inputBuf [][][]byte,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
) {
	if postProcessor != nil {
		postProcessor.PostProcessData(info,
			inputBuf, &mc.RowGroupCtr, mc.RowGroupsAvail,
			outputBuf, outRowCtr, outRowsAvail)
	} else if upsampleFunc != nil {
		upsampleFunc(info,
			inputBuf, &mc.RowGroupCtr, mc.RowGroupsAvail,
			outputBuf, outRowCtr, outRowsAvail)
	}
}

// ProcessDataCrankPost just cranks the postprocessor for 2-pass quantization.
func (mc *MainController) ProcessDataCrankPost(
	info *DecompressInfo,
	postProcessor *PostProcessor,
	outputBuf [][]byte,
	outRowCtr *int,
	outRowsAvail int,
) {
	if postProcessor != nil {
		postProcessor.PostProcessData(info,
			nil, nil, 0,
			outputBuf, outRowCtr, outRowsAvail)
	}
}
