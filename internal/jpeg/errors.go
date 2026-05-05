package jpeg

import "fmt"

// Error handling ported from IJG libjpeg 9f (jerror.h, jerror.c, cderror.h).
//
// In the C library, errors are handled through function pointers in the
// jpeg_error_mgr struct, which allows applications to override behavior.
// In Go, we use the error interface and panic/recover for fatal errors,
// while warnings are logged through the ErrorManager callbacks.

// ---------------------------------------------------------------------------
// Error codes
// ---------------------------------------------------------------------------

// Error/message codes from jerror.h.
const (
	MsgNoMessage = iota // "Bogus message code %d" - must be first

	// Fatal errors (alphabetical by name).
	ErrBadAlignType   // "ALIGN_TYPE is wrong, please fix"
	ErrBadAllocChunk  // "MAX_ALLOC_CHUNK is wrong, please fix"
	ErrBadBufferMode  // "Bogus buffer control mode"
	ErrBadComponentID // "Invalid component ID %d in SOS"
	ErrBadCropSpec    // "Invalid crop request"
	ErrBadDCTCoef     // "DCT coefficient out of range"
	ErrBadDCTSize     // "DCT scaled block size %dx%d not supported"
	ErrBadDropSampling
	ErrBadHuffTable   // "Bogus Huffman table definition"
	ErrBadInColorspace // "Bogus input colorspace"
	ErrBadJColorSpace // "Bogus JPEG colorspace"
	ErrBadLength      // "Bogus marker length"
	ErrBadLibVersion  // "Wrong JPEG library version"
	ErrBadMCUSize     // "Sampling factors too large for interleaved scan"
	ErrBadPoolID      // "Invalid memory pool code %d"
	ErrBadPrecision   // "Unsupported JPEG data precision %d"
	ErrBadProgression // "Invalid progressive parameters"
	ErrBadProgScript  // "Invalid progressive parameters at scan script entry %d"
	ErrBadSampling    // "Bogus sampling factors"
	ErrBadScanScript  // "Invalid scan script at entry %d"
	ErrBadState       // "Improper call to JPEG library in state %d"
	ErrBadStructSize  // "JPEG parameter struct mismatch"
	ErrBadVirtualAccess // "Bogus virtual array access"
	ErrBufferSize     // "Buffer passed to JPEG library is too small"
	ErrCantSuspend    // "Suspension not allowed here"
	ErrCCIR601NotImpl // "CCIR601 sampling not implemented yet"
	ErrComponentCount // "Too many color components"
	ErrConversionNotImpl // "Unsupported color conversion request"
	ErrDACIndex       // "Bogus DAC index %d"
	ErrDACValue       // "Bogus DAC value 0x%x"
	ErrDHTIndex       // "Bogus DHT index %d"
	ErrDQTIndex       // "Bogus DQT index %d"
	ErrEmptyImage     // "Empty JPEG image (DNL not supported)"
	ErrEMSRead        // "Read from EMS failed"
	ErrEMSWrite       // "Write to EMS failed"
	ErrEOIExpected    // "Didn't expect more than one scan"
	ErrFileRead       // "Input file read error"
	ErrFileWrite      // "Output file write error"
	ErrFractSampleNotImpl // "Fractional sampling not implemented yet"
	ErrHuffClenOutOfBounds
	ErrHuffMissingCode
	ErrImageTooBig    // "Maximum supported image dimension is %u pixels"
	ErrInputEmpty     // "Empty input file"
	ErrInputEOF       // "Premature end of input file"
	ErrMismatchedQuantTable
	ErrMissingData    // "Scan script does not transmit all data"
	ErrModeChange     // "Invalid color quantization mode change"
	ErrNotImpl        // "Not implemented yet"
	ErrNotCompiled    // "Requested feature was omitted at compile time"
	ErrNoArithTable   // "Arithmetic table 0x%02x was not defined"
	ErrNoBackingStore // "Backing store not supported"
	ErrNoHuffTable    // "Huffman table 0x%02x was not defined"
	ErrNoImage        // "JPEG datastream contains no image"
	ErrNoQuantTable   // "Quantization table 0x%02x was not defined"
	ErrNoSOI          // "Not a JPEG file: starts with 0x%02x 0x%02x"
	ErrOutOfMemory    // "Insufficient memory (case %d)"
	ErrQuantComponents
	ErrQuantFewColors
	ErrQuantManyColors
	ErrSOFBefore     // "Invalid JPEG file structure: %s before SOF"
	ErrSOFDuplicate  // "Invalid JPEG file structure: two SOF markers"
	ErrSOFNoSOS      // "Invalid JPEG file structure: missing SOS marker"
	ErrSOFUnsupported // "Unsupported JPEG process: SOF type 0x%02x"
	ErrSOIDuplicate  // "Invalid JPEG file structure: two SOI markers"
	ErrTFileCreate   // "Failed to create temporary file %s"
	ErrTFileRead     // "Read failed on temporary file"
	ErrTFileSeek     // "Seek failed on temporary file"
	ErrTFileWrite    // "Write failed on temporary file"
	ErrTooLittleData // "Application transferred too few scanlines"
	ErrUnknownMarker // "Unsupported marker type 0x%02x"
	ErrVirtualBug    // "Virtual array controller messed up"
	ErrWidthOverflow // "Image too wide for this implementation"
	ErrXMSRead       // "Read from XMS failed"
	ErrXMSWrite      // "Write to XMS failed"
	MsgCopyright     // JCOPYRIGHT
	MsgVersion       // JVERSION

	// Trace messages.
	Trc16BitTables
	TrcAdobe
	TrcAPP0
	TrcAPP14
	TrcDAC
	TrcDHT
	TrcDQT
	TrcDRI
	TrcEMSClose
	TrcEMSOpen
	TrcEOI
	TrcHuffBits
	TrcJFIF
	TrcJFIFBadThumbnailSize
	TrcJFIFExtension
	TrcJFIFThumbnail
	TrcMiscMarker
	TrcParmlessMarker
	TrcQuantVals
	TrcQuant3NColors
	TrcQuantNColors
	TrcQuantSelected
	TrcRecoveryAction
	TrcRST
	TrcSmoothNotImpl
	TrcSOF
	TrcSOFComponent
	TrcSOI
	TrcSOS
	TrcSOSComponent
	TrcSOSParams
	TrcTFileClose
	TrcTFileOpen
	TrcThumbJPEG
	TrcThumbPalette
	TrcThumbRGB
	TrcUnknownIDs
	TrcXMSClose
	TrcXMSOpen

	// Warning messages.
	WrnAdobeXform
	WrnArithBadCode
	WrnBogusProgression
	WrnExtraneousData
	WrnHitMarker
	WrnHuffBadCode
	WrnJFIFMajor
	WrnJPEGEOF
	WrnMustResync
	WrnNotSequential
	WrnTooMuchData

	MsgLastMsgCode // Must be last entry
)

