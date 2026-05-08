package marker

// inputController holds the private state for the input controller.
// Ported from jdinput.c my_input_controller.
type inputController struct {
	// Public fields
	ConsumeInput     func(d *Decompressor) (int, error) // consumes input data
	HasMultipleScans bool
	EOIReached       bool

	// Private fields
	inheaders int // nonzero until first SOS is reached
}

// initialSetup is called once, when first SOS marker is reached.
// Ported from jdinput.c initial_setup().
func initialSetup(d *Decompressor) error {
	// Make sure image isn't bigger than we can handle
	if d.ImageHeight > JPEGMaxDimension || d.ImageWidth > JPEGMaxDimension {
		return ErrImageTooBig
	}

	// Only 8 to 12 bits data precision are supported
	if d.DataPrecision < 8 || d.DataPrecision > 12 {
		return ErrBadPrecision
	}

	// Check that number of components won't exceed internal array sizes
	if d.NumComponents > MaxComponents {
		return ErrComponentCount
	}

	// Compute maximum sampling factors; check factor validity
	d.MaxHSampFactor = 1
	d.MaxVSampFactor = 1
	for ci := 0; ci < d.NumComponents; ci++ {
		compptr := &d.CompInfo[ci]
		if compptr.HSampFactor <= 0 || compptr.HSampFactor > MaxSampFactor ||
			compptr.VSampFactor <= 0 || compptr.VSampFactor > MaxSampFactor {
			return ErrBadSampling
		}
		if compptr.HSampFactor > d.MaxHSampFactor {
			d.MaxHSampFactor = compptr.HSampFactor
		}
		if compptr.VSampFactor > d.MaxVSampFactor {
			d.MaxVSampFactor = compptr.VSampFactor
		}
	}

	// Derive block_size, natural_order, and lim_Se
	if d.IsBaselineFlag || (d.ProgressiveMode && d.CompsInScan > 0) {
		// no pseudo SOS marker
		d.BlockSize = DCTSize
		d.NaturalOrder = NaturalOrder
		d.LimSe = DCTSize2 - 1
	} else {
		switch d.Se {
		case 1*1 - 1:
			d.BlockSize = 1
			d.NaturalOrder = NaturalOrder
			d.LimSe = d.Se
		case 2*2 - 1:
			d.BlockSize = 2
			d.NaturalOrder = NaturalOrder2
			d.LimSe = d.Se
		case 3*3 - 1:
			d.BlockSize = 3
			d.NaturalOrder = NaturalOrder3
			d.LimSe = d.Se
		case 4*4 - 1:
			d.BlockSize = 4
			d.NaturalOrder = NaturalOrder4
			d.LimSe = d.Se
		case 5*5 - 1:
			d.BlockSize = 5
			d.NaturalOrder = NaturalOrder5
			d.LimSe = d.Se
		case 6*6 - 1:
			d.BlockSize = 6
			d.NaturalOrder = NaturalOrder6
			d.LimSe = d.Se
		case 7*7 - 1:
			d.BlockSize = 7
			d.NaturalOrder = NaturalOrder7
			d.LimSe = d.Se
		case 8*8 - 1:
			d.BlockSize = 8
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 9*9 - 1:
			d.BlockSize = 9
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 10*10 - 1:
			d.BlockSize = 10
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 11*11 - 1:
			d.BlockSize = 11
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 12*12 - 1:
			d.BlockSize = 12
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 13*13 - 1:
			d.BlockSize = 13
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 14*14 - 1:
			d.BlockSize = 14
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 15*15 - 1:
			d.BlockSize = 15
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		case 16*16 - 1:
			d.BlockSize = 16
			d.NaturalOrder = NaturalOrder
			d.LimSe = DCTSize2 - 1
		default:
			return ErrBadProgression
		}
	}

	// Initialize DCT scaled sizes to block_size
	d.MinDCTHScaledSize = d.BlockSize
	d.MinDCTVScaledSize = d.BlockSize

	// Compute dimensions of components
	for ci := 0; ci < d.NumComponents; ci++ {
		compptr := &d.CompInfo[ci]
		compptr.DCHScaledSize = d.BlockSize
		compptr.DCVScaledSize = d.BlockSize

		// Size in DCT blocks
		compptr.WidthInBlocks = JDivRoundUp(
			d.ImageWidth*compptr.HSampFactor,
			d.MaxHSampFactor*d.BlockSize,
		)
		compptr.HeightInBlocks = JDivRoundUp(
			d.ImageHeight*compptr.VSampFactor,
			d.MaxVSampFactor*d.BlockSize,
		)

		// Size in samples
		compptr.DownsampledWidth = JDivRoundUp(
			d.ImageWidth*compptr.HSampFactor,
			d.MaxHSampFactor,
		)
		compptr.DownsampledHeight = JDivRoundUp(
			d.ImageHeight*compptr.VSampFactor,
			d.MaxVSampFactor,
		)

		// Mark component needed
		compptr.ComponentNeeded = true
		// Mark no quantization table yet saved
		compptr.QuantTable = nil
	}

	// Compute number of fully interleaved MCU rows
	d.TotalIMCURows = JDivRoundUp(
		d.ImageHeight,
		d.MaxVSampFactor*d.BlockSize,
	)

	// Decide whether file contains multiple scans
	if d.CompsInScan < d.NumComponents || d.ProgressiveMode {
		d.inputCtl.HasMultipleScans = true
	} else {
		d.inputCtl.HasMultipleScans = false
	}

	return nil
}

