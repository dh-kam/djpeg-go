// Package decoder implements a complete JPEG baseline decoder pipeline.
package decoder

import (
	"errors"
	"io"
	"math"
	"strings"

	"github.com/dh-kam/djpeg-go/internal/color"
	"github.com/dh-kam/djpeg-go/internal/huff"
	"github.com/dh-kam/djpeg-go/internal/marker"
)

// Decoder holds all state for JPEG decoding.
type Decoder struct {
	d *marker.Decompressor

	scanData []byte

	permState    huff.BitReadState
	savedState   huff.SavableState
	workState    huff.BitReadWorkingState
	restartsToGo int
	insufficient bool
	unreadMarker byte

	dcTables      []*huff.DerivedHuffTable
	acTables      []*huff.DerivedHuffTable
	mcuMembership []int

	idctMethod huff.IDCTMethod

	multTablesISlow []*huff.ISlowMultTable
	multTablesIFast []*huff.IFASTMultTable
	multTablesFloat []*huff.FloatMultTable

	rangeLimit  *huff.RangeLimitTable
	rlColorConv []byte

	outputBuf [16]huff.BlockRow
	blocks    []huff.Block

	quantTables [][huff.DCTSize2]int32

	vCbRow []int
	vCrRow []int

	colorConv *color.ColorConverter

	componentBuf [][][]byte

	rowGroupCtr    int
	rowGroupsAvail int
	currentIMCURow int
	totalIMCURows  int
	allDecoded     bool

	DoFancyUpsampling        bool
	DisableChromaIDCTScaling bool

	inputColorSpaceOverride    marker.ColorSpace
	hasInputColorSpaceOverride bool
}

var (
	ErrUnsupportedJPEG = errors.New("jpeg: unsupported JPEG format")
	ErrDecodeFailed    = errors.New("jpeg: decode failed")
)

const rlColorOffset = huff.RangeSubset

func New(r io.Reader) *Decoder {
	d := marker.NewDecompressor()
	d.SetSource(r)
	d.DoFancyUpsampling = true
	return &Decoder{d: d, idctMethod: huff.IDCTISlow, DoFancyUpsampling: true}
}

func (dec *Decoder) SetIDCTMethod(method string) {
	switch method {
	case "fast":
		dec.idctMethod = huff.IDCTIFast
	case "float":
		dec.idctMethod = huff.IDCTFloat
	default:
		dec.idctMethod = huff.IDCTISlow
	}
}

func (dec *Decoder) SetFancyUpsampling(fancy bool) {
	dec.DoFancyUpsampling = fancy
	dec.d.DoFancyUpsampling = fancy
}

func (dec *Decoder) SetChromaIDCTScaling(enabled bool) {
	dec.DisableChromaIDCTScaling = !enabled
	dec.d.DisableChromaIDCTScaling = !enabled
}

// SetInputColorSpace overrides the JPEG sample colorspace inferred from
// markers and component IDs. It is intended for containers such as PDF that
// carry colorspace metadata outside the JPEG stream.
func (dec *Decoder) SetInputColorSpace(space string) error {
	var cs marker.ColorSpace
	switch strings.ToLower(strings.TrimSpace(space)) {
	case "", "auto":
		dec.hasInputColorSpaceOverride = false
		return nil
	case "gray", "grey", "grayscale", "greyscale":
		cs = marker.CSGrayScale
	case "rgb":
		cs = marker.CSRGB
	case "ycbcr", "ycc":
		cs = marker.CSYCbCr
	default:
		return errors.New("jpeg: unsupported input colorspace")
	}
	dec.inputColorSpaceOverride = cs
	dec.hasInputColorSpaceOverride = true
	dec.applyInputColorSpaceOverride()
	return nil
}

func (dec *Decoder) applyInputColorSpaceOverride() {
	if !dec.hasInputColorSpaceOverride {
		return
	}
	dec.d.JPEGColorSpace = dec.inputColorSpaceOverride
	switch dec.inputColorSpaceOverride {
	case marker.CSGrayScale:
		dec.d.OutColorSpace = marker.CSGrayScale
	case marker.CSRGB, marker.CSYCbCr:
		dec.d.OutColorSpace = marker.CSRGB
	}
}

