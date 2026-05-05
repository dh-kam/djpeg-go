// Package marker provides JPEG marker parsing and decompression input control,
// ported from IJG libjpeg 9f (jdmarker.c, jdinput.c, jdapimin.c, jdapistd.c, jdmaster.c).
package marker

import (
	"errors"
	"io"
)

// JPEG marker codes.
const (
	M_SOF0  = 0xC0
	M_SOF1  = 0xC1
	M_SOF2  = 0xC2
	M_SOF3  = 0xC3
	M_SOF5  = 0xC5
	M_SOF6  = 0xC6
	M_SOF7  = 0xC7
	M_JPG   = 0xC8
	M_SOF9  = 0xC9
	M_SOF10 = 0xCA
	M_SOF11 = 0xCB
	M_SOF13 = 0xCD
	M_SOF14 = 0xCE
	M_SOF15 = 0xCF
	M_DHT   = 0xC4
	M_DAC   = 0xCC
	M_RST0  = 0xD0
	M_RST1  = 0xD1
	M_RST2  = 0xD2
	M_RST3  = 0xD3
	M_RST4  = 0xD4
	M_RST5  = 0xD5
	M_RST6  = 0xD6
	M_RST7  = 0xD7
	M_SOI   = 0xD8
	M_EOI   = 0xD9
	M_SOS   = 0xDA
	M_DQT   = 0xDB
	M_DNL   = 0xDC
	M_DRI   = 0xDD
	M_DHP   = 0xDE
	M_EXP   = 0xDF
	M_APP0  = 0xE0
	M_APP1  = 0xE1
	M_APP2  = 0xE2
	M_APP3  = 0xE3
	M_APP4  = 0xE4
	M_APP5  = 0xE5
	M_APP6  = 0xE6
	M_APP7  = 0xE7
	M_APP8  = 0xE8
	M_APP9  = 0xE9
	M_APP10 = 0xEA
	M_APP11 = 0xEB
	M_APP12 = 0xEC
	M_APP13 = 0xED
	M_APP14 = 0xEE
	M_APP15 = 0xEF
	M_JPG0  = 0xF0
	M_JPG8  = 0xF8
	M_JPG13 = 0xFD
	M_COM   = 0xFE
	M_TEM   = 0x01
)

// Constants from jpeglib.h.
const (
	DCTSize          = 8
	DCTSize2         = 64
	NumQuantTbls     = 4
	NumHuffTbls      = 4
	NumArithTbls     = 16
	MaxCompsInScan   = 4
	MaxSampFactor    = 4
	MaxBlocksInMCU   = 10
	MaxComponents    = 10
	JPEGMaxDimension = 65500
	RGBPixelSize     = 3
)

// Color space enumeration.
type ColorSpace int

const (
	CSUnknown   ColorSpace = iota
	CSGrayScale            // monochrome
	CSRGB                  // red/green/blue, standard RGB (sRGB)
	CSYCbCr                // Y/Cb/Cr (also known as YUV)
	CSCMYK                 // C/M/Y/K
	CSYCCK                 // Y/Cb/Cr/K
	CSBGRGB                // big gamut red/green/blue
	CSBGYCC                // big gamut Y/Cb/Cr
)

// Color transform enumeration.
type ColorTransform int

const (
	CTNone          ColorTransform = 0
	CTSubtractGreen ColorTransform = 1
)

// DCT method enumeration.
type DCTMethod int

const (
	DCTISlow DCTMethod = iota
	DCTIFast
	DCTFloat
)

// Dither mode enumeration.
type DitherMode int

const (
	DitherNone DitherMode = iota
	DitherOrdered
	DitherFS
)

// Return codes for marker reading / consume_input.
const (
	JPEGSuspended        = 0
	JPEGReachedSOS       = 1
	JPEGReachedEOI       = 2
	JPEGHeaderOK         = 3
	JPEGHeaderTablesOnly = 4
	JPEGRowCompleted     = 5
)