// Add-on message codes from cderror.h (starting at 1000).
const (
	MsgFirstAddonCode = 1000

	// Format-related errors (from cderror.h)
	ErrBadCMAPFile       = 1100 + iota // "Color map file is invalid"
	ErrTooManyColors                   // "Output file format cannot handle %d colormap entries"
	ErrUngetcFailed                    // "ungetc failed"
	ErrUnknownFormat                   // "Unrecognized input file format"
	ErrUnsupportedFormat               // "Unsupported output file format"

	MsgLastAddonCode
)

// ---------------------------------------------------------------------------
// Standard message table
// ---------------------------------------------------------------------------

// JPEGStdMessageTable contains the standard error and trace message strings.
var JPEGStdMessageTable = []string{
	MsgNoMessage:           "Bogus message code %d",
	ErrBadAlignType:        "ALIGN_TYPE is wrong, please fix",
	ErrBadAllocChunk:       "MAX_ALLOC_CHUNK is wrong, please fix",
	ErrBadBufferMode:       "Bogus buffer control mode",
	ErrBadComponentID:      "Invalid component ID %d in SOS",
	ErrBadCropSpec:         "Invalid crop request",
	ErrBadDCTCoef:          "DCT coefficient out of range",
	ErrBadDCTSize:          "DCT scaled block size %dx%d not supported",
	ErrBadDropSampling:     "Component index %d: mismatching sampling ratio %d:%d, %d:%d, %c",
	ErrBadHuffTable:        "Bogus Huffman table definition",
	ErrBadInColorspace:     "Bogus input colorspace",
	ErrBadJColorSpace:      "Bogus JPEG colorspace",
	ErrBadLength:           "Bogus marker length",
	ErrBadLibVersion:       "Wrong JPEG library version: library is %d, caller expects %d",
	ErrBadMCUSize:          "Sampling factors too large for interleaved scan",
	ErrBadPoolID:           "Invalid memory pool code %d",
	ErrBadPrecision:        "Unsupported JPEG data precision %d",
	ErrBadProgression:      "Invalid progressive parameters Ss=%d Se=%d Ah=%d Al=%d",
	ErrBadProgScript:       "Invalid progressive parameters at scan script entry %d",
	ErrBadSampling:         "Bogus sampling factors",
	ErrBadScanScript:       "Invalid scan script at entry %d",
	ErrBadState:            "Improper call to JPEG library in state %d",
	ErrBadStructSize:       "JPEG parameter struct mismatch: library thinks size is %u, caller expects %u",
	ErrBadVirtualAccess:    "Bogus virtual array access",
	ErrBufferSize:          "Buffer passed to JPEG library is too small",
	ErrCantSuspend:         "Suspension not allowed here",
	ErrCCIR601NotImpl:      "CCIR601 sampling not implemented yet",
	ErrComponentCount:      "Too many color components: %d, max %d",
	ErrConversionNotImpl:   "Unsupported color conversion request",
	ErrDACIndex:            "Bogus DAC index %d",
	ErrDACValue:            "Bogus DAC value 0x%x",
	ErrDHTIndex:            "Bogus DHT index %d",
	ErrDQTIndex:            "Bogus DQT index %d",
	ErrEmptyImage:          "Empty JPEG image (DNL not supported)",
	ErrEMSRead:             "Read from EMS failed",
	ErrEMSWrite:            "Write to EMS failed",
	ErrEOIExpected:         "Didn't expect more than one scan",
	ErrFileRead:            "Input file read error",
	ErrFileWrite:           "Output file write error --- out of disk space?",
	ErrFractSampleNotImpl:  "Fractional sampling not implemented yet",
	ErrHuffClenOutOfBounds: "Huffman code size table out of bounds",
	ErrHuffMissingCode:     "Missing Huffman code table entry",
	ErrImageTooBig:         "Maximum supported image dimension is %u pixels",
	ErrInputEmpty:          "Empty input file",
	ErrInputEOF:            "Premature end of input file",
	ErrMismatchedQuantTable: "Cannot transcode due to multiple use of quantization table %d",
	ErrMissingData:         "Scan script does not transmit all data",
	ErrModeChange:          "Invalid color quantization mode change",
	ErrNotImpl:             "Not implemented yet",
	ErrNotCompiled:         "Requested feature was omitted at compile time",
	ErrNoArithTable:        "Arithmetic table 0x%02x was not defined",
	ErrNoBackingStore:      "Backing store not supported",
	ErrNoHuffTable:         "Huffman table 0x%02x was not defined",
	ErrNoImage:             "JPEG datastream contains no image",
	ErrNoQuantTable:        "Quantization table 0x%02x was not defined",
	ErrNoSOI:               "Not a JPEG file: starts with 0x%02x 0x%02x",
	ErrOutOfMemory:         "Insufficient memory (case %d)",
	ErrQuantComponents:     "Cannot quantize more than %d color components",
	ErrQuantFewColors:      "Cannot quantize to fewer than %d colors",
	ErrQuantManyColors:     "Cannot quantize to more than %d colors",
	ErrSOFBefore:           "Invalid JPEG file structure: %s before SOF",
	ErrSOFDuplicate:        "Invalid JPEG file structure: two SOF markers",
	ErrSOFNoSOS:            "Invalid JPEG file structure: missing SOS marker",
	ErrSOFUnsupported:      "Unsupported JPEG process: SOF type 0x%02x",
	ErrSOIDuplicate:        "Invalid JPEG file structure: two SOI markers",
	ErrTFileCreate:         "Failed to create temporary file %s",
	ErrTFileRead:           "Read failed on temporary file",
	ErrTFileSeek:           "Seek failed on temporary file",
	ErrTFileWrite:          "Write failed on temporary file --- out of disk space?",
	ErrTooLittleData:       "Application transferred too few scanlines",
	ErrUnknownMarker:       "Unsupported marker type 0x%02x",
	ErrVirtualBug:          "Virtual array controller messed up",
	ErrWidthOverflow:       "Image too wide for this implementation",
	ErrXMSRead:             "Read from XMS failed",
	ErrXMSWrite:            "Write to XMS failed",
	MsgCopyright:           JCopyright,
	MsgVersion:             JVersion,
	Trc16BitTables:         "Caution: quantization tables are too coarse for baseline JPEG",
	TrcAdobe:               "Adobe APP14 marker: version %d, flags 0x%04x 0x%04x, transform %d",
	TrcAPP0:                "Unknown APP0 marker (not JFIF), length %u",
	TrcAPP14:               "Unknown APP14 marker (not Adobe), length %u",
	TrcDAC:                 "Define Arithmetic Table 0x%02x: 0x%02x",
	TrcDHT:                 "Define Huffman Table 0x%02x",
	TrcDQT:                 "Define Quantization Table %d  precision %d",
	TrcDRI:                 "Define Restart Interval %u",
	TrcEMSClose:            "Freed EMS handle %u",
	TrcEMSOpen:             "Obtained EMS handle %u",
	TrcEOI:                 "End Of Image",
	TrcHuffBits:            "        %3d %3d %3d %3d %3d %3d %3d %3d",
	TrcJFIF:                "JFIF APP0 marker: version %d.%02d, density %dx%d  %d",
	TrcJFIFBadThumbnailSize: "Warning: thumbnail image size does not match data length %u",
	TrcJFIFExtension:       "JFIF extension marker: type 0x%02x, length %u",
	TrcJFIFThumbnail:       "    with %d x %d thumbnail image",
	TrcMiscMarker:          "Miscellaneous marker 0x%02x, length %u",
	TrcParmlessMarker:      "Unexpected marker 0x%02x",
	TrcQuantVals:           "        %4u %4u %4u %4u %4u %4u %4u %4u",
	TrcQuant3NColors:       "Quantizing to %d = %d*%d*%d colors",
	TrcQuantNColors:        "Quantizing to %d colors",
	TrcQuantSelected:       "Selected %d colors for quantization",
	TrcRecoveryAction:      "At marker 0x%02x, recovery action %d",
	TrcRST:                 "RST%d",
	TrcSmoothNotImpl:       "Smoothing not supported with nonstandard sampling ratios",
	TrcSOF:                 "Start Of Frame 0x%02x: width=%u, height=%u, components=%d",
	TrcSOFComponent:        "    Component %d: %dhx%dv q=%d",
	TrcSOI:                 "Start of Image",
	TrcSOS:                 "Start Of Scan: %d components",
	TrcSOSComponent:        "    Component %d: dc=%d ac=%d",
	TrcSOSParams:           "  Ss=%d, Se=%d, Ah=%d, Al=%d",
	TrcTFileClose:          "Closed temporary file %s",
	TrcTFileOpen:           "Opened temporary file %s",
	TrcThumbJPEG:           "JFIF extension marker: JPEG-compressed thumbnail image, length %u",
	TrcThumbPalette:        "JFIF extension marker: palette thumbnail image, length %u",
	TrcThumbRGB:            "JFIF extension marker: RGB thumbnail image, length %u",
	TrcUnknownIDs:          "Unrecognized component IDs %d %d %d, assuming YCbCr",
	TrcXMSClose:            "Freed XMS handle %u",
	TrcXMSOpen:             "Obtained XMS handle %u",
	WrnAdobeXform:          "Unknown Adobe color transform code %d",
	WrnArithBadCode:        "Corrupt JPEG data: bad arithmetic code",
	WrnBogusProgression:    "Inconsistent progression sequence for component %d coefficient %d",
	WrnExtraneousData:      "Corrupt JPEG data: %u extraneous bytes before marker 0x%02x",
	WrnHitMarker:           "Corrupt JPEG data: premature end of data segment",
	WrnHuffBadCode:         "Corrupt JPEG data: bad Huffman code",
	WrnJFIFMajor:           "Warning: unknown JFIF revision number %d.%02d",
	WrnJPEGEOF:             "Premature end of JPEG file",
	WrnMustResync:          "Corrupt JPEG data: found marker 0x%02x instead of RST%d",
	WrnNotSequential:       "Invalid SOS parameters for sequential JPEG",
	WrnTooMuchData:         "Application transferred too many scanlines",
}