func (dec *Decoder) ReadHeader() (int, int, int, marker.ColorSpace, error) {
	retcode, err := dec.d.ReadHeader(true)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if retcode != marker.JPEGHeaderOK {
		return 0, 0, 0, 0, ErrUnsupportedJPEG
	}
	if dec.d.IsProgressive() {
		return 0, 0, 0, 0, errors.New("jpeg: progressive JPEG not yet supported")
	}
	if dec.d.ArithCodeFlag {
		return 0, 0, 0, 0, errors.New("jpeg: arithmetic coding not supported")
	}
	dec.applyInputColorSpaceOverride()
	return dec.d.ImageWidth, dec.d.ImageHeight, dec.d.NumComponents, dec.d.JPEGColorSpace, nil
}

func (dec *Decoder) StartDecompress() error {
	d := dec.d
	dec.applyInputColorSpaceOverride()
	d.DoFancyUpsampling = dec.DoFancyUpsampling
	d.DisableChromaIDCTScaling = dec.DisableChromaIDCTScaling
	if err := d.StartInputPass(); err != nil {
		return err
	}
	if err := dec.buildHuffmanTables(); err != nil {
		return err
	}
	dec.buildQuantTables()
	dec.buildIDCTTables()

	dec.rangeLimit = huff.NewRangeLimitTable()
	dec.rlColorConv = make([]byte, 1024+2*rlColorOffset)
	for i := range dec.rlColorConv {
		v := i - rlColorOffset
		if v < 0 {
			dec.rlColorConv[i] = 0
		} else if v > 255 {
			dec.rlColorConv[i] = 255
		} else {
			dec.rlColorConv[i] = byte(v)
		}
	}

	dec.restartsToGo = int(d.RestartInterval)
	dec.insufficient = false
	dec.unreadMarker = 0
	dec.permState = huff.BitReadState{}
	dec.savedState = huff.SavableState{}
	dec.workState = huff.BitReadWorkingState{}

	dec.mcuMembership = make([]int, d.BlocksInMCU)
	copy(dec.mcuMembership, d.MCUMembership[:d.BlocksInMCU])
	dec.blocks = make([]huff.Block, d.BlocksInMCU)

	if err := dec.readAllScanData(); err != nil {
		return err
	}
	huff.InitBitReader(&dec.workState, dec.scanData)
	if err := d.StartDecompress(); err != nil {
		return err
	}
	dec.setupColorPipeline()
	dec.totalIMCURows = d.MCURowsInScan
	dec.currentIMCURow = 0
	return nil
}

func (dec *Decoder) OutputWidth() int                  { return dec.d.OutputWidth }
func (dec *Decoder) OutputHeight() int                 { return dec.d.OutputHeight }
func (dec *Decoder) OutputComponents() int             { return dec.d.OutputComponents }
func (dec *Decoder) OutColorSpace() marker.ColorSpace  { return dec.d.OutColorSpace }
func (dec *Decoder) JPEGColorSpace() marker.ColorSpace { return dec.d.JPEGColorSpace }

func (dec *Decoder) ReadScanlines(scanlines [][]uint8) (int, error) {
	if dec.d.GlobalState != marker.DStateScanning {
		return 0, marker.ErrBadState
	}
	if dec.d.OutputScanline >= dec.d.OutputHeight {
		return 0, nil
	}
	maxLines := len(scanlines)
	rowsRead := 0
	if !dec.allDecoded {
		for dec.currentIMCURow < dec.totalIMCURows {
			dec.decodeIMCURow()
			dec.currentIMCURow++
		}
		dec.allDecoded = true
		dec.rowGroupCtr = 0
		dec.rowGroupsAvail = dec.d.OutputHeight
	}
	for rowsRead < maxLines && dec.d.OutputScanline < dec.d.OutputHeight {
		dec.upsampleAndConvert(scanlines[rowsRead])
		rowsRead++
		dec.d.OutputScanline++
		dec.rowGroupCtr++
	}
	return rowsRead, nil
}

func (dec *Decoder) FinishDecompress() error {
	err := dec.d.FinishDecompress()
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err.Error() == "jpeg: data source suspension" {
			return nil
		}
	}
	return err
}

func (dec *Decoder) readAllScanData() error {
	capHint := 0
	if r, ok := dec.d.Src.(interface{ Len() int }); ok {
		capHint = r.Len()
	}
	buf := make([]byte, 0, capHint)
	readBuf := make([]byte, 8192)
	for {
		n, err := io.ReadFull(dec.d.Src, readBuf)
		if n > 0 {
			buf = append(buf, readBuf[:n]...)
		}
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
	}
	end := len(buf)
	i := 0
	for i < len(buf) {
		if buf[i] == 0xFF {
			if i+1 >= len(buf) {
				end = i
				break
			}
			next := buf[i+1]
			if next == 0x00 {
				i += 2
				continue
			}
			if next >= 0xD0 && next <= 0xD7 {
				copy(buf[i:], buf[i+2:])
				end -= 2
				continue
			}
			if next == 0xFF {
				i++
				continue
			}
			end = i
			break
		}
		i++
	}
	dec.scanData = buf[:end]
	return nil
}