// perScanSetup does computations needed before processing a JPEG scan.
// Ported from jdinput.c per_scan_setup().
func perScanSetup(d *Decompressor) error {
	if d.CompsInScan == 1 {
		// Noninterleaved (single-component) scan
		compptr := d.CurCompInfo[0]

		// Overall image size in MCUs
		d.MCUsPerRow = compptr.WidthInBlocks
		d.MCURowsInScan = compptr.HeightInBlocks

		// For noninterleaved scan, always one block per MCU
		compptr.MCUWidth = 1
		compptr.MCUHeight = 1
		compptr.MCUBlocks = 1
		compptr.MCUSampleWidth = compptr.DCHScaledSize
		compptr.LastColWidth = 1

		// For noninterleaved scans, last_row_height = number of block rows
		// present in the last iMCU row
		tmp := compptr.HeightInBlocks % compptr.VSampFactor
		if tmp == 0 {
			tmp = compptr.VSampFactor
		}
		compptr.LastRowHeight = tmp

		// Prepare array describing MCU composition
		d.BlocksInMCU = 1
		d.MCUMembership[0] = 0
	} else {
		// Interleaved (multi-component) scan
		if d.CompsInScan <= 0 || d.CompsInScan > MaxCompsInScan {
			return ErrComponentCount
		}

		// Overall image size in MCUs
		d.MCUsPerRow = JDivRoundUp(
			d.ImageWidth,
			d.MaxHSampFactor*d.BlockSize,
		)
		d.MCURowsInScan = d.TotalIMCURows

		d.BlocksInMCU = 0

		for ci := 0; ci < d.CompsInScan; ci++ {
			compptr := d.CurCompInfo[ci]

			compptr.MCUWidth = compptr.HSampFactor
			compptr.MCUHeight = compptr.VSampFactor
			compptr.MCUBlocks = compptr.MCUWidth * compptr.MCUHeight
			compptr.MCUSampleWidth = compptr.MCUWidth * compptr.DCHScaledSize

			// Figure number of non-dummy blocks in last MCU column & row
			tmp := compptr.WidthInBlocks % compptr.MCUWidth
			if tmp == 0 {
				tmp = compptr.MCUWidth
			}
			compptr.LastColWidth = tmp

			tmp = compptr.HeightInBlocks % compptr.MCUHeight
			if tmp == 0 {
				tmp = compptr.MCUHeight
			}
			compptr.LastRowHeight = tmp

			// Prepare array describing MCU composition
			mcublks := compptr.MCUBlocks
			if d.BlocksInMCU+mcublks > MaxBlocksInMCU {
				return ErrBadMCUSize
			}
			for mcublks > 0 {
				d.MCUMembership[d.BlocksInMCU] = ci
				d.BlocksInMCU++
				mcublks--
			}
		}
	}

	return nil
}

