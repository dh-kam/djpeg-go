package jpeg

import "io"

// Core types ported from IJG libjpeg 9f (jpeglib.h, jpegint.h, jmorecfg.h, jdct.h).
//
// Design notes:
//   - C function pointer fields in structs have been converted to Go interfaces
//     or are embedded directly as function fields on the struct that owns them.
//   - C unions are replaced with Go struct fields or interfaces.
//   - C typedef'd pointer chains (JSAMPROW -> *JSAMPLE, JSAMPARRAY -> *[]JSAMPLE, etc.)
//     are simplified to Go slice types.
//   - The decompression sub-object pointers (master, main, coef, post, inputctl,
//     marker, entropy, idct, upsample, cconvert, cquantize) use interfaces so that
//     concrete implementations can be provided by other packages.

// ---------------------------------------------------------------------------
// Basic sample and coefficient types
// ---------------------------------------------------------------------------

// JSAMPLE is a single sample (pixel element value). 8-bit mode: 0..255.
type JSAMPLE = uint8

// JCOEF is a DCT frequency coefficient (signed 16-bit).
type JCOEF = int16

// JOCTET is a compressed data byte (exactly 8 bits).
type JOCTET = byte

// DCTELEM is the working type for DCT computations. 8-bit mode: int.
type DCTELEM = int

// JSAMPROW is a pointer to one image row of pixel samples.
type JSAMPROW = []JSAMPLE

// JSAMPARRAY is a 2-D sample array (pointer to some rows).
type JSAMPARRAY = []JSAMPROW

// JSAMPIMAGE is a 3-D sample array (top index is color).
type JSAMPIMAGE = []JSAMPARRAY

// JBLOCK is one block of DCT coefficients (DCTSize2 elements).
type JBLOCK = [DCTSize2]JCOEF

// JBLOCKROW is a pointer to one row of coefficient blocks.
type JBLOCKROW = []JBLOCK

// JBLOCKARRAY is a 2-D array of coefficient blocks.
type JBLOCKARRAY = []JBLOCKROW

// JBLOCKIMAGE is a 3-D array of coefficient blocks.
type JBLOCKIMAGE = []JBLOCKARRAY

// JCOEFPTR is a pointer to coefficient data.
type JCOEFPTR = []JCOEF

// ISLOWMultType is the multiplier type for slow integer IDCT.
type ISLOWMultType = int

// IFASTMultType is the multiplier type for fast integer IDCT.
type IFASTMultType = int

// IFASTScaleBits is the number of fractional bits in scale factors for IFAST.
const IFASTScaleBits = 2

// FLOATMultType is the multiplier type for floating-point IDCT.
type FLOATMultType = float64

// ---------------------------------------------------------------------------
// Tables
// ---------------------------------------------------------------------------

// JQuantTbl represents a DCT coefficient quantization table.
type JQuantTbl struct {
	// Quantization step for each coefficient in natural array order
	// (not zigzag order).
	QuantVal [DCTSize2]uint16
	// SentTable is true when table has been output (compression only).
	SentTable bool
}

// JHuffTbl represents a Huffman coding table.
type JHuffTbl struct {
	// Bits[k] = number of symbols with codes of length k bits.
	// Bits[0] is unused.
	Bits [17]uint8
	// HuffVal is the symbols, in order of increasing code length.
	HuffVal [256]uint8
	// SentTable is true when table has been output (compression only).
	SentTable bool
}

// ---------------------------------------------------------------------------
// Component info
// ---------------------------------------------------------------------------

