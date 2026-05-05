// Package huff provides Huffman decoding, arithmetic decoding, and IDCT
// implementations ported from IJG libjpeg 9f.
//
// This is a pure Go port of:
//   - jdhuff.c    — Huffman entropy decoding (baseline and progressive)
//   - jdarith.c   — Arithmetic entropy decoding
//   - jaricom.c   — Arithmetic coding common tables
//   - jddctmgr.c  — IDCT manager
//   - jidctint.c  — Accurate integer IDCT (ISLOW)
//   - jidctflt.c  — Floating-point IDCT
//   - jidctfst.c  — Fast integer IDCT (IFAST)
package huff

// Constants from IJG libjpeg.

const (
	DCTSize       = 8             // DCTSIZE in IJG
	DCTSize2      = 64            // DCTSIZE2 = 8*8
	MaxCompsInScan = 4            // MAX_COMPS_IN_SCAN
	NumHuffTbls   = 4             // NUM_HFF_TBLS
	NumArithTbls  = 4             // NUM_ARITH_TBLS
	DMaxBlocksInMCU = 10          // D_MAX_BLOCKS_IN_MCU

	// Bit-reading constants
	HuffLookahead = 8             // # of bits of lookahead
	BitBufSize    = 32            // size of buffer in bits

	// Range limit constants (matching IJG libjpeg)
	CenterJSample = 128           // CENTERJSAMPLE
	RangeBits     = 2             // RANGE_BITS
	RangeCenter   = CenterJSample << RangeBits // RANGE_CENTER = 512
	RangeMask     = RangeCenter*2 - 1          // RANGE_MASK = 1023
	RangeSubset   = RangeCenter - CenterJSample // RANGE_SUBSET = 384
	MaxJSample    = 255

	// IDCT method constants (JDCT_*)
	JDCTISlow = 0  // accurate integer
	JDCTIFast = 1  // fast integer
	JDCTFloat = 2  // floating-point
)

// JCOEF is the type for JPEG coefficient values.
type JCOEF = int16

// JCOEFPTR is a pointer into a coefficient block (64 values in zigzag order).
type JCOEFPTR = []JCOEF

// JSAMPLE is an 8-bit output sample.
type JSAMPLE = uint8

// JOCTET is a byte from the JPEG data stream.
type JOCTET = byte

// Block represents an 8x8 block of DCT coefficients in zigzag order.
type Block [DCTSize2]JCOEF

// BlockRow is a row of output samples.
type BlockRow []JSAMPLE

// ComponentInfo holds per-component information needed by the decoder.
type ComponentInfo struct {
	// Quantization table values (64 entries in zigzag order).
	QuantTable [DCTSize2]int32

	// Multiplier table for IDCT (method-dependent; filled by IDCT manager).
	DCTTable interface{} // will be *ISlowMultTable, *IFastMultTable, or *FloatMultTable

	// Table numbers
	DCTblNo int
	ACTblNo int

	// Whether this component is needed for output.
	ComponentNeeded bool

	// Scaled DCT block dimensions.
	DCTHScaledSize int
	DCTVScaledSize int
}

// ISlowMultTable is the multiplier table for the accurate integer IDCT.
type ISlowMultTable [DCTSize2]int32

// IFASTMultTable is the multiplier table for the fast integer IDCT.
type IFASTMultTable [DCTSize2]int32

// FloatMultTable is the multiplier table for the floating-point IDCT.
type FloatMultTable [DCTSize2]float64

// RangeLimitTable provides the range-limiting lookup table for post-IDCT
// sample clamping and level shifting.
//
// This matches IJG's IDCT_range_limit(cinfo) pointer, which is
// sample_range_limit - RANGE_SUBSET, where RANGE_SUBSET = RANGE_CENTER - CENTERJSAMPLE.
//
// The IDCT adds RANGE_CENTER to the descaled result, then indexes this table
// with (value & RANGE_MASK). The table converts from level-shifted values
// to the final unsigned output:
//
//   Index   0..383   : value 0    (for signed values < -128, clamp to black)
//   Index 384..639   : value 0..255 (identity for signed values -128..127)
//   Index 640..1023  : value 255  (for signed values > 127, clamp to white)
//
// Entries 1024..1279 are a copy of 0..255 for safety.
type RangeLimitTable [5 * 256]JSAMPLE

// globalRangeLimitTable is a package-level singleton range-limit table,
// initialized once and reused by all decoders. This avoids allocating
// 1280 bytes (5*256) per decoder instance.
var globalRangeLimitTable *RangeLimitTable

func init() {
	var t RangeLimitTable

	// Index 0..383: clamp to 0 (negative values)
	for i := 0; i < RangeSubset; i++ {
		t[i] = 0
	}

	// Index 384..639: identity mapping (0..255)
	for i := 0; i <= MaxJSample; i++ {
		t[RangeSubset+i] = JSAMPLE(i)
	}

	// Index 640..1023: clamp to 255 (positive overflow)
	for i := RangeSubset + MaxJSample + 1; i <= RangeMask; i++ {
		t[i] = MaxJSample
	}

	// Entries 1024..1279: simple 0..255 copy (safety/fallback)
	for i := 0; i < 256; i++ {
		t[1024+i] = JSAMPLE(i)
	}

	globalRangeLimitTable = &t
}