func (dec *Decoder) decodeIMCURow() {
	d := dec.d
	mcusPerRow := d.MCUsPerRow
	blocksInMCU := d.BlocksInMCU
	blocks := dec.blocks

	for mcuCol := 0; mcuCol < mcusPerRow; mcuCol++ {
		for i := range blocks {
			blocks[i] = huff.Block{}
		}
		if d.RestartInterval != 0 {
			if dec.restartsToGo == 0 {
				dec.savedState = huff.SavableState{}
				dec.unreadMarker = 0
				dec.insufficient = false
				dec.restartsToGo = int(d.RestartInterval)
			}
			dec.restartsToGo--
		}
		_ = huff.DecodeMCUSequential(
			&dec.workState, &dec.permState, &dec.savedState, blocks,
			dec.dcTables, dec.acTables, blocksInMCU, dec.mcuMembership,
			0, &dec.restartsToGo, &dec.insufficient, &dec.unreadMarker,
		)
		dec.routeBlocks(blocks, mcuCol)
	}
}

func (dec *Decoder) routeBlocks(blocks []huff.Block, mcuCol int) {
	d := dec.d
	rl := dec.rangeLimit
	blkIdx := 0
	isISlow := dec.idctMethod == huff.IDCTISlow

	for ci := 0; ci < d.CompsInScan; ci++ {
		comp := d.CurCompInfo[ci]
		if !comp.ComponentNeeded {
			blkIdx += comp.MCUBlocks
			continue
		}
		usefulWidth := comp.MCUWidth
		if mcuCol == d.MCUsPerRow-1 {
			usefulWidth = comp.LastColWidth
		}
		startCol := mcuCol * comp.MCUSampleWidth
		compBuf := dec.componentBuf[comp.ComponentIndex]
		numRows := len(compBuf)
		rowOffset := dec.currentIMCURow * comp.MCUHeight * comp.DCVScaledSize
		dctSize := comp.DCVScaledSize
		dctStep := comp.DCHScaledSize

		if isISlow {
			qt := dec.multTablesISlow[ci]
			for yIndex := 0; yIndex < comp.MCUHeight; yIndex++ {
				outputBuf := dec.outputBuf[:dctSize]
				for row := 0; row < dctSize; row++ {
					bufRow := rowOffset + yIndex*dctSize + row
					if bufRow < numRows {
						outputBuf[row] = compBuf[bufRow]
					} else {
						// Replicate the last valid row
						outputBuf[row] = compBuf[numRows-1]
					}
				}
				outputCol := startCol
				for xIndex := 0; xIndex < usefulWidth; xIndex++ {
					if dctSize == 16 && dctStep == 16 {
						huff.IDCT16x16Impl(blocks[blkIdx+xIndex][:], qt, outputBuf, outputCol, rl)
					} else {
						huff.IDCTISlowImpl(blocks[blkIdx+xIndex][:], qt, outputBuf, outputCol, rl)
					}
					outputCol += dctStep
				}
				blkIdx += comp.MCUWidth
			}
		} else {
			for yIndex := 0; yIndex < comp.MCUHeight; yIndex++ {
				outputBuf := dec.outputBuf[:dctSize]
				for row := 0; row < dctSize; row++ {
					bufRow := rowOffset + yIndex*dctSize + row
					if bufRow < len(compBuf) {
						outputBuf[row] = compBuf[bufRow]
					}
				}
				outputCol := startCol
				for xIndex := 0; xIndex < usefulWidth; xIndex++ {
					coefBlock := blocks[blkIdx+xIndex][:]
					switch dec.idctMethod {
					case huff.IDCTIFast:
						huff.IDCTIFastImpl(coefBlock, dec.multTablesIFast[ci], outputBuf, outputCol, rl)
					case huff.IDCTFloat:
						huff.IDCTFloatImpl(coefBlock, dec.multTablesFloat[ci], outputBuf, outputCol, rl)
					}
					outputCol += dctStep
				}
				blkIdx += comp.MCUWidth
			}
		}
	}
}