// ---------------------------------------------------------------------------
// JPEGError type
// ---------------------------------------------------------------------------

// JPEGError represents an error from the JPEG library.
type JPEGError struct {
	Code    int
	Message string
	Params  [8]int
}

// Error returns the error message string.
func (e *JPEGError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("JPEG error code %d", e.Code)
}

// NewError creates a JPEGError with the given code and parameters.
func NewError(code int, params ...int) *JPEGError {
	msg := FormatMessage(code, params...)
	return &JPEGError{
		Code:    code,
		Message: msg,
	}
}

// FormatMessage returns a formatted message string for the given error code
// and integer parameters.
func FormatMessage(code int, params ...int) string {
	if code < 0 || code >= len(JPEGStdMessageTable) {
		return fmt.Sprintf("Bogus message code %d", code)
	}
	tmpl := JPEGStdMessageTable[code]
	p := [8]int{}
	for i, v := range params {
		if i >= 8 {
			break
		}
		p[i] = v
	}
	return fmt.Sprintf(tmpl,
		p[0], p[1], p[2], p[3], p[4], p[5], p[6], p[7])
}

// ---------------------------------------------------------------------------
// Error reporting helpers (replace the C ERREXIT/WARNMS/TRACEMS macros)
// ---------------------------------------------------------------------------