// JPEGComponentInfo holds basic info about one component (color channel).
type JPEGComponentInfo struct {
	// These values are fixed over the whole image.
	ComponentID     int // Identifier for this component (0..255)
	ComponentIndex  int // Its index in SOF or cinfo.CompInfo[]
	HSampFactor     int // Horizontal sampling factor (1..4)
	VSampFactor     int // Vertical sampling factor (1..4)
	QuantTblNo      int // Quantization table selector (0..3)

	// These values may vary between scans.
	DCTblNo int // DC entropy table selector (0..3)
	ACTblNo int // AC entropy table selector (0..3)

	// Computed during compression or decompression startup:
	WidthInBlocks  int // Component's size in DCT blocks (width)
	HeightInBlocks int // Component's size in DCT blocks (height)

	// Size of a DCT block in samples. Values from 1 to 16 are supported.
	DCTHScaledSize int
	DCTVScaledSize int

	// Downsampled dimensions (actual, unpadded).
	DownsampledWidth  int
	DownsampledHeight int

	// ComponentNeeded indicates whether this component is needed for output.
	ComponentNeeded bool

	// Computed before starting a scan of the component.
	MCUWidth        int // Number of blocks per MCU, horizontally
	MCUHeight       int // Number of blocks per MCU, vertically
	MCUBlocks       int // MCUWidth * MCUHeight
	MCUSampleWidth  int // MCU width in samples: MCUWidth * DCTHScaledSize
	LastColWidth    int // Number of non-dummy blocks across in last MCU
	LastRowHeight   int // Number of non-dummy blocks down in last MCU

	// Saved quantization table for component; nil if none yet saved.
	QuantTable *JQuantTbl

	// Private per-component storage for DCT or IDCT subsystem.
	DCTTable interface{}
}

// ---------------------------------------------------------------------------
// Scan info and markers
// ---------------------------------------------------------------------------

// JPEGScanInfo is the script for encoding a multiple-scan file.
type JPEGScanInfo struct {
	CompsInScan    int                       // Number of components in this scan
	ComponentIndex [MaxCompsInScan]int       // Their SOF/comp_info[] indexes
	Ss, Se         int                       // Progressive JPEG spectral selection
	Ah, Al         int                       // Progressive JPEG successive approx
}

// JPEGMarkerStruct represents a saved marker (APPn or COM).
type JPEGMarkerStruct struct {
	Next           *JPEGMarkerStruct // Next in list, or nil
	Marker         uint8             // Marker code: JPEG_COM, or JPEG_APP0+n
	OriginalLength uint              // Number of bytes of data in the file
	DataLength     uint              // Number of bytes of data saved
	Data           []JOCTET          // The data contained in the marker
}

// ---------------------------------------------------------------------------
// Decompression sub-object interfaces
// ---------------------------------------------------------------------------

// Decompressor is the common interface for decompression sub-objects.
// Each sub-module will implement the relevant methods.
type Decompressor interface{}

// DecompressMaster is the interface for the decompression master control module.
type DecompressMaster interface {
	PrepareForOutputPass(cinfo *JPEGDecompress)
	FinishOutputPass(cinfo *JPEGDecompress)
	IsDummyPass() bool
}

// InputController is the interface for the input control module.
type InputController interface {
	ConsumeInput(cinfo *JPEGDecompress) int
	ResetInputController(cinfo *JPEGDecompress)
	StartInputPass(cinfo *JPEGDecompress)
	FinishInputPass(cinfo *JPEGDecompress)
	HasMultipleScans() bool
	SetHasMultipleScans(v bool)
	EOIReached() bool
	SetEOIReached(v bool)
}

// DecompressMainController is the interface for the main buffer control module.
type DecompressMainController interface {
	StartPass(cinfo *JPEGDecompress, mode JBufMode)
	ProcessData(cinfo *JPEGDecompress, outputBuf JSAMPARRAY, outRowCtr *int, outRowsAvail int)
}

// DecompressCoefController is the interface for the coefficient buffer control module.
type DecompressCoefController interface {
	StartInputPass(cinfo *JPEGDecompress)
	ConsumeData(cinfo *JPEGDecompress) int
	StartOutputPass(cinfo *JPEGDecompress)
	DecompressData(cinfo *JPEGDecompress, outputBuf JSAMPIMAGE) int
	CoefArrays() []VirtBArray
}

// DecompressPostController is the interface for the postprocessing module.
type DecompressPostController interface {
	StartPass(cinfo *JPEGDecompress, mode JBufMode)
	PostProcessData(cinfo *JPEGDecompress, inputBuf JSAMPIMAGE,
		inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf JSAMPARRAY, outRowCtr *int, outRowsAvail int)
}

// MarkerReader is the interface for the marker reading & parsing module.
type MarkerReader interface {
	ResetMarkerReader(cinfo *JPEGDecompress)
	ReadMarkers(cinfo *JPEGDecompress) int
	ReadRestartMarker() func(cinfo *JPEGDecompress) bool
	SawSOI() bool
	SetSawSOI(v bool)
	SawSOF() bool
	SetSawSOF(v bool)
	NextRestartNum() int
	SetNextRestartNum(v int)
	DiscardedBytes() uint
	SetDiscardedBytes(v uint)
}

