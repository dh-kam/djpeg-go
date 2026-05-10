package color

// This file ports jdcoefct.c from IJG libjpeg 9f to pure Go.
// It implements the coefficient buffer controller for JPEG decompression.
//
// The coefficient controller sits between the entropy decoder and the IDCT step.
// It handles:
//   - Single-pass (baseline) decoding: buffers one MCU at a time, applies IDCT immediately
//   - Multi-pass (progressive) decoding: stores all coefficients in virtual arrays,
//     then outputs them scan by scan
//   - Block smoothing for progressive JPEG (estimates AC coefficients from DC neighbors)
//
// Key data structures:
//   - MCU_buffer: array of pointers to DCT coefficient blocks for the current MCU
//   - whole_image: virtual arrays holding all DCT coefficients for progressive mode
//   - blk_buffer: workspace for single-pass mode (one MCU worth of blocks)

// CoefController manages coefficient buffering and IDCT scheduling.
type CoefController struct {
	// Current MCU position (input side)
	MCUCtr         int // counts MCUs processed in current row
	MCUVertOffset  int // counts MCU rows within iMCU row
	MCURowsPerIMCU int // number of MCU rows per iMCU row

	// MCU buffer: pointers to coefficient blocks for the current MCU
	MCUBuffer [][]int16 // [DMaxBlocksInMCU][DCTSize2]
	BlkBuffer []int16   // flat backing storage

	// Virtual arrays for progressive mode (per-component)
	// Each component has a 3D array: [blockRow][blockCol][DCTSize2]
	WholeImage [][][][]int16 // [component][blockRow][blockCol][DCTSize2]

	// Block smoothing state
	CoefBitsLatch    []int // latched coefficient precision bits
	DoBlockSmoothing bool

	// Function pointers for decode and decompress
	ConsumeData    func(info *DecompressInfo, decodeMCU func() bool) int
	DecompressData func(info *DecompressInfo, outputBuf [][][]byte) int
	DecodeMCU      func(info *DecompressInfo, mcuBuffer [][]int16) bool

	// Whether using full-image buffering (progressive mode)
	UseFullBuffer bool

	// coef_arrays pointer (nil for single-pass)
	CoefArrays interface{}
}

// NewCoefController creates a coefficient controller.
// If needFullBuffer is true, allocates virtual arrays for progressive mode.
func NewCoefController(info *DecompressInfo, needFullBuffer bool) *CoefController {
	cc := &CoefController{
		UseFullBuffer: needFullBuffer,
	}

	if needFullBuffer {
		// Progressive mode: allocate full-image coefficient arrays
		cc.WholeImage = make([][][][]int16, info.NumComponents)
		for ci := range info.CompInfo {
			compptr := &info.CompInfo[ci]
			accessRows := compptr.VSampFactor
			if info.ProgressiveMode {
				accessRows *= 3 // for block smoothing
			}

			blocksHigh := roundUp(compptr.HeightInBlocks, compptr.VSampFactor)
			blocksWide := roundUp(compptr.WidthInBlocks, compptr.HSampFactor)

			cc.WholeImage[ci] = make([][][]int16, blocksHigh)
			for br := 0; br < blocksHigh; br++ {
				cc.WholeImage[ci][br] = make([][]int16, blocksWide)
				for bc := 0; bc < blocksWide; bc++ {
					cc.WholeImage[ci][br][bc] = make([]int16, DCTSize2)
				}
			}
		}
		cc.ConsumeData = cc.consumeData
		cc.DecompressData = cc.decompressData
		cc.CoefArrays = cc.WholeImage
	} else {
		// Single-pass (baseline) mode: allocate one MCU buffer
		cc.BlkBuffer = make([]int16, DMaxBlocksInMCU*DCTSize2)
		cc.MCUBuffer = make([][]int16, DMaxBlocksInMCU)
		for i := range cc.MCUBuffer {
			cc.MCUBuffer[i] = cc.BlkBuffer[i*DCTSize2 : (i+1)*DCTSize2]
		}
		cc.ConsumeData = cc.dummyConsumeData
		cc.DecompressData = cc.decompressOnePass
		cc.CoefArrays = nil
	}

	return cc
}