func (dec *Decoder) upsampleAndConvert(outputRow []byte) {
	d := dec.d
	outputWidth := d.OutputWidth

	if d.OutputComponents == 1 {
		rowIdx := dec.rowGroupCtr
		if rowIdx >= len(dec.componentBuf[0]) {
			rowIdx = len(dec.componentBuf[0]) - 1
		}
		srcRow := dec.componentBuf[0][rowIdx]
		n := outputWidth
		if n > len(srcRow) {
			n = len(srcRow)
		}
		copy(outputRow[:n], srcRow[:n])
		return
	}

	if d.JPEGColorSpace == marker.CSRGB && d.OutColorSpace == marker.CSRGB && d.NumComponents >= 3 {
		dec.upsampleRGB(outputRow)
		return
	}

	yRow := dec.rowGroupCtr
	if yRow >= len(dec.componentBuf[0]) {
		yRow = len(dec.componentBuf[0]) - 1
	}
	yData := dec.componentBuf[0][yRow]

	if d.NumComponents < 3 {
		for col := 0; col < outputWidth; col++ {
			idx := col * 3
			v := yData[col]
			outputRow[idx] = v
			outputRow[idx+1] = v
			outputRow[idx+2] = v
		}
		return
	}

	yComp := &d.CompInfo[0]
	cbComp := &d.CompInfo[1]
	hUpsample := yComp.HSampFactor*yComp.DCHScaledSize > cbComp.HSampFactor*cbComp.DCHScaledSize
	vUpsample := yComp.VSampFactor*yComp.DCVScaledSize > cbComp.VSampFactor*cbComp.DCVScaledSize

	cbBuf := dec.componentBuf[1]
	crBuf := dec.componentBuf[2]
	cbBufLen := len(cbBuf)

	conv := dec.colorConv
	rl := dec.rlColorConv
	crR := conv.CrRTabInt
	cbB := conv.CbBTabInt
	crG := conv.CrGTabInt
	cbG := conv.CbGTabInt

	if hUpsample && vUpsample {
		chromaRow := yRow >> 1
		if chromaRow >= cbBufLen {
			chromaRow = cbBufLen - 1
		}

		if !dec.DoFancyUpsampling {
			cbRow := cbBuf[chromaRow]
			crRow := crBuf[chromaRow]
			for col := 0; col < outputWidth; col++ {
				y := int(yData[col])
				cCol := col >> 1
				if cCol >= len(cbRow) {
					cCol = len(cbRow) - 1
				}
				cb := int(cbRow[cCol])
				cr := int(crRow[cCol])
				r := y + crR[cr]
				g := y + dec.greenContribution(cb, cr, cbG, crG)
				b := y + cbB[cb]
				idx := col * 3
				outputRow[idx] = rl[r+rlColorOffset]
				outputRow[idx+1] = rl[g+rlColorOffset]
				outputRow[idx+2] = rl[b+rlColorOffset]
			}
			return
		}

		// 4:2:0 h2v2 fancy upsampling: single-pass 2D bilinear filter
		// matching libjpeg-turbo h2v2_fancy_upsample exactly.
		// colsum = near*3 + far absorbs vertical weighting.
		// (thiscolsum*3 + neighbor_colsum + bias) >> 4 absorbs horizontal weighting.
		// Bias: +8 for even, +7 for odd (ordered dithering).
		nearRow := chromaRow
		var farRow int
		if (yRow & 1) == 0 {
			farRow = chromaRow - 1
			if farRow < 0 {
				farRow = 0
			}
		} else {
			farRow = chromaRow + 1
			if farRow >= cbBufLen {
				farRow = cbBufLen - 1
			}
		}
		cbNear := cbBuf[nearRow]
		crNear := crBuf[nearRow]
		cbFar := cbBuf[farRow]
		crFar := crBuf[farRow]
		srcLen := len(cbNear)
		dstLen := srcLen * 2
		if dstLen > outputWidth {
			dstLen = outputWidth
		}

		// Pre-compute upsampled chroma via colsum algorithm.
		if cap(dec.vCbRow) < dstLen {
			dec.vCbRow = make([]int, dstLen)
			dec.vCrRow = make([]int, dstLen)
		}
		cbUp := dec.vCbRow[:dstLen]
		crUp := dec.vCrRow[:dstLen]

		// First column
		thisCbCS := int(cbNear[0])*3 + int(cbFar[0])
		thisCrCS := int(crNear[0])*3 + int(crFar[0])
		var nextCbCS, nextCrCS int
		if srcLen > 1 {
			nextCbCS = int(cbNear[1])*3 + int(cbFar[1])
			nextCrCS = int(crNear[1])*3 + int(crFar[1])
		}
		cbUp[0] = (thisCbCS*4 + 8) >> 4
		crUp[0] = (thisCrCS*4 + 8) >> 4
		if dstLen > 1 {
			cbUp[1] = (thisCbCS*3 + nextCbCS + 7) >> 4
			crUp[1] = (thisCrCS*3 + nextCrCS + 7) >> 4
		}
		lastCbCS := thisCbCS
		lastCrCS := thisCrCS
		thisCbCS = nextCbCS
		thisCrCS = nextCrCS

		// Interior columns
		for cCol := 1; cCol < srcLen-1; cCol++ {
			if cCol*2+1 >= dstLen {
				break
			}
			nextCbCS = int(cbNear[cCol+1])*3 + int(cbFar[cCol+1])
			nextCrCS = int(crNear[cCol+1])*3 + int(crFar[cCol+1])
			cbUp[cCol*2] = (thisCbCS*3 + lastCbCS + 8) >> 4
			crUp[cCol*2] = (thisCrCS*3 + lastCrCS + 8) >> 4
			cbUp[cCol*2+1] = (thisCbCS*3 + nextCbCS + 7) >> 4
			crUp[cCol*2+1] = (thisCrCS*3 + nextCrCS + 7) >> 4
			lastCbCS = thisCbCS
			lastCrCS = thisCrCS
			thisCbCS = nextCbCS
			thisCrCS = nextCrCS
		}

		// Last column
		if srcLen > 1 && dstLen >= 2 {
			lastIdx := dstLen - 1
			cbUp[lastIdx-1] = (thisCbCS*3 + lastCbCS + 8) >> 4
			crUp[lastIdx-1] = (thisCrCS*3 + lastCrCS + 8) >> 4
			cbUp[lastIdx] = (thisCbCS*4 + 7) >> 4
			crUp[lastIdx] = (thisCrCS*4 + 7) >> 4
		}

		// Apply color conversion
		for col := 0; col < outputWidth; col++ {
			y := int(yData[col])
			uCol := col
			if uCol >= dstLen {
				uCol = dstLen - 1
			}
			r := y + crR[crUp[uCol]]
			g := y + dec.greenContribution(cbUp[uCol], crUp[uCol], cbG, crG)
			b := y + cbB[cbUp[uCol]]
			idx := col * 3
			outputRow[idx] = rl[r+rlColorOffset]
			outputRow[idx+1] = rl[g+rlColorOffset]
			outputRow[idx+2] = rl[b+rlColorOffset]
		}

	} else if hUpsample {
		chromaRow := yRow
		if chromaRow >= cbBufLen {
			chromaRow = cbBufLen - 1
		}
		cbRow := cbBuf[chromaRow]
		crRow := crBuf[chromaRow]
		for col := 0; col < outputWidth; col++ {
			y := int(yData[col])
			cCol := col >> 1
			cb := int(cbRow[cCol])
			cr := int(crRow[cCol])
			r := y + crR[cr]
			g := y + dec.greenContribution(cb, cr, cbG, crG)
			b := y + cbB[cb]
			idx := col * 3
			outputRow[idx] = rl[r+rlColorOffset]
			outputRow[idx+1] = rl[g+rlColorOffset]
			outputRow[idx+2] = rl[b+rlColorOffset]
		}
	} else if vUpsample {
		chromaRow := yRow >> 1
		if chromaRow >= cbBufLen {
			chromaRow = cbBufLen - 1
		}
		cbRow := cbBuf[chromaRow]
		crRow := crBuf[chromaRow]
		for col := 0; col < outputWidth; col++ {
			y := int(yData[col])
			cb := int(cbRow[col])
			cr := int(crRow[col])
			r := y + crR[cr]
			g := y + dec.greenContribution(cb, cr, cbG, crG)
			b := y + cbB[cb]
			idx := col * 3
			outputRow[idx] = rl[r+rlColorOffset]
			outputRow[idx+1] = rl[g+rlColorOffset]
			outputRow[idx+2] = rl[b+rlColorOffset]
		}
	} else {
		chromaRow := yRow
		if chromaRow >= cbBufLen {
			chromaRow = cbBufLen - 1
		}
		cbRow := cbBuf[chromaRow]
		crRow := crBuf[chromaRow]
		for col := 0; col < outputWidth; col++ {
			y := int(yData[col])
			cb := int(cbRow[col])
			cr := int(crRow[col])
			r := y + crR[cr]
			g := y + dec.greenContribution(cb, cr, cbG, crG)
			b := y + cbB[cb]
			idx := col * 3
			outputRow[idx] = rl[r+rlColorOffset]
			outputRow[idx+1] = rl[g+rlColorOffset]
			outputRow[idx+2] = rl[b+rlColorOffset]
		}
	}
}