// NewRangeLimitTable creates and initializes the range-limit table.
// This matches IJG's IDCT_range_limit pointer layout exactly.
// Returns a pointer to a shared singleton; do not modify the returned table.
func NewRangeLimitTable() *RangeLimitTable {
	return globalRangeLimitTable
}

// Clip applies range limiting to a sample value.
// This replaces the range_limit[] lookup in the C code.
func Clip(v int) JSAMPLE {
	// Fast clamp to [0, 255]
	// In the C code, this is done via the range_limit table with RANGE_MASK.
	// The value v is expected to already be biased by RANGE_CENTER.
	// We add the offset for negative-index handling.
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return JSAMPLE(v)
}

// BitReadState holds the persistent bit-reading state (saved across MCUs).
type BitReadState struct {
	GetBuffer uint32 // current bit-extraction buffer
	BitsLeft  int    // # of unused bits in it
}

// SavableState holds entropy decoder state that changes within an MCU
// but must not be updated permanently until MCU completion.
type SavableState struct {
	EOBRUN     uint  // remaining EOBs in EOBRUN
	LastDCVal  [MaxCompsInScan]int // last DC coef for each component
}

// HuffmanTable represents a JPEG Huffman table (DHT marker data).
// This is the "public" table (JHUFF_TBL in IJG).
type HuffmanTable struct {
	Bits   [17]uint8 // bits[1..16] = # of codes of each length; bits[0] unused
	HuffVal [256]uint8 // symbol values in order of increasing code length
	SentTable bool     // whether the table has been output (for encoding)
}

// DerivedHuffTable contains the precomputed decoding data for a Huffman table.
// Ported from d_derived_tbl in jdhuff.c.
type DerivedHuffTable struct {
	// MaxCode[l] = largest code of length l (-1 if none).
	// Index 0 is unused; index 17 is a sentinel (0xFFFFF).
	MaxCode [18]int32

	// ValOffset[l] = huffval[] index for 1st symbol of code length l,
	// minus the minimum code of length l.
	ValOffset [17]int32

	// Link to public Huffman table (needed for symbol lookup).
	Pub *HuffmanTable

	// Lookahead tables: indexed by the next HuffLookahead bits of
	// the input data stream. If the next Huffman code is no more
	// than HuffLookahead bits long, we can obtain its length and
	// the corresponding symbol directly from these tables.
	LookNBits [1 << HuffLookahead]int    // # bits, or 0 if too long
	LookSym   [1 << HuffLookahead]uint8  // symbol, or unused
}

// BitReadWorkingState holds working bit-reading state within an MCU.
type BitReadWorkingState struct {
	NextInputByte []byte // next byte(s) to read from source
	BytesInBuffer int    // # of bytes remaining in source buffer
	GetBuffer     uint32 // current bit-extraction buffer
	BitsLeft      int    // # of unused bits in it
}

// Mask for n rightmost bits.
var bmask = [16]int32{
	0, 0x0001, 0x0003, 0x0007, 0x000F, 0x001F, 0x003F, 0x007F, 0x00FF,
	0x01FF, 0x03FF, 0x07FF, 0x0FFF, 0x1FFF, 0x3FFF, 0x7FFF,
}

// HuffExtend extends a Huffman-decoded value to signed.
// This implements Figure F.12 from the JPEG spec.
func HuffExtend(x int, s int) int {
	if x <= int(bmask[s-1]) {
		return x - int(bmask[s])
	}
	return x
}

// ArithDCContext holds the DC context conditioning state for arithmetic decoding.
type ArithDCContext struct {
	LastDCVal [MaxCompsInScan]int
	DCContext [MaxCompsInScan]int
}

// ArithEntropyState holds the state for arithmetic decoding.
type ArithEntropyState struct {
	C           int32 // C register
	A           int32 // A register
	Ct          int   // bit shift counter
	LastDCVal   [MaxCompsInScan]int
	DCContext   [MaxCompsInScan]int
	RestartsToGo uint

	// Statistics areas
	DCStats [NumArithTbls][]byte
	ACStats [NumArithTbls][]byte

	// Statistics bin for coding with fixed probability 0.5
	FixedBin [4]byte
}

// IDCTFunc is the function signature for an IDCT implementation.
// It takes:
//   - coefBlock: the 8x8 block of dequantized DCT coefficients
//   - quantTable: the quantization multiplier table
//   - outputBuf: the output sample buffer (rows of JSAMPLE)
//   - outputCol: starting column in output rows
//   - rangeLimit: the range-limiting lookup table
type IDCTFunc func(coefBlock []JCOEF, quantTable interface{}, outputBuf []BlockRow, outputCol int, rangeLimit *RangeLimitTable)

// NaturalOrder maps zigzag index to natural (row-major) order.
// This is the standard JPEG zigzag scan order for 8x8 blocks.
var NaturalOrder = [80]int{
	 0,  1,  8, 16,  9,  2,  3, 10,
	17, 24, 32, 25, 18, 11,  4,  5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13,  6,  7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
	// Extra entries to handle corrupted data
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}