// Decompressor states (DSTATE_xxx).
const (
	DStateStart    = 201
	DStateInHeader = 202
	DStateReady    = 203
	DStatePreload  = 204
	DStatePreScan  = 205
	DStateScanning = 206
	DStateRawOK    = 207
	DStateBufImage = 208
	DStateBufPost  = 209
	DStateStopping = 210
)

// Quantization table: 64 values in natural (row-major) order.
type QuantTable struct {
	QuantVal  [DCTSize2]uint16
	SentTable bool
}

// Huffman table.
type HuffTable struct {
	Bits      [17]uint8 // Bits[k] = # of symbols with code length k; Bits[0] unused
	HuffVal   [256]uint8
	SentTable bool
}

// ComponentInfo holds basic info about one component (color channel).
type ComponentInfo struct {
	ComponentID    int // identifier for this component (0..255)
	ComponentIndex int // its index in SOF / CompInfo[]
	HSampFactor    int // horizontal sampling factor (1..4)
	VSampFactor    int // vertical sampling factor (1..4)
	QuantTblNo     int // quantization table selector (0..3)

	// Per-scan fields
	DCTblNo int // DC entropy table selector (0..3)
	ACTblNo int // AC entropy table selector (0..3)

	// Computed during startup
	WidthInBlocks     int
	HeightInBlocks    int
	DCHScaledSize     int
	DCVScaledSize     int
	DownsampledWidth  int
	DownsampledHeight int
	ComponentNeeded   bool

	// Per-scan computed
	MCUWidth       int
	MCUHeight      int
	MCUBlocks      int
	MCUSampleWidth int
	LastColWidth   int
	LastRowHeight  int

	// Saved quantization table for component
	QuantTable *QuantTable

	// Private per-component DCT/IDCT storage
	DCTTable interface{}
}

// SavedMarker represents a saved APPn or COM marker.
type SavedMarker struct {
	Next           *SavedMarker
	Marker         uint8
	OriginalLength uint
	DataLength     uint
	Data           []byte
}

// Decompressor holds all state for JPEG decompression.
type Decompressor struct {
	// Basic image description (filled in by ReadHeader)
	ImageWidth     int
	ImageHeight    int
	NumComponents  int
	JPEGColorSpace ColorSpace

	// Decompression parameters (set before StartDecompress)
	OutColorSpace       ColorSpace
	ScaleNum            uint
	ScaleDenom          uint
	OutputGamma         float64
	BufferedImage       bool
	RawDataOut          bool
	DCTMethod           DCTMethod
	DoFancyUpsampling   bool
	DoBlockSmoothing    bool
	QuantizeColors      bool
	DitherMode          DitherMode
	TwoPassQuantize     bool
	DesiredNumColors    int
	Enable1PassQuant    bool
	EnableExternalQuant bool
	Enable2PassQuant    bool

	// Computed output description
	OutputWidth        int
	OutputHeight       int
	OutColorComponents int
	OutputComponents   int
	RecOutbufHeight    int
	ActualNumColors    int

	// State
	OutputScanline   int
	InputScanNumber  int
	InputIMCURow     int
	OutputScanNumber int
	OutputIMCURow    int

	// Internal parameters
	DataPrecision   int
	CompInfo        []ComponentInfo
	IsBaselineFlag  bool
	ProgressiveMode bool
	ArithCodeFlag   bool

	ArithDCL [NumArithTbls]uint8
	ArithDCU [NumArithTbls]uint8
	ArithACK [NumArithTbls]uint8

	RestartInterval uint

	// Optional marker data
	SawJFIFMarker    bool
	JFIFMajorVersion uint8
	JFIFMinorVersion uint8
	DensityUnit      uint8
	XDensity         uint16
	YDensity         uint16
	SawAdobeMarker   bool
	AdobeTransform   uint8
	ColorTransform   ColorTransform
	CCIR601Sampling  bool

	MarkerList *SavedMarker

	// Computed during startup
	MaxHSampFactor    int
	MaxVSampFactor    int
	MinDCTHScaledSize int
	MinDCTVScaledSize int
	TotalIMCURows     int
	SampleRangeLimit  []uint8

	// Per-scan fields
	CompsInScan    int
	CurCompInfo    [MaxCompsInScan]*ComponentInfo
	MCUsPerRow     int
	MCURowsInScan  int
	BlocksInMCU    int
	MCUMembership  [MaxBlocksInMCU]int
	Ss, Se, Ah, Al int

	BlockSize    int
	NaturalOrder []int
	LimSe        int

	UnreadMarker int

	// Global state machine state
	GlobalState int

	// Tables
	QuantTbls  [NumQuantTbls]*QuantTable
	DCHuffTbls [NumHuffTbls]*HuffTable
	ACHuffTbls [NumHuffTbls]*HuffTable

	// Coefficient bits for progressive
	CoefBits [][]int

	// marker reader state
	marker *markerReader
	// input controller state
	inputCtl *inputController
	// master controller state
	master *decompMaster
	// Source reader
	Src io.Reader
}