func (dec *Decoder) greenContribution(cb, cr int, cbG, crG []int) int {
	if dec.DisableChromaIDCTScaling {
		cbf := float64(cb - 128)
		crf := float64(cr - 128)
		return int(math.Floor(-0.344143*cbf - 0.71414*crf + 0.5))
	}
	return (cbG[cb] + crG[cr]) >> 16
}

func (dec *Decoder) upsampleRGB(outputRow []byte) {
	d := dec.d
	outputWidth := d.OutputWidth

	if cap(dec.vCbRow) < outputWidth {
		dec.vCbRow = make([]int, outputWidth)
	}
	if cap(dec.vCrRow) < outputWidth {
		dec.vCrRow = make([]int, outputWidth)
	}

	green := dec.vCbRow[:outputWidth]
	blue := dec.vCrRow[:outputWidth]
	dec.upsampleComponentToInt(1, dec.rowGroupCtr, green)
	dec.upsampleComponentToInt(2, dec.rowGroupCtr, blue)

	redRow := dec.componentBuf[0][clampIndex(dec.rowGroupCtr, len(dec.componentBuf[0]))]
	for col := 0; col < outputWidth; col++ {
		rCol := col
		if rCol >= len(redRow) {
			rCol = len(redRow) - 1
		}
		idx := col * 3
		outputRow[idx] = redRow[rCol]
		outputRow[idx+1] = byte(green[col])
		outputRow[idx+2] = byte(blue[col])
	}
}