// EntropyDecoder is the interface for the entropy decoding module.
type EntropyDecoder interface {
	StartPass(cinfo *JPEGDecompress)
	DecodeMCU(cinfo *JPEGDecompress, MCUData JBLOCKARRAY) bool
	FinishPass(cinfo *JPEGDecompress)
}

// InverseDCT is the interface for the inverse DCT module.
type InverseDCT interface {
	StartPass(cinfo *JPEGDecompress)
	// InverseDCT performs the IDCT for one component. The concrete
	// implementations are per-component function arrays; we simplify
	// to a single method.
	InverseDCTComponent(cinfo *JPEGDecompress, compptr *JPEGComponentInfo,
		coefBlock JCOEFPTR, outputBuf JSAMPARRAY, outputCol int)
}

// Upsampler is the interface for the upsampling module.
type Upsampler interface {
	StartPass(cinfo *JPEGDecompress)
	Upsample(cinfo *JPEGDecompress, inputBuf JSAMPIMAGE,
		inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf JSAMPARRAY, outRowCtr *int, outRowsAvail int)
	NeedContextRows() bool
}

// ColorDeconverter is the interface for the colorspace conversion module.
type ColorDeconverter interface {
	StartPass(cinfo *JPEGDecompress)
	ColorConvert(cinfo *JPEGDecompress, inputBuf JSAMPIMAGE, inputRow int,
		outputBuf JSAMPARRAY, numRows int)
}

// ColorQuantizer is the interface for the color quantization module.
type ColorQuantizer interface {
	StartPass(cinfo *JPEGDecompress, isPreScan bool)
	ColorQuantize(cinfo *JPEGDecompress, inputBuf JSAMPARRAY, outputBuf JSAMPARRAY, numRows int)
	FinishPass(cinfo *JPEGDecompress)
	NewColorMap(cinfo *JPEGDecompress)
}

// ---------------------------------------------------------------------------
// Virtual array types
// ---------------------------------------------------------------------------

// VirtSArray represents a virtual 2-D sample array.
type VirtSArray struct {
	MemBuffer      JSAMPARRAY // The in-memory buffer
	RowsInArray    int        // Total virtual array height
	SamplesPerRow  int        // Width of array (and of memory buffer)
	MaxAccess      int        // Max rows accessed by access method
	RowsInMem      int        // Height of memory buffer
	RowsPerChunk   int        // Allocation chunk size in mem_buffer
	CurStartRow    int        // First logical row number in the buffer
	FirstUndefRow  int        // Row number of first uninitialized row
	PreZero        bool       // Pre-zero mode requested?
	Dirty          bool       // Do current buffer contents need writing?
	BSOpen         bool       // Is backing-store data valid?
	Next           *VirtSArray // Link to next virtual sarray control block
}

// VirtBArray represents a virtual 2-D coefficient-block array.
type VirtBArray struct {
	MemBuffer      JBLOCKARRAY // The in-memory buffer
	RowsInArray    int         // Total virtual array height
	BlocksPerRow   int         // Width of array (and of memory buffer)
	MaxAccess      int         // Max rows accessed by access method
	RowsInMem      int         // Height of memory buffer
	RowsPerChunk   int         // Allocation chunk size in mem_buffer
	CurStartRow    int         // First logical row number in the buffer
	FirstUndefRow  int         // Row number of first uninitialized row
	PreZero        bool        // Pre-zero mode requested?
	Dirty          bool        // Do current buffer contents need writing?
	BSOpen         bool        // Is backing-store data valid?
	Next           *VirtBArray // Link to next virtual barray control block
}

// ---------------------------------------------------------------------------
// Error manager
// ---------------------------------------------------------------------------