// StartInputPass resets state for a new input scan.
func (cc *CoefController) StartInputPass(info *DecompressInfo) {
	info.InputIMCURow = 0
	cc.startIMCURow(info)
}

// StartOutputPass initializes for an output scan.
func (cc *CoefController) StartOutputPass(info *DecompressInfo) {
	if cc.UseFullBuffer && info.ProgressiveMode && cc.CoefBitsLatch != nil {
		if info.DoBlockSmoothing && cc.smoothingOK(info) {
			cc.DecompressData = cc.decompressSmoothData
		} else {
			cc.DecompressData = cc.decompressData
		}
	}
	info.OutputIMCURow = 0
}

// startIMCURow resets within-iMCU-row counters.
func (cc *CoefController) startIMCURow(info *DecompressInfo) {
	if info.CompsInScan > 1 {
		cc.MCURowsPerIMCU = 1
	} else {
		if info.InputIMCURow < info.TotalIMCURows-1 {
			cc.MCURowsPerIMCU = info.CurCompInfo[0].VSampFactor
		} else {
			cc.MCURowsPerIMCU = info.CurCompInfo[0].LastRowHeight
		}
	}
	cc.MCUCtr = 0
	cc.MCUVertOffset = 0
}

// ---- Single-pass (baseline) mode ----

// decompressOnePass decodes and IDCTs one iMCU row.
// Returns JPEGRowCompleted, JPEGScanCompleted, or JPEGSuspended.
func (cc *CoefController) decompressOnePass(
	info *DecompressInfo,
	outputBuf [][][]byte, // [component][row][col]
) int {
	lastMCUCol := info.MCUsPerRow - 1
	lastIMCURow := info.TotalIMCURows - 1

	// Process each MCU row within this iMCU row
	for yoffset := cc.MCUVertOffset; yoffset < cc.MCURowsPerIMCU; yoffset++ {
		for mcuCol := cc.MCUCtr; mcuCol <= lastMCUCol; mcuCol++ {
			cc.zeroMCUBuffer(info)
			if cc.DecodeMCU != nil && !cc.DecodeMCU(info, cc.MCUBuffer) {
				cc.MCUVertOffset = yoffset
				cc.MCUCtr = mcuCol
				return JPEGSuspended
			}

			// Route decoded blocks to output buffers via IDCT
			cc.routeBlocksToOutput(info, outputBuf, mcuCol, yoffset, lastMCUCol, lastIMCURow)
		}
		cc.MCUCtr = 0
	}

	// Completed the iMCU row
	info.OutputIMCURow++
	info.InputIMCURow++
	if info.InputIMCURow <= lastIMCURow {
		cc.startIMCURow(info)
		return JPEGRowCompleted
	}
	return JPEGScanCompleted
}

func (cc *CoefController) zeroMCUBuffer(info *DecompressInfo) {
	blocks := info.BlocksInMCU
	if blocks <= 0 || blocks > len(cc.MCUBuffer) {
		blocks = len(cc.MCUBuffer)
	}
	for bi := 0; bi < blocks; bi++ {
		block := cc.MCUBuffer[bi]
		for i := range block {
			block[i] = 0
		}
	}
}

// routeBlocksToOutput dispatches decoded DCT blocks to IDCT and output.
func (cc *CoefController) routeBlocksToOutput(
	info *DecompressInfo,
	outputBuf [][][]byte,
	mcuCol, yoffset int,
	lastMCUCol, lastIMCURow int,
) {
	blkIdx := 0 // index into MCU buffer

	for ci := 0; ci < info.CompsInScan; ci++ {
		compptr := info.CurCompInfo[ci]
		if !compptr.ComponentNeeded {
			blkIdx += compptr.MCUBlocks
			continue
		}

		outputPtr := outputBuf[compptr.ComponentIndex]

		usefulWidth := compptr.MCUWidth
		if mcuCol == lastMCUCol {
			usefulWidth = compptr.LastColWidth
		}
		startCol := mcuCol * compptr.MCUSampleWidth

		for yindex := 0; yindex < compptr.MCUHeight; yindex++ {
			if info.InputIMCURow < lastIMCURow ||
				yoffset+yindex < compptr.LastRowHeight {
				outputCol := startCol
				outputRows := outputPtr[(yoffset+yindex)*compptr.DCTVScalSize:]
				for xindex := 0; xindex < usefulWidth; xindex++ {
					// Apply IDCT to the block
					if info.IDCTFunc != nil {
						info.IDCTFunc(compptr.ComponentIndex, compptr,
							cc.MCUBuffer[blkIdx+xindex],
							outputRows, outputCol)
					}
					outputCol += compptr.DCTHScalSize
				}
			}
			blkIdx += compptr.MCUWidth
		}
	}
}