func (dec *Decoder) upsampleComponentToInt(componentIndex, outputRow int, dst []int) {
	d := dec.d
	comp := &d.CompInfo[componentIndex]
	buf := dec.componentBuf[componentIndex]
	outputWidth := len(dst)
	fullH := d.MaxHSampFactor * d.MinDCTHScaledSize
	fullV := d.MaxVSampFactor * d.MinDCTVScaledSize
	compH := comp.HSampFactor * comp.DCHScaledSize
	compV := comp.VSampFactor * comp.DCVScaledSize
	hRatio := ratioOrOne(fullH, compH)
	vRatio := ratioOrOne(fullV, compV)
	hUpsample := hRatio > 1
	vUpsample := vRatio > 1

	if hRatio == 2 && vRatio == 2 && dec.DoFancyUpsampling {
		dec.upsampleH2V2FancyComponentToInt(buf, outputRow, dst)
		return
	}

	srcRowIndex := outputRow
	if vUpsample {
		srcRowIndex = outputRow / vRatio
	}
	srcRow := buf[clampIndex(srcRowIndex, len(buf))]
	for col := 0; col < outputWidth; col++ {
		srcCol := col
		if hUpsample {
			srcCol = col / hRatio
		}
		if srcCol >= len(srcRow) {
			srcCol = len(srcRow) - 1
		}
		dst[col] = int(srcRow[srcCol])
	}
}

func (dec *Decoder) upsampleH2V2FancyComponentToInt(buf [][]byte, outputRow int, dst []int) {
	srcRowIndex := outputRow >> 1
	if srcRowIndex >= len(buf) {
		srcRowIndex = len(buf) - 1
	}
	nearRow := srcRowIndex
	var farRow int
	if (outputRow & 1) == 0 {
		farRow = srcRowIndex - 1
		if farRow < 0 {
			farRow = 0
		}
	} else {
		farRow = srcRowIndex + 1
		if farRow >= len(buf) {
			farRow = len(buf) - 1
		}
	}

	near := buf[nearRow]
	far := buf[farRow]
	srcLen := len(near)
	dstLen := srcLen * 2
	if dstLen > len(dst) {
		dstLen = len(dst)
	}
	if srcLen == 0 || dstLen == 0 {
		return
	}

	thisColsum := int(near[0])*3 + int(far[0])
	var nextColsum int
	if srcLen > 1 {
		nextColsum = int(near[1])*3 + int(far[1])
	}
	dst[0] = (thisColsum*4 + 8) >> 4
	if dstLen > 1 {
		dst[1] = (thisColsum*3 + nextColsum + 7) >> 4
	}
	lastColsum := thisColsum
	thisColsum = nextColsum

	for srcCol := 1; srcCol < srcLen-1; srcCol++ {
		if srcCol*2+1 >= dstLen {
			break
		}
		nextColsum = int(near[srcCol+1])*3 + int(far[srcCol+1])
		dst[srcCol*2] = (thisColsum*3 + lastColsum + 8) >> 4
		dst[srcCol*2+1] = (thisColsum*3 + nextColsum + 7) >> 4
		lastColsum = thisColsum
		thisColsum = nextColsum
	}

	if srcLen > 1 && dstLen >= 2 {
		lastIdx := dstLen - 1
		dst[lastIdx-1] = (thisColsum*3 + lastColsum + 8) >> 4
		dst[lastIdx] = (thisColsum*4 + 7) >> 4
	}

	for col := dstLen; col < len(dst); col++ {
		dst[col] = dst[dstLen-1]
	}
}