// ErrorManager handles error and trace message output.
type ErrorManager struct {
	// ErrorExit handles fatal errors (does not return to caller).
	ErrorExit func(cinfo *JPEGCommon)
	// EmitMessage conditionally emits a trace or warning message.
	EmitMessage func(cinfo *JPEGCommon, msgLevel int)
	// OutputMessage actually outputs a trace or error message.
	OutputMessage func(cinfo *JPEGCommon)
	// FormatMessage formats a message string for the most recent JPEG error.
	FormatMessage func(cinfo *JPEGCommon, buffer []byte)
	// ResetErrorMgr resets error state variables at start of a new image.
	ResetErrorMgr func(cinfo *JPEGCommon)

	// MsgCode is the message ID code.
	MsgCode int
	// MsgParm holds up to 8 integer parameters or one string parameter.
	MsgParmInt [8]int
	MsgParmStr [JMSGStrParmMax]byte

	// TraceLevel is the max msg_level that will be displayed.
	TraceLevel int
	// NumWarnings is the number of corrupt-data warnings.
	NumWarnings int64

	// Message table pointers.
	JPEGMessageTable   []string
	LastJPEGMessage    int
	AddonMessageTable  []string
	FirstAddonMessage  int
	LastAddonMessage   int
}

// ProgressManager monitors progress of compression/decompression.
type ProgressManager struct {
	ProgressMonitor func(cinfo *JPEGCommon)
	PassCounter     int64 // Work units completed in this pass
	PassLimit       int64 // Total work units in this pass
	CompletedPasses int   // Passes completed so far
	TotalPasses     int   // Total number of passes expected
}

// ---------------------------------------------------------------------------
// Source manager (for decompression)
// ---------------------------------------------------------------------------

// SourceManager manages the data source for decompression.
type SourceManager struct {
	// NextInputByte points to the next byte to read from buffer.
	NextInputByte []JOCTET
	// BytesInBuffer is the number of bytes remaining in buffer.
	// We track this as a read offset; BytesInBuffer = len(NextInputByte) - readOffset.
	// For simplicity, we track the current read position.
	bytesInBuffer int
	nextIdx       int

	// InitSource is called by jpeg_read_header before any data is actually read.
	InitSource func(cinfo *JPEGDecompress)
	// FillInputBuffer is called whenever the buffer is emptied.
	FillInputBuffer func(cinfo *JPEGDecompress) bool
	// SkipInputData skips data.
	SkipInputData func(cinfo *JPEGDecompress, numBytes int64)
	// ResyncToRestart is for error recovery in the presence of RST markers.
	ResyncToRestart func(cinfo *JPEGDecompress, desired int) bool
	// TermSource is called by jpeg_finish_decompress after all data has been read.
	TermSource func(cinfo *JPEGDecompress)
}

// BytesInBuffer returns the number of unread bytes in the source buffer.
func (s *SourceManager) BytesInBuffer() int {
	return len(s.NextInputByte) - s.nextIdx
}

// GetByte reads and returns the next byte from the source buffer.
// Returns -1 on underflow (caller should call FillInputBuffer).
func (s *SourceManager) GetByte() int {
	if s.nextIdx >= len(s.NextInputByte) {
		return -1
	}
	b := int(s.NextInputByte[s.nextIdx])
	s.nextIdx++
	return b
}

// Consume removes n bytes from the front of the buffer.
func (s *SourceManager) Consume(n int) {
	s.nextIdx += n
}

// ResetBuffer resets the internal buffer tracking after a FillInputBuffer call.
func (s *SourceManager) ResetBuffer(data []JOCTET) {
	s.NextInputByte = data
	s.nextIdx = 0
}

// ---------------------------------------------------------------------------
// Memory manager (Go GC-based simplification)
// ---------------------------------------------------------------------------

// MemoryManager provides allocation functions. In the Go port, allocations are
// simply Go slices and maps; the pool-based allocation from C is replaced by
// Go's garbage collector. This struct exists primarily as a compatibility shim.
type MemoryManager struct {
	// MaxMemoryToUse is the limit on memory allocation for this JPEG object.
	MaxMemoryToUse int64
	// MaxAllocChunk is the maximum allocation request accepted.
	MaxAllocChunk int64
}

// ---------------------------------------------------------------------------
// Main structs
// ---------------------------------------------------------------------------

// JPEGCommon represents the common fields shared between compression and
// decompression master structs. In the C code this was embedded via the
// jpeg_common_fields macro. Here it is a standalone struct.
type JPEGCommon struct {
	Err      *ErrorManager     // Error handler module
	Mem      *MemoryManager    // Memory manager module
	Progress *ProgressManager  // Progress monitor, or nil
	ClientData interface{}     // Available for use by application
	IsDecompressor bool        // So common code can tell which is which
	GlobalState    int         // For checking call sequence validity
}

