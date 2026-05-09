package marker

import (
	"errors"
	"io"
)

// NewDecompressor creates and initializes a new JPEG decompressor.
// Ported from jdapimin.c jpeg_CreateDecompress().
func NewDecompressor() *Decompressor {
	d := &Decompressor{}

	// Zero out table pointers
	for i := 0; i < NumQuantTbls; i++ {
		d.QuantTbls[i] = nil
	}
	for i := 0; i < NumHuffTbls; i++ {
		d.DCHuffTbls[i] = nil
		d.ACHuffTbls[i] = nil
	}

	d.MarkerList = nil

	// Initialize marker reader
	initMarkerReader(d)

	// Initialize input controller
	initInputController(d)

	d.GlobalState = DStateStart

	return d
}

// ReadHeader reads the start of the JPEG datastream.
// It reads as far as the first SOS marker and saves all tables and parameters.
// Returns JPEGHeaderOK on success, JPEGHeaderTablesOnly for tables-only streams.
// Ported from jdapimin.c jpeg_read_header().
func (d *Decompressor) ReadHeader(requireImage bool) (int, error) {
	if d.GlobalState != DStateStart && d.GlobalState != DStateInHeader {
		return JPEGSuspended, ErrBadState
	}

	retcode, err := d.ConsumeInput()
	if err != nil {
		return retcode, err
	}

	switch retcode {
	case JPEGReachedSOS:
		retcode = JPEGHeaderOK
	case JPEGReachedEOI:
		if requireImage {
			return JPEGSuspended, ErrNoImage
		}
		// Reset to start state. Like libjpeg's jpeg_abort(), this preserves
		// permanent quantization and Huffman tables for abbreviated streams.
		d.Reset()
		retcode = JPEGHeaderTablesOnly
	}

	return retcode, nil
}

// ConsumeInput consumes data in advance of what the decompressor requires.
// Ported from jdapimin.c jpeg_consume_input().
func (d *Decompressor) ConsumeInput() (int, error) {
	var retcode int = JPEGSuspended
	var err error

	switch d.GlobalState {
	case DStateStart:
		// Start-of-datastream actions: reset appropriate modules
		resetInputController(d)
		d.GlobalState = DStateInHeader
		fallthrough

	case DStateInHeader:
		retcode, err = d.inputCtl.ConsumeInput(d)
		if retcode == JPEGReachedSOS {
			// Set up default parameters based on header data
			defaultDecompressParms(d)
			d.GlobalState = DStateReady
		}
		return retcode, err

	case DStateReady:
		return JPEGReachedSOS, nil

	case DStatePreload, DStatePreScan, DStateScanning,
		DStateRawOK, DStateBufImage, DStateBufPost, DStateStopping:
		return d.inputCtl.ConsumeInput(d)

	default:
		return JPEGSuspended, ErrBadState
	}
}

// StartDecompress starts the decompression.
// Ported from jdapistd.c jpeg_start_decompress().
func (d *Decompressor) StartDecompress() error {
	if d.GlobalState == DStateReady {
		// First call: initialize master control
		if err := initMasterDecompress(d); err != nil {
			return err
		}
		if d.BufferedImage {
			d.GlobalState = DStateBufImage
			return nil
		}
		d.GlobalState = DStatePreload
	}

	if d.GlobalState == DStatePreload {
		// If file has multiple scans, absorb them all
		if d.inputCtl.HasMultipleScans {
			for {
				retcode, err := d.inputCtl.ConsumeInput(d)
				if err != nil {
					return err
				}
				if retcode == JPEGSuspended {
					return ErrSuspension
				}
				if retcode == JPEGReachedEOI {
					break
				}
			}
		}
		d.OutputScanNumber = d.InputScanNumber
	} else if d.GlobalState != DStatePreScan {
		return ErrBadState
	}

	// Perform output pass setup
	return d.outputPassSetup()
}

// outputPassSetup sets up for an output pass.
// Ported from jdapistd.c output_pass_setup().
func (d *Decompressor) outputPassSetup() error {
	if d.GlobalState != DStatePreScan {
		// First call: do pass setup
		PrepareForOutputPass(d)
		d.OutputScanline = 0
		d.GlobalState = DStatePreScan
	}

	// Ready for application to drive output pass
	if d.RawDataOut {
		d.GlobalState = DStateRawOK
	} else {
		d.GlobalState = DStateScanning
	}
	return nil
}

// ReadScanlines reads some scanlines of data from the JPEG decompressor.
// Returns the number of scanlines actually read.
// Ported from jdapistd.c jpeg_read_scanlines().
// Note: The actual data reading requires the full decompressor pipeline
// (entropy decoding, IDCT, color conversion, upsampling). This method
// provides the API framework; actual scanline data reading will be
// implemented in the decompressor pipeline modules.
func (d *Decompressor) ReadScanlines(scanlines [][]uint8) (int, error) {
	if d.GlobalState != DStateScanning {
		return 0, ErrBadState
	}
	if d.OutputScanline >= d.OutputHeight {
		return 0, nil
	}

	// In a full implementation, this would call the main controller's
	// process_data method. For now, return 0 to indicate the pipeline
	// is not yet fully connected.
	rowCtr := 0
	d.OutputScanline += rowCtr
	return rowCtr, nil
}