func ratioOrOne(full, partial int) int {
	if partial <= 0 || full <= partial {
		return 1
	}
	ratio := full / partial
	if ratio < 1 {
		return 1
	}
	return ratio
}

func clampIndex(idx, length int) int {
	if idx < 0 {
		return 0
	}
	if idx >= length {
		return length - 1
	}
	return idx
}

func (dec *Decoder) buildHuffmanTables() error {
	d := dec.d
	blocksInMCU := d.BlocksInMCU
	dec.dcTables = make([]*huff.DerivedHuffTable, blocksInMCU)
	dec.acTables = make([]*huff.DerivedHuffTable, blocksInMCU)
	dcCache := make(map[int]*huff.DerivedHuffTable)
	acCache := make(map[int]*huff.DerivedHuffTable)

	for blkn := 0; blkn < blocksInMCU; blkn++ {
		ci := d.MCUMembership[blkn]
		comp := d.CurCompInfo[ci]

		dcTblNo := comp.DCTblNo
		if derivedDC, ok := dcCache[dcTblNo]; ok {
			dec.dcTables[blkn] = derivedDC
		} else {
			dcTbl := d.DCHuffTbls[dcTblNo]
			if dcTbl == nil {
				return errors.New("jpeg: missing DC Huffman table")
			}
			derivedDC, err := huff.MakeDerivedHuffTable(convertHuffTable(dcTbl))
			if err != nil {
				return err
			}
			dcCache[dcTblNo] = derivedDC
			dec.dcTables[blkn] = derivedDC
		}

		acTblNo := comp.ACTblNo
		if derivedAC, ok := acCache[acTblNo]; ok {
			dec.acTables[blkn] = derivedAC
		} else {
			acTbl := d.ACHuffTbls[acTblNo]
			if acTbl == nil {
				return errors.New("jpeg: missing AC Huffman table")
			}
			derivedAC, err := huff.MakeDerivedHuffTable(convertHuffTable(acTbl))
			if err != nil {
				return err
			}
			acCache[acTblNo] = derivedAC
			dec.acTables[blkn] = derivedAC
		}
	}
	return nil
}

func (dec *Decoder) buildQuantTables() {
	d := dec.d
	numComps := d.CompsInScan
	dec.quantTables = make([][huff.DCTSize2]int32, numComps)
	for ci := 0; ci < numComps; ci++ {
		comp := d.CurCompInfo[ci]
		qt := d.QuantTbls[comp.QuantTblNo]
		if qt != nil {
			for i := 0; i < huff.DCTSize2; i++ {
				dec.quantTables[ci][i] = int32(qt.QuantVal[i])
			}
		}
	}
}

func (dec *Decoder) buildIDCTTables() {
	numComps := dec.d.CompsInScan
	switch dec.idctMethod {
	case huff.IDCTISlow:
		dec.multTablesISlow = make([]*huff.ISlowMultTable, numComps)
		for ci := 0; ci < numComps; ci++ {
			dec.multTablesISlow[ci] = huff.BuildISlowMultTable(dec.quantTables[ci])
		}
	case huff.IDCTIFast:
		dec.multTablesIFast = make([]*huff.IFASTMultTable, numComps)
		for ci := 0; ci < numComps; ci++ {
			dec.multTablesIFast[ci] = huff.BuildIFastMultTable(dec.quantTables[ci])
		}
	case huff.IDCTFloat:
		dec.multTablesFloat = make([]*huff.FloatMultTable, numComps)
		for ci := 0; ci < numComps; ci++ {
			dec.multTablesFloat[ci] = huff.BuildFloatMultTable(dec.quantTables[ci])
		}
	}
}