// ErrExit signals a fatal error. In Go, we panic with a JPEGError.
func ErrExit(cinfo *JPEGCommon, code int, params ...int) {
	msg := FormatMessage(code, params...)
	if cinfo != nil && cinfo.Err != nil && cinfo.Err.ErrorExit != nil {
		cinfo.Err.MsgCode = code
		for i, v := range params {
			if i >= 8 {
				break
			}
			cinfo.Err.MsgParmInt[i] = v
		}
		cinfo.Err.ErrorExit(cinfo)
		// If ErrorExit returns (non-standard), fall through to panic.
	}
	panic(&JPEGError{Code: code, Message: msg})
}

// ErrExitDecompress signals a fatal error on a decompression context.
func ErrExitDecompress(d *JPEGDecompress, code int, params ...int) {
	ErrExit(&d.JPEGCommon, code, params...)
}

// WarnMS emits a warning message (non-fatal).
func WarnMS(cinfo *JPEGCommon, code int, params ...int) {
	if cinfo != nil && cinfo.Err != nil && cinfo.Err.EmitMessage != nil {
		cinfo.Err.MsgCode = code
		for i, v := range params {
			if i >= 8 {
				break
			}
			cinfo.Err.MsgParmInt[i] = v
		}
		cinfo.Err.EmitMessage(cinfo, -1)
	}
}