// JPEGDecompress is the master record for a decompression instance.
// This is THE main struct, ported from jpeg_decompress_struct.
type JPEGDecompress struct {
	// Embedded common fields
	JPEGCommon

	// Source of compressed data
	Src *SourceManager

	// Basic description of image (filled in by jpeg_read_header).
	ImageWidth    int          // Nominal image width (from SOF marker)
	ImageHeight   int          // Nominal image height
	NumComponents int          // Number of color components in JPEG image
	JPEGColorSpace JColorSpace // Colorspace of JPEG image

	// Decompression processing parameters (set before jpeg_start_decompress).
	OutColorSpace   JColorSpace // Colorspace for output
	ScaleNum        uint        // Fraction by which to scale image (numerator)
	ScaleDenom      uint        // Fraction by which to scale image (denominator)
	OutputGamma     float64     // Image gamma wanted in output
	BufferedImage   bool        // True = multiple output passes
	RawDataOut      bool        // True = downsampled data wanted
	DCTMethod       JDCTMethod  // IDCT algorithm selector
	DoFancyUpsampling  bool    // True = apply fancy upsampling
	DoBlockSmoothing   bool    // True = apply interblock smoothing
	QuantizeColors     bool    // True = colormapped output wanted
	DitherMode         JDitherMode // Type of color dithering to use
	TwoPassQuantize    bool    // True = use two-pass color quantization
	DesiredNumColors   int     // Max number of colors to use in created colormap
	Enable1PassQuant   bool    // Enable future use of 1-pass quantizer
	EnableExternalQuant bool   // Enable future use of external colormap
	Enable2PassQuant   bool    // Enable future use of 2-pass quantizer

	// Description of actual output image (computed by jpeg_start_decompress).
	OutputWidth        int // Scaled image width
	OutputHeight       int // Scaled image height
	OutColorComponents int // Number of color components in out_color_space
	OutputComponents   int // Number of color components returned (1 when quantizing)
	RecOutbufHeight    int // Min recommended height of scanline buffer

	// When quantizing colors, the output colormap.
	ActualNumberOfColors int        // Number of entries in use
	Colormap             JSAMPARRAY // The colormap as a 2-D pixel array

	// State variables.
	OutputScanline   int // Row index of next scanline to be read (0..output_height-1)
	InputScanNumber  int // Number of SOS markers seen so far
	InputIMCURow     int // Number of iMCU rows completed (input side)
	OutputScanNumber int // Nominal scan number being displayed
	OutputIMCURow    int // Number of iMCU rows read (output side)

	// CoefBits[c][i] indicates the precision with which component c's
	// DCT coefficient i (in zigzag order) is known. Nil for non-progressive.
	CoefBits [][]int

	// Internal JPEG parameters.
	// Quantization and Huffman tables carried forward across datastreams.
	QuantTblPtrs  [NumQuantTbls]*JQuantTbl
	DCHuffTblPtrs [NumHuffTbls]*JHuffTbl
	ACHuffTblPtrs [NumHuffTbls]*JHuffTbl

	DataPrecision int // Bits of precision in image data

	CompInfo []JPEGComponentInfo // CompInfo[i] describes component i'th in SOF

	IsBaseline     bool // True if Baseline SOF0 encountered
	ProgressiveMode bool // True if SOFn specifies progressive mode
	ArithCode      bool // True = arithmetic coding, False = Huffman

	ArithDC_L [NumArithTbls]uint8 // L values for DC arith-coding tables
	ArithDC_U [NumArithTbls]uint8 // U values for DC arith-coding tables
	ArithAC_K [NumArithTbls]uint8 // Kx values for AC arith-coding tables

	RestartInterval uint // MCUs per restart interval, or 0 for no restart

	// Data from optional markers.
	SawJFIFMarker    bool
	JFIFMajorVersion uint8
	JFIFMinorVersion uint8
	DensityUnit      uint8
	XDensity         uint16
	YDensity         uint16
	SawAdobeMarker   bool
	AdobeTransform   uint8
	ColorTransform   JColorTransform
	CCIR601Sampling  bool

	MarkerList *JPEGMarkerStruct // Head of list of saved markers

	// Computed during decompression startup.
	MaxHSampFactor int // Largest h_samp_factor
	MaxVSampFactor int // Largest v_samp_factor
	MinDCTHScaledSize int // Smallest DCT_h_scaled_size of any component
	MinDCTVScaledSize int // Smallest DCT_v_scaled_size of any component
	TotalIMCURows    int // Number of iMCU rows in image

	SampleRangeLimit []JSAMPLE // Table for fast range-limiting

	// Valid during any one scan.
	CompsInScan   int                          // Number of JPEG components in this scan
	CurCompInfo   [MaxCompsInScan]*JPEGComponentInfo // CurCompInfo[i] describes i'th in SOS
	MCUsPerRow    int // Number of MCUs across the image
	MCURowsInScan int // Number of MCU rows in the image
	BlocksInMCU   int // Number of DCT blocks per MCU
	MCUMembership [DMaxBlocksInMCU]int // MCU_membership[i] is index in CurCompInfo
	Ss, Se, Ah, Al int // Progressive JPEG parameters for scan

	// Derived from Se of first SOS marker.
	BlockSize    int   // The basic DCT block size: 1..16
	NaturalOrder []int // Natural-order position array for entropy decode
	LimSe        int   // Min(Se, DCTSize2-1) for entropy decode

	UnreadMarker int // Either zero or the code of a JPEG marker not yet processed

	// Links to decompression sub-objects (interfaces for module implementations).
	Master    DecompressMaster
	Main      DecompressMainController
	Coef      DecompressCoefController
	Post      DecompressPostController
	InputCtl  InputController
	Marker    MarkerReader
	Entropy   EntropyDecoder
	IDCT      InverseDCT
	Upsample  Upsampler
	CConvert  ColorDeconverter
	CQuantize ColorQuantizer
}