// Errors.
var (
	ErrNoSOI             = errors.New("jpeg: not a JPEG file: missing SOI marker")
	ErrSOIDuplicate      = errors.New("jpeg: duplicate SOI marker")
	ErrSOFDuplicate      = errors.New("jpeg: duplicate SOF marker")
	ErrEmptyImage        = errors.New("jpeg: empty JPEG image (DNL not supported)")
	ErrBadLength         = errors.New("jpeg: marker length is invalid")
	ErrBadHuffTable      = errors.New("jpeg: bogus Huffman table definition")
	ErrBadDHTIndex       = errors.New("jpeg: bogus DHT index")
	ErrBadDQTIndex       = errors.New("jpeg: bogus DQT index")
	ErrBadPrecision      = errors.New("jpeg: unsupported data precision")
	ErrBadSampling       = errors.New("jpeg: bad sampling factor")
	ErrBadComponentID    = errors.New("jpeg: bad component ID in SOS")
	ErrSOFUnsupported    = errors.New("jpeg: unsupported SOF marker type")
	ErrUnknownMarker     = errors.New("jpeg: unknown marker")
	ErrSOFBeforeSOS      = errors.New("jpeg: SOF encountered before SOS")
	ErrSOFBeforeLSE      = errors.New("jpeg: SOF encountered before LSE")
	ErrImageTooBig       = errors.New("jpeg: image too large")
	ErrBadMCUSize        = errors.New("jpeg: MCU size too large")
	ErrComponentCount    = errors.New("jpeg: too many components")
	ErrEOIExpected       = errors.New("jpeg: expected EOI")
	ErrNoQuantTable      = errors.New("jpeg: missing quantization table")
	ErrBadState          = errors.New("jpeg: bad decompressor state")
	ErrTooLittleData     = errors.New("jpeg: too little data")
	ErrNoImage           = errors.New("jpeg: no image in file")
	ErrBadProgression    = errors.New("jpeg: bad progression parameters")
	ErrConversionNotImpl = errors.New("jpeg: color conversion not implemented")
	ErrNotImpl           = errors.New("jpeg: not implemented")
	ErrWidthOverflow     = errors.New("jpeg: output width overflow")
	ErrSOFNoSOS          = errors.New("jpeg: SOF encountered but no SOS before EOI")
	ErrSuspension        = errors.New("jpeg: data source suspension")
	ErrDACIndex          = errors.New("jpeg: bogus DAC index")
	ErrDACValue          = errors.New("jpeg: bogus DAC value")
)

// JDivRoundUp divides a by b, rounding up (integer ceiling division).
func JDivRoundUp(a, b int) int {
	return (a + b - 1) / b
}