// WarnMSDecompress emits a warning message on a decompression context.
func WarnMSDecompress(d *JPEGDecompress, code int, params ...int) {
	WarnMS(&d.JPEGCommon, code, params...)
}

// TraceMS emits a trace message at the given level.
func TraceMS(cinfo *JPEGCommon, level int, code int, params ...int) {
	if cinfo != nil && cinfo.Err != nil && cinfo.Err.EmitMessage != nil {
		cinfo.Err.MsgCode = code
		for i, v := range params {
			if i >= 8 {
				break
			}
			cinfo.Err.MsgParmInt[i] = v
		}
		cinfo.Err.EmitMessage(cinfo, level)
	}
}

// ---------------------------------------------------------------------------
// Standard error manager setup (equivalent to jpeg_std_error)
// ---------------------------------------------------------------------------

// StdError initializes an ErrorManager with the standard methods.
func StdError() *ErrorManager {
	err := &ErrorManager{
		TraceLevel:        0,
		NumWarnings:       0,
		MsgCode:           0,
		JPEGMessageTable:  JPEGStdMessageTable,
		LastJPEGMessage:   MsgLastMsgCode - 1,
		AddonMessageTable: nil,
		FirstAddonMessage: 0,
		LastAddonMessage:  0,
	}

	err.ErrorExit = stdErrorExit
	err.EmitMessage = stdEmitMessage
	err.OutputMessage = stdOutputMessage
	err.FormatMessage = stdFormatMessage
	err.ResetErrorMgr = stdResetErrorMgr

	return err
}