// ---- Multi-pass (progressive) mode ----

// consumeData stores decoded coefficients into the full-image virtual arrays.
// Returns JPEGRowCompleted, JPEGScanCompleted, or JPEGSuspended.
func (cc *CoefController) consumeData(info *DecompressInfo, decodeMCU func() bool) int {
	// The caller must have decoded MCUs into cc.MCUBuffer.
	// This function routes them into the virtual arrays.

	for yoffset := cc.MCUVertOffset; yoffset < cc.MCURowsPerIMCU; yoffset++ {
		for mcuCol := cc.MCUCtr; mcuCol < info.MCUsPerRow; mcuCol++ {
			// Build pointer list from virtual arrays for this MCU
			cc.buildMCUPtrList(info, mcuCol, yoffset)

			// Decode the MCU (caller provides decode function)
			if !decodeMCU() {
				cc.MCUVertOffset = yoffset
				cc.MCUCtr = mcuCol
				return JPEGSuspended
			}
		}
		cc.MCUCtr = 0
	}

	// Completed the iMCU row
	info.InputIMCURow++
	if info.InputIMCURow < info.TotalIMCURows {
		cc.startIMCURow(info)
		return JPEGRowCompleted
	}
	return JPEGScanCompleted
}

// buildMCUPtrList fills MCUBuffer with pointers into the virtual arrays
// for the given MCU position.
func (cc *CoefController) buildMCUPtrList(info *DecompressInfo, mcuCol, yoffset int) {
	blkIdx := 0
	for ci := 0; ci < info.CompsInScan; ci++ {
		compptr := info.CurCompInfo[ci]
		startCol := mcuCol * compptr.MCUWidth
		virtArray := cc.WholeImage[compptr.ComponentIndex]
		baseRow := info.InputIMCURow * compptr.VSampFactor

		for yindex := 0; yindex < compptr.MCUHeight; yindex++ {
			row := baseRow + yoffset + yindex
			for xindex := 0; xindex < compptr.MCUWidth; xindex++ {
				col := startCol + xindex
				if row < len(virtArray) && col < len(virtArray[row]) {
					cc.MCUBuffer[blkIdx] = virtArray[row][col][:DCTSize2]
				}
				blkIdx++
			}
		}
	}
}

// decompressData outputs one iMCU row from the virtual arrays via IDCT.
// Used for progressive JPEG output.
func (cc *CoefController) decompressData(
	info *DecompressInfo,
	outputBuf [][][]byte,
) int {
	lastIMCURow := info.TotalIMCURows - 1

	for ci := range info.CompInfo {
		compptr := &info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}

		// Determine number of non-dummy block rows
		blockRows := compptr.VSampFactor
		if info.OutputIMCURow == lastIMCURow {
			blockRows = compptr.HeightInBlocks % compptr.VSampFactor
			if blockRows == 0 {
				blockRows = compptr.VSampFactor
			}
		}

		outputPtr := outputBuf[ci]
		baseRow := info.OutputIMCURow * compptr.VSampFactor

		for blockRow := 0; blockRow < blockRows; blockRow++ {
			row := baseRow + blockRow
			outputCol := 0
			outputRows := outputPtr[blockRow*compptr.DCTVScalSize:]
			for blockNum := 0; blockNum < compptr.WidthInBlocks; blockNum++ {
				if info.IDCTFunc != nil && row < len(cc.WholeImage[ci]) &&
					blockNum < len(cc.WholeImage[ci][row]) {
					info.IDCTFunc(ci, compptr,
						cc.WholeImage[ci][row][blockNum],
						outputRows, outputCol)
				}
				outputCol += compptr.DCTHScalSize
			}
		}
	}

	info.OutputIMCURow++
	if info.OutputIMCURow <= lastIMCURow {
		return JPEGRowCompleted
	}
	return JPEGScanCompleted
}