// Reader returns an io.Reader-compatible interface for the source manager's
// current buffer position. This is a convenience for the Go port.
func (d *JPEGDecompress) Reader() io.Reader {
	return &decompressReader{d: d}
}

// decompressReader wraps JPEGDecompress as an io.Reader.
type decompressReader struct {
	d *JPEGDecompress
}

func (r *decompressReader) Read(p []byte) (n int, err error) {
	src := r.d.Src
	if src == nil {
		return 0, io.EOF
	}
	for n < len(p) {
		if src.nextIdx >= len(src.NextInputByte) {
			if src.FillInputBuffer != nil {
				if !src.FillInputBuffer(r.d) {
					return n, io.EOF
				}
			} else {
				return n, io.EOF
			}
		}
		avail := len(src.NextInputByte) - src.nextIdx
		want := len(p) - n
		toCopy := avail
		if toCopy > want {
			toCopy = want
		}
		copy(p[n:], src.NextInputByte[src.nextIdx:src.nextIdx+toCopy])
		src.nextIdx += toCopy
		n += toCopy
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Natural order tables (from jutils.c)
// ---------------------------------------------------------------------------

// JPEGNaturalOrder maps zigzag order to natural order for 8x8 blocks.
// Extra entries at the end (all 63) prevent wild stores in the Huffman decoder.
var JPEGNaturalOrder = [DCTSize2 + 16]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}

// JPEGNaturalOrder7 is the natural order table for 7x7 blocks.
var JPEGNaturalOrder7 = [7*7 + 16]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 14, 21, 28, 35,
	42, 49, 50, 43, 36, 29, 22, 30,
	37, 44, 51, 52, 45, 38, 46, 53,
	54,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}

// JPEGNaturalOrder6 is the natural order table for 6x6 blocks.
var JPEGNaturalOrder6 = [6*6 + 16]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 41, 34, 27,
	20, 13, 21, 28, 35, 42, 43, 36,
	29, 37, 44, 45,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}

// JPEGNaturalOrder5 is the natural order table for 5x5 blocks.
var JPEGNaturalOrder5 = [5*5 + 16]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 12,
	19, 26, 33, 34, 27, 20, 28, 35,
	36,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}

// JPEGNaturalOrder4 is the natural order table for 4x4 blocks.
var JPEGNaturalOrder4 = [4*4 + 16]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 25, 18, 11, 19, 26, 27,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}

// JPEGNaturalOrder3 is the natural order table for 3x3 blocks.
var JPEGNaturalOrder3 = [3*3 + 16]int{
	0, 1, 8, 16, 9, 2, 10, 17,
	18,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}

// JPEGNaturalOrder2 is the natural order table for 2x2 blocks.
var JPEGNaturalOrder2 = [2*2 + 16]int{
	0, 1, 8, 9,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}