// stdErrorExit is the default error exit handler.
func stdErrorExit(cinfo *JPEGCommon) {
	if cinfo.Err.OutputMessage != nil {
		cinfo.Err.OutputMessage(cinfo)
	}
	panic(NewError(cinfo.Err.MsgCode, cinfo.Err.MsgParmInt[:]...))
}

// stdEmitMessage is the default emit_message handler.
func stdEmitMessage(cinfo *JPEGCommon, msgLevel int) {
	if msgLevel < 0 {
		// Warning
		if cinfo.Err.NumWarnings == 0 || cinfo.Err.TraceLevel >= 3 {
			if cinfo.Err.OutputMessage != nil {
				cinfo.Err.OutputMessage(cinfo)
			}
		}
		cinfo.Err.NumWarnings++
	} else {
		// Trace
		if cinfo.Err.TraceLevel >= msgLevel {
			if cinfo.Err.OutputMessage != nil {
				cinfo.Err.OutputMessage(cinfo)
			}
		}
	}
}

// stdOutputMessage is the default output_message handler.
func stdOutputMessage(cinfo *JPEGCommon) {
	var buf [JMSGLengthMax]byte
	// In Go we just format to string and write to stderr via fmt.
	// Applications can override this.
	if cinfo.Err.FormatMessage != nil {
		cinfo.Err.FormatMessage(cinfo, buf[:])
		// Find the null terminator
		msg := ""
		for i, b := range buf {
			if b == 0 {
				msg = string(buf[:i])
				break
			}
		}
		if msg == "" {
			msg = string(buf[:])
		}
		fmt.Println(msg)
	}
}

// stdFormatMessage is the default format_message handler.
func stdFormatMessage(cinfo *JPEGCommon, buffer []byte) {
	msgCode := cinfo.Err.MsgCode
	msgText := ""

	if msgCode > 0 && msgCode <= cinfo.Err.LastJPEGMessage {
		if msgCode < len(cinfo.Err.JPEGMessageTable) {
			msgText = cinfo.Err.JPEGMessageTable[msgCode]
		}
	} else if cinfo.Err.AddonMessageTable != nil &&
		msgCode >= cinfo.Err.FirstAddonMessage &&
		msgCode <= cinfo.Err.LastAddonMessage {
		idx := msgCode - cinfo.Err.FirstAddonMessage
		if idx < len(cinfo.Err.AddonMessageTable) {
			msgText = cinfo.Err.AddonMessageTable[idx]
		}
	}

	if msgText == "" {
		cinfo.Err.MsgParmInt[0] = msgCode
		if len(cinfo.Err.JPEGMessageTable) > 0 {
			msgText = cinfo.Err.JPEGMessageTable[0]
		}
	}

	// Format and copy to buffer
	formatted := fmt.Sprintf(msgText,
		cinfo.Err.MsgParmInt[0], cinfo.Err.MsgParmInt[1],
		cinfo.Err.MsgParmInt[2], cinfo.Err.MsgParmInt[3],
		cinfo.Err.MsgParmInt[4], cinfo.Err.MsgParmInt[5],
		cinfo.Err.MsgParmInt[6], cinfo.Err.MsgParmInt[7])

	copy(buffer, formatted)
	if len(formatted) < len(buffer) {
		buffer[len(formatted)] = 0
	}
}

// stdResetErrorMgr is the default reset handler.
func stdResetErrorMgr(cinfo *JPEGCommon) {
	cinfo.Err.NumWarnings = 0
	cinfo.Err.MsgCode = 0
}