func (dec *Decoder) setupColorPipeline() {
	d := dec.d
	info := &color.DecompressInfo{
		OutputWidth:        d.OutputWidth,
		OutputHeight:       d.OutputHeight,
		MaxHSampFactor:     d.MaxHSampFactor,
		MaxVSampFactor:     d.MaxVSampFactor,
		MinDCTHScalSize:    d.MinDCTHScaledSize,
		MinDCTVScalSize:    d.MinDCTVScaledSize,
		NumComponents:      d.NumComponents,
		OutColorComponents: d.OutColorComponents,
		OutputComponents:   d.OutputComponents,
		ProgressiveMode:    d.ProgressiveMode,
		QuantizeColors:     d.QuantizeColors,
		CompsInScan:        d.CompsInScan,
		TotalIMCURows:      d.TotalIMCURows,
		MCUsPerRow:         d.MCUsPerRow,
		LimSe:              d.LimSe,
		BlocksInMCU:        d.BlocksInMCU,
	}
	switch d.JPEGColorSpace {
	case marker.CSGrayScale:
		info.JpegColorSpace = color.JCS_GRAYSCALE
	case marker.CSYCbCr:
		info.JpegColorSpace = color.JCS_YCbCr
	case marker.CSRGB:
		info.JpegColorSpace = color.JCS_RGB
	default:
		info.JpegColorSpace = color.JCS_YCbCr
	}
	switch d.OutColorSpace {
	case marker.CSGrayScale:
		info.OutColorSpace = color.JCS_GRAYSCALE
	case marker.CSRGB:
		info.OutColorSpace = color.JCS_RGB
	default:
		info.OutColorSpace = color.JCS_RGB
	}

	info.CompInfo = make([]color.ComponentInfo, d.NumComponents)
	for ci := 0; ci < d.NumComponents; ci++ {
		src := &d.CompInfo[ci]
		dst := &info.CompInfo[ci]
		dst.ComponentNeeded = src.ComponentNeeded
		dst.ComponentIndex = src.ComponentIndex
		dst.HSampFactor = src.HSampFactor
		dst.VSampFactor = src.VSampFactor
		dst.DCTHScalSize = src.DCHScaledSize
		dst.DCTVScalSize = src.DCVScaledSize
		dst.WidthInBlocks = src.WidthInBlocks
		dst.HeightInBlocks = src.HeightInBlocks
		dst.DownsampledWidth = src.DownsampledWidth
		dst.DownsampledHeight = src.DownsampledHeight
		dst.MCUWidth = src.MCUWidth
		dst.MCUHeight = src.MCUHeight
		dst.MCUBlocks = src.MCUBlocks
		dst.MCUSampleWidth = src.MCUSampleWidth
		dst.LastColWidth = src.LastColWidth
		dst.LastRowHeight = src.LastRowHeight
	}
	info.CurCompInfo = make([]*color.ComponentInfo, d.CompsInScan)
	for ci := 0; ci < d.CompsInScan; ci++ {
		info.CurCompInfo[ci] = &info.CompInfo[d.CurCompInfo[ci].ComponentIndex]
	}

	dec.colorConv = color.NewColorConverter(info)

	dec.componentBuf = make([][][]byte, d.NumComponents)
	for ci := 0; ci < d.NumComponents; ci++ {
		comp := &info.CompInfo[ci]
		rowWidth := info.MCUsPerRow * comp.MCUWidth * comp.DCTHScalSize
		numRows := info.TotalIMCURows * comp.MCUHeight * comp.DCTVScalSize
		flatBuf := make([]byte, numRows*rowWidth)
		dec.componentBuf[ci] = make([][]byte, numRows)
		for r := 0; r < numRows; r++ {
			dec.componentBuf[ci][r] = flatBuf[r*rowWidth : (r+1)*rowWidth]
		}
	}

	dec.allDecoded = false
	dec.rowGroupCtr = d.MaxVSampFactor * d.MinDCTVScaledSize
	dec.rowGroupsAvail = d.MaxVSampFactor * d.MinDCTVScaledSize
}

func convertHuffTable(mt *marker.HuffTable) *huff.HuffmanTable {
	ht := &huff.HuffmanTable{}
	ht.Bits = mt.Bits
	ht.HuffVal = mt.HuffVal
	ht.SentTable = mt.SentTable
	return ht
}

func DecodeToRGB(r io.Reader) (pixels []byte, width, height, components int, err error) {
	dec := New(r)
	w, h, _, _, err := dec.ReadHeader()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	width, height = w, h
	if err := dec.StartDecompress(); err != nil {
		return nil, 0, 0, 0, err
	}
	components = dec.OutputComponents()
	rowStride := width * components
	pixels = make([]byte, height*rowStride)
	scanline := make([]byte, rowStride)
	for y := 0; y < height; y++ {
		n, err := dec.ReadScanlines([][]byte{scanline})
		if err != nil {
			return nil, 0, 0, 0, err
		}
		if n == 0 {
			break
		}
		copy(pixels[y*rowStride:], scanline)
	}
	dec.FinishDecompress()
	return pixels, width, height, components, nil
}