// latchQuantTables saves a copy of the Q-table referenced by each component.
// Ported from jdinput.c latch_quant_tables().
func latchQuantTables(d *Decompressor) error {
	for ci := 0; ci < d.CompsInScan; ci++ {
		compptr := d.CurCompInfo[ci]
		// No work if we already saved Q-table for this component
		if compptr.QuantTable != nil {
			continue
		}
		qtblno := compptr.QuantTblNo
		if qtblno < 0 || qtblno >= NumQuantTbls || d.QuantTbls[qtblno] == nil {
			return ErrNoQuantTable
		}
		// Save away the quantization table
		qt := *d.QuantTbls[qtblno]
		compptr.QuantTable = &qt
	}
	return nil
}

// consumeMarkers reads JPEG markers before, between, or after compressed-data scans.
// Returns JPEGReachedSOS, JPEGReachedEOI, JPEGSuspended, or an error.
// Ported from jdinput.c consume_markers().
func consumeMarkers(d *Decompressor) (int, error) {
	ic := d.inputCtl

	if ic.EOIReached {
		return JPEGReachedEOI, nil
	}

	for {
		val, err := readMarkers(d)
		if err != nil {
			return JPEGSuspended, err
		}

		switch val {
		case JPEGReachedSOS:
			if ic.inheaders != 0 { // 1st SOS
				if ic.inheaders == 1 {
					if err := initialSetup(d); err != nil {
						return JPEGSuspended, err
					}
				}
				if d.CompsInScan == 0 { // pseudo SOS marker
					ic.inheaders = 2
					continue
				}
				ic.inheaders = 0
			} else { // 2nd or later SOS marker
				if !ic.HasMultipleScans {
					return JPEGSuspended, ErrEOIExpected
				}
				if d.CompsInScan == 0 { // unexpected pseudo SOS marker
					continue
				}
				// Start the input pass for this scan
				if err := startInputPass(d); err != nil {
					return JPEGSuspended, err
				}
			}
			return val, nil

		case JPEGReachedEOI:
			ic.EOIReached = true
			if ic.inheaders != 0 {
				// Tables-only datastream
				if d.marker.SawSOF {
					return JPEGSuspended, ErrSOFNoSOS
				}
			} else {
				if d.OutputScanNumber > d.InputScanNumber {
					d.OutputScanNumber = d.InputScanNumber
				}
			}
			return val, nil

		default:
			return val, nil
		}
	}
}

// startInputPass initializes the input modules to read a scan.
// Ported from jdinput.c start_input_pass().
func startInputPass(d *Decompressor) error {
	if err := perScanSetup(d); err != nil {
		return err
	}
	if err := latchQuantTables(d); err != nil {
		return err
	}
	// In the full decompressor, entropy and coef start_pass would be called here.
	// For now, just switch consume_input to consume_data (which is not yet implemented).
	return nil
}

// StartInputPass is the exported version of startInputPass.
// It initializes per-scan data structures (MCU layout, quantization tables).
func (d *Decompressor) StartInputPass() error {
	return startInputPass(d)
}

// resetInputController resets state to begin a fresh datastream.
// Ported from jdinput.c reset_input_controller().
func resetInputController(d *Decompressor) {
	ic := d.inputCtl
	ic.ConsumeInput = consumeMarkers
	ic.HasMultipleScans = false
	ic.EOIReached = false
	ic.inheaders = 1

	// Reset marker reader state
	resetMarkerReader(d)

	// Reset progression state
	d.CoefBits = nil
}