// FinishDecompress finishes JPEG decompression.
// Ported from jdapistd.c jpeg_finish_decompress().
func (d *Decompressor) FinishDecompress() error {
	if (d.GlobalState == DStateScanning || d.GlobalState == DStateRawOK) && !d.BufferedImage {
		// Terminate final pass of non-buffered mode
		if d.OutputScanline < d.OutputHeight {
			return ErrTooLittleData
		}
		FinishOutputPass(d)
		d.GlobalState = DStateStopping
	} else if d.GlobalState == DStateBufImage {
		d.GlobalState = DStateStopping
	} else if d.GlobalState != DStateStopping {
		return ErrBadState
	}

	// Read until EOI
	for !d.inputCtl.EOIReached {
		retcode, err := d.inputCtl.ConsumeInput(d)
		if err != nil {
			return err
		}
		if retcode == JPEGSuspended {
			return ErrSuspension
		}
	}

	// Final cleanup
	d.Reset()
	return nil
}

// InputComplete returns whether the input file has been fully read.
// Ported from jdapimin.c jpeg_input_complete().
func (d *Decompressor) InputComplete() bool {
	if d.GlobalState < DStateStart || d.GlobalState > DStateStopping {
		return false
	}
	return d.inputCtl.EOIReached
}

// HasMultipleScans returns whether the image has more than one scan.
// Ported from jdapimin.c jpeg_has_multiple_scans().
func (d *Decompressor) HasMultipleScans() bool {
	if d.GlobalState < DStateReady || d.GlobalState > DStateStopping {
		return false
	}
	return d.inputCtl.HasMultipleScans
}

// Reset resets the decompressor to initial state for reuse.
// Ported from jcomapi.c jpeg_abort().
func (d *Decompressor) Reset() {
	d.CompInfo = nil
	d.MarkerList = nil
	d.CoefBits = nil
	d.GlobalState = DStateStart
}

// Abort resets state for an aborted decompression operation.
func (d *Decompressor) Abort() {
	d.Reset()
	if d.inputCtl != nil {
		resetInputController(d)
	}
	d.ImageWidth = 0
	d.ImageHeight = 0
	d.NumComponents = 0
	d.JPEGColorSpace = CSUnknown
	d.OutColorSpace = CSUnknown
	d.OutputWidth = 0
	d.OutputHeight = 0
	d.OutColorComponents = 0
	d.OutputComponents = 0
	d.OutputScanline = 0
	d.InputScanNumber = 0
	d.InputIMCURow = 0
	d.OutputScanNumber = 0
	d.OutputIMCURow = 0
	d.DataPrecision = 0
	d.IsBaselineFlag = false
	d.ProgressiveMode = false
	d.ArithCodeFlag = false
	d.RestartInterval = 0
	d.SawJFIFMarker = false
	d.JFIFMajorVersion = 1
	d.JFIFMinorVersion = 1
	d.DensityUnit = 0
	d.XDensity = 1
	d.YDensity = 1
	d.SawAdobeMarker = false
	d.AdobeTransform = 0
	d.ColorTransform = CTNone
	d.CCIR601Sampling = false
}

// SetSource sets the io.Reader source for JPEG data.
func (d *Decompressor) SetSource(r io.Reader) {
	d.Src = r
}

// ReadRestartMarker reads a restart marker. This is a public wrapper
// used by the entropy decoder.
func (d *Decompressor) ReadRestartMarker() error {
	return readRestartMarker(d)
}

// ResyncToRestart is the default resync method.
func (d *Decompressor) ResyncToRestart(desired int) error {
	return resyncToRestart(d, desired)
}

// GetQuantTable returns the quantization table at the given index, or nil.
func (d *Decompressor) GetQuantTable(index int) *QuantTable {
	if index < 0 || index >= NumQuantTbls {
		return nil
	}
	return d.QuantTbls[index]
}

// GetHuffTable returns the Huffman table at the given index.
// isDC selects DC (true) or AC (false) tables.
func (d *Decompressor) GetHuffTable(index int, isDC bool) *HuffTable {
	if index < 0 || index >= NumHuffTbls {
		return nil
	}
	if isDC {
		return d.DCHuffTbls[index]
	}
	return d.ACHuffTbls[index]
}

// Component returns a pointer to the component info for the given index,
// or nil if out of range.
func (d *Decompressor) Component(index int) *ComponentInfo {
	if index < 0 || index >= d.NumComponents {
		return nil
	}
	return &d.CompInfo[index]
}

// IsProgressive returns true if the image uses progressive encoding.
func (d *Decompressor) IsProgressive() bool {
	return d.ProgressiveMode
}

// IsBaselineJPEG returns true if the image is baseline JPEG.
func (d *Decompressor) IsBaselineJPEG() bool {
	return d.IsBaselineFlag
}

// ValidateHeader performs additional validation after header parsing.
// Returns an error if any required tables are missing for decompression.
func (d *Decompressor) ValidateHeader() error {
	if d.NumComponents <= 0 {
		return ErrEmptyImage
	}
	if d.ImageWidth <= 0 || d.ImageHeight <= 0 {
		return ErrEmptyImage
	}
	if d.DataPrecision != 8 {
		return ErrBadPrecision
	}
	for ci := 0; ci < d.NumComponents; ci++ {
		qt := d.CompInfo[ci].QuantTblNo
		if qt < 0 || qt >= NumQuantTbls {
			return ErrNoQuantTable
		}
	}
	return nil
}

// ErrUnsupported is a general error for unsupported features.
var ErrUnsupported = errors.New("jpeg: unsupported feature")