// ---- Block smoothing (progressive JPEG) ----

// Block smoothing positions in natural order
const (
	q01Pos     = 1
	q10Pos     = 8
	q20Pos     = 16
	q11Pos     = 9
	q02Pos     = 2
	savedCoefs = 6
)

// smoothingOK checks whether block smoothing is applicable and safe.
func (cc *CoefController) smoothingOK(info *DecompressInfo) bool {
	if !info.ProgressiveMode || len(info.CoefBits) == 0 {
		return false
	}

	// Allocate latch area
	if cc.CoefBitsLatch == nil {
		cc.CoefBitsLatch = make([]int, info.NumComponents*savedCoefs)
	}

	smoothingUseful := false
	latchIdx := 0

	for ci := range info.CompInfo {
		compptr := &info.CompInfo[ci]
		qtable := compptr.QuantTable
		if qtable == nil {
			return false
		}

		// Verify DC & first 5 AC quantizers are nonzero
		if qtable.QuantVal[0] == 0 ||
			qtable.QuantVal[q01Pos] == 0 ||
			qtable.QuantVal[q10Pos] == 0 ||
			qtable.QuantVal[q20Pos] == 0 ||
			qtable.QuantVal[q11Pos] == 0 ||
			qtable.QuantVal[q02Pos] == 0 {
			return false
		}

		// DC values must be at least partly known
		coefBits := info.CoefBits[ci]
		if coefBits[0] < 0 {
			return false
		}

		// Block smoothing is helpful if some AC coefficients remain inaccurate
		for coefi := 1; coefi <= 5; coefi++ {
			cc.CoefBitsLatch[latchIdx+coefi] = coefBits[coefi]
			if coefBits[coefi] != 0 {
				smoothingUseful = true
			}
		}
		latchIdx += savedCoefs
	}

	return smoothingUseful
}

