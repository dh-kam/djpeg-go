package marker

// decompMaster holds private state for the master decompression controller.
// Ported from jdmaster.c my_decomp_master.
type decompMaster struct {
	PassNumber          int
	UsingMergedUpsample bool
	IsDummyPass         bool

	// Saved references to initialized quantizer modules
	// (for color quantization mode switching)
}

// prepareRangeLimitTable allocates and fills in the sample_range_limit table.
// Ported from jdmaster.c prepare_range_limit_table().
func prepareRangeLimitTable(d *Decompressor) {
	const (
		maxJSample  = 255
		rangeCenter = (maxJSample + 1) / 2 // 128
	)

	// Total table size: entries from -rangeCenter to maxJSample + rangeCenter
	tableSize := 2*(maxJSample+1) + maxJSample + 1
	table := make([]uint8, tableSize)

	// First segment: limit[x] = 0 for x < 0
	// (already zero from make)

	// Offset so table[0] corresponds to index -rangeCenter
	// We'll store the slice starting from index rangeCenter
	offset := rangeCenter
	d.SampleRangeLimit = table[offset:]

	// Main part: limit[x] = x for 0..maxJSample
	for i := 0; i <= maxJSample; i++ {
		d.SampleRangeLimit[i] = uint8(i)
	}

	// End: limit[x] = maxJSample for x > maxJSample
	for i := maxJSample + 1; i <= maxJSample+rangeCenter; i++ {
		d.SampleRangeLimit[i] = maxJSample
	}
}

// CalcOutputDimensions computes output image dimensions for full decompression.
// Ported from jdmaster.c jpeg_calc_output_dimensions().
func CalcOutputDimensions(d *Decompressor) {
	// Compute core output dimensions
	coreOutputDimensions(d)

	// Report number of components in selected colorspace
	switch d.OutColorSpace {
	case CSGrayScale:
		d.OutColorComponents = 1
	case CSRGB, CSBGRGB:
		d.OutColorComponents = RGBPixelSize
	default:
		i := 0
		for ci := 0; ci < d.NumComponents; ci++ {
			if d.CompInfo[ci].ComponentNeeded {
				i++
			}
		}
		d.OutColorComponents = i
	}

	if d.QuantizeColors {
		d.OutputComponents = 1
	} else {
		d.OutputComponents = d.OutColorComponents
	}

	// Recommended output buffer height
	d.RecOutbufHeight = 1
}

// masterSelection selects and initializes decompression modules.
// Ported from jdmaster.c master_selection().
func masterSelection(d *Decompressor) error {
	master := d.master

	// Check precision
	if d.DataPrecision != 8 {
		return ErrBadPrecision
	}

	// Initialize dimensions
	CalcOutputDimensions(d)
	prepareRangeLimitTable(d)

	// Sanity check on image dimensions
	if d.OutputHeight <= 0 || d.OutputWidth <= 0 || d.OutColorComponents <= 0 {
		return ErrEmptyImage
	}

	// Check for width overflow
	samplesPerRow := int64(d.OutputWidth) * int64(d.OutColorComponents)
	if samplesPerRow > 0x7FFFFFFF {
		return ErrWidthOverflow
	}

	// Initialize private state
	master.PassNumber = 0
	master.UsingMergedUpsample = false
	master.IsDummyPass = false

	return nil
}

// initMasterDecompress initializes the master decompression controller.
// Ported from jdmaster.c jinit_master_decompress().
func initMasterDecompress(d *Decompressor) error {
	d.master = &decompMaster{}
	return masterSelection(d)
}

// PrepareForOutputPass sets up for an output pass.
// Ported from jdmaster.c prepare_for_output_pass().
func PrepareForOutputPass(d *Decompressor) {
	master := d.master
	_ = master
	// In the full decompressor this would set up IDCT, entropy, color conversion,
	// upsampling, post-processing, and main controller modules.
	// For our purposes (marker parsing + input control), this is a no-op
	// that can be extended later.
}

// FinishOutputPass finishes up at end of an output pass.
// Ported from jdmaster.c finish_output_pass().
func FinishOutputPass(d *Decompressor) {
	// No-op for now; extended later with full decompressor
}

// defaultDecompressParms sets default decompression parameters.
// Ported from jdapimin.c default_decompress_parms().
func defaultDecompressParms(d *Decompressor) {
	switch d.NumComponents {
	case 1:
		d.JPEGColorSpace = CSGrayScale
		d.OutColorSpace = CSGrayScale

	case 3:
		cid0 := d.CompInfo[0].ComponentID
		cid1 := d.CompInfo[1].ComponentID
		cid2 := d.CompInfo[2].ComponentID

		if cid0 == 0x01 && cid1 == 0x02 && cid2 == 0x03 {
			d.JPEGColorSpace = CSYCbCr
		} else if cid0 == 0x01 && cid1 == 0x22 && cid2 == 0x23 {
			d.JPEGColorSpace = CSBGYCC
		} else if cid0 == 0x52 && cid1 == 0x47 && cid2 == 0x42 {
			d.JPEGColorSpace = CSRGB // ASCII 'R', 'G', 'B'
		} else if cid0 == 0x72 && cid1 == 0x67 && cid2 == 0x62 {
			d.JPEGColorSpace = CSBGRGB // ASCII 'r', 'g', 'b'
		} else if d.SawJFIFMarker {
			d.JPEGColorSpace = CSYCbCr
		} else if d.SawAdobeMarker {
			switch d.AdobeTransform {
			case 0:
				d.JPEGColorSpace = CSRGB
			case 1:
				d.JPEGColorSpace = CSYCbCr
			default:
				d.JPEGColorSpace = CSYCbCr
			}
		} else {
			d.JPEGColorSpace = CSYCbCr
		}
		d.OutColorSpace = CSRGB

	case 4:
		cid0 := d.CompInfo[0].ComponentID
		cid1 := d.CompInfo[1].ComponentID
		cid2 := d.CompInfo[2].ComponentID
		cid3 := d.CompInfo[3].ComponentID

		if cid0 == 0x01 && cid1 == 0x02 && cid2 == 0x03 && cid3 == 0x04 {
			d.JPEGColorSpace = CSYCCK
		} else if cid0 == 0x43 && cid1 == 0x4D && cid2 == 0x59 && cid3 == 0x4B {
			d.JPEGColorSpace = CSCMYK // ASCII 'C', 'M', 'Y', 'K'
		} else if d.SawAdobeMarker {
			switch d.AdobeTransform {
			case 0:
				d.JPEGColorSpace = CSCMYK
			case 2:
				d.JPEGColorSpace = CSYCCK
			default:
				d.JPEGColorSpace = CSYCCK
			}
		} else {
			d.JPEGColorSpace = CSCMYK
		}
		d.OutColorSpace = CSCMYK

	default:
		d.JPEGColorSpace = CSUnknown
		d.OutColorSpace = CSUnknown
	}

	// Set defaults for other decompression parameters
	blockSize := d.BlockSize
	if blockSize == 0 {
		blockSize = DCTSize
	}
	d.ScaleNum = uint(blockSize)
	d.ScaleDenom = uint(blockSize)
	d.OutputGamma = 1.0
	d.BufferedImage = false
	d.RawDataOut = false
	d.DCTMethod = DCTISlow
	d.DoFancyUpsampling = true
	d.DoBlockSmoothing = true
	d.QuantizeColors = false
	d.DitherMode = DitherFS
	d.TwoPassQuantize = true
	d.DesiredNumColors = 256
	d.Enable1PassQuant = false
	d.EnableExternalQuant = false
	d.Enable2PassQuant = false
}