// initInputController initializes the input controller module.
// Ported from jdinput.c jinit_input_controller().
func initInputController(d *Decompressor) {
	ic := &inputController{}
	d.inputCtl = ic

	// Initialize method pointers
	ic.ConsumeInput = consumeMarkers
	ic.HasMultipleScans = false
	ic.EOIReached = false
	ic.inheaders = 1
}

// coreOutputDimensions computes output image dimensions and related values.
// Ported from jdinput.c jpeg_core_output_dimensions().
func coreOutputDimensions(d *Decompressor) {
	// Handle scaling by scale_num/scale_denom relative to block_size
	scaleNum := int(d.ScaleNum)
	scaleDenom := int(d.ScaleDenom)
	blockSize := d.BlockSize

	if blockSize == 0 {
		blockSize = DCTSize
	}

	if scaleNum*blockSize <= scaleDenom {
		d.OutputWidth = JDivRoundUp(d.ImageWidth, blockSize)
		d.OutputHeight = JDivRoundUp(d.ImageHeight, blockSize)
		d.MinDCTHScaledSize = 1
		d.MinDCTVScaledSize = 1
	} else {
		// Determine the scale factor
		for scale := 1; scale <= 16; scale++ {
			if scaleNum*blockSize <= scaleDenom*scale {
				d.OutputWidth = JDivRoundUp(d.ImageWidth*scale, blockSize)
				d.OutputHeight = JDivRoundUp(d.ImageHeight*scale, blockSize)
				d.MinDCTHScaledSize = scale
				d.MinDCTVScaledSize = scale
				break
			}
		}
		if d.OutputWidth == 0 || d.OutputHeight == 0 {
			// Default to no scaling if nothing matched
			d.OutputWidth = d.ImageWidth
			d.OutputHeight = d.ImageHeight
			d.MinDCTHScaledSize = blockSize
			d.MinDCTVScaledSize = blockSize
		}
	}

	// Recompute per-component IDCT scaling. IJG 9f scales chroma up in
	// the IDCT when fancy upsampling is enabled so the upsampler can often
	// run at 1:1.
	for ci := 0; ci < d.NumComponents; ci++ {
		compptr := &d.CompInfo[ci]
		hSize := 1
		if !d.RawDataOut {
			threshold := DCTSize / 2
			if d.DoFancyUpsampling {
				threshold = DCTSize
			}
			for d.MinDCTHScaledSize*hSize <= threshold &&
				d.MaxHSampFactor%(compptr.HSampFactor*hSize*2) == 0 {
				hSize *= 2
			}
		}
		compptr.DCHScaledSize = d.MinDCTHScaledSize * hSize

		vSize := 1
		if !d.RawDataOut {
			threshold := DCTSize / 2
			if d.DoFancyUpsampling {
				threshold = DCTSize
			}
			for d.MinDCTVScaledSize*vSize <= threshold &&
				d.MaxVSampFactor%(compptr.VSampFactor*vSize*2) == 0 {
				vSize *= 2
			}
		}
		compptr.DCVScaledSize = d.MinDCTVScaledSize * vSize

		if compptr.DCHScaledSize > compptr.DCVScaledSize*2 {
			compptr.DCHScaledSize = compptr.DCVScaledSize * 2
		} else if compptr.DCVScaledSize > compptr.DCHScaledSize*2 {
			compptr.DCVScaledSize = compptr.DCHScaledSize * 2
		}

		compptr.MCUSampleWidth = compptr.MCUWidth * compptr.DCHScaledSize
		compptr.DownsampledWidth = JDivRoundUp(
			d.ImageWidth*compptr.HSampFactor*compptr.DCHScaledSize,
			d.MaxHSampFactor*d.BlockSize,
		)
		compptr.DownsampledHeight = JDivRoundUp(
			d.ImageHeight*compptr.VSampFactor*compptr.DCVScaledSize,
			d.MaxVSampFactor*d.BlockSize,
		)
	}
}