// decompressSmoothData outputs one iMCU row with block smoothing applied.
// Estimates AC coefficients from neighboring DC values per JPEG spec section K.8.
func (cc *CoefController) decompressSmoothData(
	info *DecompressInfo,
	outputBuf [][][]byte,
) int {
	lastIMCURow := info.TotalIMCURows - 1

	for ci := range info.CompInfo {
		compptr := &info.CompInfo[ci]
		if !compptr.ComponentNeeded {
			continue
		}

		qtable := compptr.QuantTable
		if qtable == nil {
			continue
		}

		// Determine block rows
		blockRows := compptr.VSampFactor
		lastRow := false
		if info.OutputIMCURow == lastIMCURow {
			blockRows = compptr.HeightInBlocks % compptr.VSampFactor
			if blockRows == 0 {
				blockRows = compptr.VSampFactor
			}
			lastRow = true
		}

		firstRow := info.OutputIMCURow == 0
		baseRow := info.OutputIMCURow * compptr.VSampFactor

		// Fetch quantization values
		Q00 := int32(qtable.QuantVal[0])
		Q01 := int32(qtable.QuantVal[q01Pos])
		Q10 := int32(qtable.QuantVal[q10Pos])
		Q20 := int32(qtable.QuantVal[q20Pos])
		Q11 := int32(qtable.QuantVal[q11Pos])
		Q02 := int32(qtable.QuantVal[q02Pos])

		coefBits := cc.CoefBitsLatch[ci*savedCoefs:]
		outputPtr := outputBuf[ci]
		virtArray := cc.WholeImage[ci]

		lastBlockColumn := compptr.WidthInBlocks - 1

		for blockRow := 0; blockRow < blockRows; blockRow++ {
			row := baseRow + blockRow

			// Get neighbor block rows
			var prevBlockRow, curBlockRow, nextBlockRow [][]int16
			if firstRow && blockRow == 0 {
				prevBlockRow = virtArray[row]
			} else if row > 0 {
				prevBlockRow = virtArray[row-1]
			} else {
				prevBlockRow = virtArray[row]
			}
			curBlockRow = virtArray[row]
			if lastRow && blockRow == blockRows-1 {
				nextBlockRow = virtArray[row]
			} else if row+1 < len(virtArray) {
				nextBlockRow = virtArray[row+1]
			} else {
				nextBlockRow = virtArray[row]
			}

			// Initialize DC sliding register
			DC1 := int(prevBlockRow[0][0])
			DC2 := DC1
			DC3 := DC1
			DC4 := int(curBlockRow[0][0])
			DC5 := DC4
			DC6 := DC4
			DC7 := int(nextBlockRow[0][0])
			DC8 := DC7
			DC9 := DC7

			outputCol := 0
			outputRows := outputPtr[blockRow*compptr.DCTVScalSize:]
			for blockNum := 0; blockNum <= lastBlockColumn; blockNum++ {
				// Copy current block into workspace
				workspace := make([]int16, DCTSize2)
				copy(workspace, curBlockRow[blockNum][:DCTSize2])

				// Update DC values from right neighbors
				if blockNum < lastBlockColumn {
					DC3 = int(prevBlockRow[blockNum+1][0])
					DC6 = int(curBlockRow[blockNum+1][0])
					DC9 = int(nextBlockRow[blockNum+1][0])
				}

				// AC01 estimation
				if Al := coefBits[1]; Al != 0 && workspace[1] == 0 {
					num := 36 * Q00 * int32(DC4-DC6)
					pred := smoothPredict(num, Q01, Al)
					workspace[1] = int16(pred)
				}
				// AC10 estimation
				if Al := coefBits[2]; Al != 0 && workspace[8] == 0 {
					num := 36 * Q00 * int32(DC2-DC8)
					pred := smoothPredict(num, Q10, Al)
					workspace[8] = int16(pred)
				}
				// AC20 estimation
				if Al := coefBits[3]; Al != 0 && workspace[16] == 0 {
					num := 9 * Q00 * int32(DC2+DC8-2*DC5)
					pred := smoothPredict(num, Q20, Al)
					workspace[16] = int16(pred)
				}
				// AC11 estimation
				if Al := coefBits[4]; Al != 0 && workspace[9] == 0 {
					num := 5 * Q00 * int32(DC1-DC3-DC7+DC9)
					pred := smoothPredict(num, Q11, Al)
					workspace[9] = int16(pred)
				}
				// AC02 estimation
				if Al := coefBits[5]; Al != 0 && workspace[2] == 0 {
					num := 9 * Q00 * int32(DC4+DC6-2*DC5)
					pred := smoothPredict(num, Q02, Al)
					workspace[2] = int16(pred)
				}

				// Apply IDCT
				if info.IDCTFunc != nil {
					info.IDCTFunc(ci, compptr, workspace, outputRows, outputCol)
				}

				// Advance sliding DC register
				// DC3, DC6, DC9 are updated from right neighbors at the
				// start of the next iteration when blockNum < lastBlockColumn.
				DC1 = DC2
				DC2 = DC3
				DC4 = DC5
				DC5 = DC6
				DC7 = DC8
				DC8 = DC9
				outputCol += compptr.DCTHScalSize
			}
		}
	}

	info.OutputIMCURow++
	if info.OutputIMCURow <= lastIMCURow {
		return JPEGRowCompleted
	}
	return JPEGScanCompleted
}

// smoothPredict computes a smoothed coefficient estimate per JPEG spec K.8.
func smoothPredict(num, Q int32, Al int) int {
	if num >= 0 {
		pred := int(((Q << 7) + num) / (Q << 8))
		if Al > 0 && pred >= (1<<Al) {
			pred = (1 << Al) - 1
		}
		return pred
	}
	pred := int(((Q << 7) - num) / (Q << 8))
	if Al > 0 && pred >= (1<<Al) {
		pred = (1 << Al) - 1
	}
	return -pred
}

// dummyConsumeData is the no-op consume function for single-pass mode.
func (cc *CoefController) dummyConsumeData(info *DecompressInfo, decodeMCU func() bool) int {
	return JPEGSuspended
}

// GetMCUBuffer returns the MCU buffer for the entropy decoder to write into.
func (cc *CoefController) GetMCUBuffer() [][]int16 {
	return cc.MCUBuffer
}
