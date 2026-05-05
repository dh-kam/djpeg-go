// Package color implements JPEG decompression pipeline stages ported from
// IJG libjpeg 9f: color conversion, upsampling, merged upsampling,
// post-processing, coefficient handling, and main buffer control.
package color

// JPEG color space constants (mirrors J_COLOR_SPACE from jpeglib.h).
const (
	JCS_UNKNOWN   = 0
	JCS_GRAYSCALE = 1
	JCS_RGB       = 2
	JCS_YCbCr     = 3
	JCS_CMYK      = 4
	JCS_YCCK      = 5
	JCS_BG_RGB    = 6
	JCS_BG_YCC    = 7
)

// JPEG color transform constants.
const (
	JCT_NONE           = 0
	JCT_SUBTRACT_GREEN = 1
)

// JPEG buffer mode constants.
const (
	JBufPassThru    = 0
	JBufSaveAndPass = 1
	JBufCrankDest   = 2
)

// JPEG scan result constants.
const (
	JPEGRowCompleted  = 3
	JPEGScanCompleted = 4
	JPEGSuspended     = 0
)

// ComponentInfo holds per-component decompression parameters.
type ComponentInfo struct {
	ComponentNeeded    bool
	ComponentIndex     int
	HSampFactor        int
	VSampFactor        int
	DCTHScalSize       int // DCT_h_scaled_size
	DCTVScalSize       int // DCT_v_scaled_size
	QuantTable         *QuantTable
	WidthInBlocks      int
	HeightInBlocks     int
	DownsampledWidth   int
	DownsampledHeight  int
	MCUWidth           int
	MCUHeight          int
	MCUBlocks          int // total blocks per MCU for this component
	MCUSampleWidth     int
	LastColWidth       int
	LastRowHeight      int
}

// QuantTable mirrors JQUANT_TBL.
type QuantTable struct {
	QuantVal [64]int16
}

// DecompressInfo holds the state needed by the color/upsample/coef pipeline.
// This is a simplified version of j_decompress_ptr fields used by these modules.
type DecompressInfo struct {
	// Image dimensions
	OutputWidth     int
	OutputHeight    int

	// Sampling factors
	MaxHSampFactor  int
	MaxVSampFactor  int
	MinDCTHScalSize int
	MinDCTVScalSize int

	// Color space settings
	JpegColorSpace    int
	OutColorSpace     int
	ColorTransform    int
	NumComponents     int
	OutColorComponents int
	OutputComponents  int

	// Component info
	CompInfo []ComponentInfo

	// Range limit table: maps any int to a clamped byte [0..255].
	// Index by (value + MaxJSample + 1) or however the caller sets it up.
	// For 8-bit, typically 576 entries: indices [-256..511] offset by CenterJSample<<RANGE_BITS.
	SampleRangeLimit []byte

	// Quantization flag
	QuantizeColors    bool
	ProgressiveMode   bool
	DoBlockSmoothing  bool

	// Scan parameters
	CompsInScan    int
	CurCompInfo    []*ComponentInfo
	TotalIMCURows  int
	MCUsPerRow     int
	InputIMCURow   int
	OutputIMCURow  int
	InputScanNumber  int
	OutputScanNumber int
	LimSe          int
	BlocksInMCU    int
	Ss             int // spectral selection start
	EoiReached     bool

	// Coefficient bits for progressive
	CoefBits [][]int // [component][DCTSIZE2]

	// Callbacks / sub-controllers (set by higher-level code)
	IDCTFunc func(ci int, compptr *ComponentInfo, coefBlock []int16, outputBuf [][]byte, outputCol int)
}

// maxJSample returns the maximum sample value for 8-bit.
const (
	MaxJSample    = 255
	CenterJSample = 128
	ScaleBits     = 16
	OneHalf       = 1 << (ScaleBits - 1) // 0x8000
	RGBPixelSize  = 3
	RGBRed        = 0
	RGBGreen      = 1
	RGBBlue       = 2
	DCTSize       = 8
	DCTSize2      = 64
	MaxComponents = 10
	MaxCompsInScan = 4
	DMaxBlocksInMCU = 10
)

// fix scales a float by 2^16 for fixed-point arithmetic.
func fix(x float64) int32 {
	return int32(x*65536.0 + 0.5)
}

// RangeLimit clamps v to [0..255] using a lookup table.
// The table is expected to have entries at offsets [MaxJSample+1..MaxJSample+255+MaxJSample]
// so that index = v + MaxJSample + 1 gives the clamped result.
func RangeLimit(table []byte, v int) byte {
	// Standard libjpeg range_limit indexing:
	// table is 5*(MaxJSample+1) entries.
	// The useful region starts at offset MaxJSample+1.
	// index = v + MaxJSample + 1
	idx := v + MaxJSample + 1
	if idx < 0 {
		return 0
	}
	if idx >= len(table) {
		return 255
	}
	return table[idx]
}

// BuildSampleRangeLimit creates the standard range-limit table for 8-bit samples.
// The table maps arbitrary int values to [0..255] with clamping.
func BuildSampleRangeLimit() []byte {
	// Table size: 2*(MaxJSample+1) + CenterJSample
	// = 2*256 + 128 = 640
	// Actually libjpeg allocates 5*(MaxJSAMPLE+1) = 1280 entries,
	// but we only need 2*(MaxJSAMPLE+1) + CENTERJSAMPLE = 640.
	// Simpler: allocate enough for index range [-256..511] = 768 entries,
	// offset so index 0 is at position 256.
	tableSize := 2*(MaxJSample+1) + CenterJSample // 640
	table := make([]byte, tableSize)

	// First segment: [-MaxJSample-1 .. -1] maps to 0 (negative values clamp to 0)
	// These are at indices [0 .. MaxJSample]
	for i := 0; i <= MaxJSample; i++ {
		table[i] = 0
	}

	// Middle segment: [0 .. MaxJSample] maps to itself
	for i := 0; i <= MaxJSample; i++ {
		table[MaxJSample+1+i] = byte(i)
	}

	// Last segment: [MaxJSample+1 .. CenterJSample+MaxJSample] maps to MaxJSample
	for i := MaxJSample + 1; i <= CenterJSample+MaxJSample; i++ {
		table[MaxJSample+1+i] = MaxJSample
	}

	return table
}

// clampByte clamps an int to [0, 255].
func clampByte(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

// copyRow copies numSamples bytes from src to dst.
func copyRow(dst, src []byte) {
	copy(dst, src)
}

// copySampleRows copies num_rows rows from src starting at srcStart
// into dst starting at dstStart.
func copySampleRows(src, dst [][]byte, srcStart, dstStart, numRows, width int) {
	for r := 0; r < numRows; r++ {
		copy(dst[dstStart+r][:width], src[srcStart+r][:width])
	}
}

// roundUp rounds n up to a multiple of m.
func roundUp(n, m int) int {
	return (n + m - 1) / m * m
}
