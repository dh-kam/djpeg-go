package testutil

import (
	"encoding/binary"
)

// MinimalJPEGBuilder constructs minimal valid JPEG byte sequences for testing.
// It generates proper SOI, DQT, SOF0, DHT, SOS markers with standard Huffman
// tables, supporting configurable dimensions, color mode, and subsampling.
type MinimalJPEGBuilder struct {
	width       int
	height      int
	numComp     int // 1 = grayscale, 3 = color
	hSamp       [3]int // horizontal sampling factor per component
	vSamp       [3]int // vertical sampling factor per component
	quantVals   [3][64]uint16 // quantization table per component
	dcCoeffs    [][64]int16    // DCT coefficients per block (in zigzag order)
	pattern     string         // pixel pattern to generate
}

// NewMinimalJPEGBuilder creates a builder with default settings (8x8 grayscale).
func NewMinimalJPEGBuilder() *MinimalJPEGBuilder {
	b := &MinimalJPEGBuilder{
		width:   8,
		height:  8,
		numComp: 1,
		pattern: "solid",
	}
	b.hSamp = [3]int{1, 1, 1}
	b.vSamp = [3]int{1, 1, 1}
	// Default quant table: all 1s (minimal quantization loss)
	for t := 0; t < 3; t++ {
		for i := 0; i < 64; i++ {
			b.quantVals[t][i] = 1
		}
	}
	return b
}

// SetDimensions sets the image dimensions.
func (b *MinimalJPEGBuilder) SetDimensions(w, h int) *MinimalJPEGBuilder {
	b.width = w
	b.height = h
	return b
}

// SetGrayscale configures for 1-component grayscale output.
func (b *MinimalJPEGBuilder) SetGrayscale() *MinimalJPEGBuilder {
	b.numComp = 1
	b.hSamp[0] = 1
	b.vSamp[0] = 1
	return b
}

// SetColor configures for 3-component YCbCr output with 4:4:4 subsampling.
func (b *MinimalJPEGBuilder) SetColor() *MinimalJPEGBuilder {
	b.numComp = 3
	b.hSamp = [3]int{1, 1, 1}
	b.vSamp = [3]int{1, 1, 1}
	return b
}

// SetSubsampling sets the horizontal and vertical sampling factors for component ci.
func (b *MinimalJPEGBuilder) SetSubsampling(ci, h, v int) *MinimalJPEGBuilder {
	if ci >= 0 && ci < 3 {
		b.hSamp[ci] = h
		b.vSamp[ci] = v
	}
	return b
}

// SetQuantTable sets the quantization table values (natural order) for table t.
func (b *MinimalJPEGBuilder) SetQuantTable(t int, vals [64]uint16) *MinimalJPEGBuilder {
	if t >= 0 && t < 3 {
		b.quantVals[t] = vals
	}
	return b
}

// SetPattern sets the pixel pattern to generate: "solid", "gradient", "checkerboard".
func (b *MinimalJPEGBuilder) SetPattern(pattern string) *MinimalJPEGBuilder {
	b.pattern = pattern
	return b
}

// Build constructs the JPEG byte sequence.
func (b *MinimalJPEGBuilder) Build() []byte {
	var buf []byte

	// SOI
	buf = append(buf, 0xFF, 0xD8)

	// DQT (quantization table(s))
	buf = appendDQT(buf, b)

	// SOF0 (start of frame - baseline)
	buf = appendSOF0(buf, b)

	// DHT (Huffman tables)
	buf = appendDHT(buf, b)

	// SOS (start of scan) + compressed data
	buf = appendSOS(buf, b)

	// EOI
	buf = append(buf, 0xFF, 0xD9)

	return buf
}

// appendDQT appends a DQT marker with quantization tables.
func appendDQT(buf []byte, b *MinimalJPEGBuilder) []byte {
	numTables := 1
	if b.numComp == 3 {
		numTables = 2 // one for luma, one for chroma
	}

	payloadLen := 2 // length field itself
	for t := 0; t < numTables; t++ {
		payloadLen += 1 + 64 // precision/tq byte + 64 quant values
	}

	marker := make([]byte, 2+payloadLen)
	marker[0] = 0xFF
	marker[1] = 0xDB // M_DQT
	binary.BigEndian.PutUint16(marker[2:], uint16(payloadLen))

	offset := 4
	for t := 0; t < numTables; t++ {
		// Precision (0=8-bit, 1=16-bit) in high nibble, table index in low nibble
		marker[offset] = byte(t) // 8-bit precision, table 0 (or 1)
		offset++
		for i := 0; i < 64; i++ {
			marker[offset] = byte(b.quantVals[t][i])
			offset++
		}
	}

	return append(buf, marker...)
}

// appendSOF0 appends a SOF0 (baseline) marker.
func appendSOF0(buf []byte, b *MinimalJPEGBuilder) []byte {
	// SOF0 payload: length(2) + precision(1) + height(2) + width(2) + numComp(1)
	// + numComp * (componentID(1) + samplingFactors(1) + quantTableSel(1))
	payloadLen := 2 + 1 + 2 + 2 + 1 + b.numComp*3

	marker := make([]byte, 2+payloadLen)
	marker[0] = 0xFF
	marker[1] = 0xC0 // M_SOF0
	binary.BigEndian.PutUint16(marker[2:], uint16(payloadLen))

	marker[4] = 8 // precision = 8 bits
	binary.BigEndian.PutUint16(marker[5:], uint16(b.height))
	binary.BigEndian.PutUint16(marker[7:], uint16(b.width))
	marker[9] = byte(b.numComp)

	offset := 10
	for ci := 0; ci < b.numComp; ci++ {
		marker[offset] = byte(ci + 1) // component ID (1-based)
		offset++
		marker[offset] = byte(b.hSamp[ci]<<4 | b.vSamp[ci])
		offset++
		// Quant table selector: use table 0 for luma, table 1 for chroma
		if ci == 0 {
			marker[offset] = 0
		} else {
			marker[offset] = 1
		}
		offset++
	}

	return append(buf, marker...)
}

// Standard JPEG Huffman tables (from JPEG spec, Annex K).

// Standard DC luminance Huffman table.
var stdDCLuminanceBits = [17]byte{0, 0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
var stdDCLuminanceVals = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

// Standard AC luminance Huffman table.
var stdACLuminanceBits = [17]byte{0, 0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 0x7D}
var stdACLuminanceVals = []byte{
	0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
	0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
	0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08,
	0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0,
	0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0A, 0x16,
	0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28,
	0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
	0x3A, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
	0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
	0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
	0x6A, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
	0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
	0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
	0x99, 0x9A, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7,
	0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6,
	0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5,
	0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xD2, 0xD3, 0xD4,
	0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2,
	0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA,
	0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8,
	0xF9, 0xFA,
}

// Standard DC chrominance Huffman table.
var stdDCChrominanceBits = [17]byte{0, 0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0}
var stdDCChrominanceVals = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

// Standard AC chrominance Huffman table.
var stdACChrominanceBits = [17]byte{0, 0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 0x77}
var stdACChrominanceVals = []byte{
	0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
	0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
	0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
	0xA1, 0xB1, 0xC1, 0x09, 0x23, 0x33, 0x52, 0xF0,
	0x15, 0x62, 0x72, 0xD1, 0x0A, 0x16, 0x24, 0x34,
	0xE1, 0x25, 0xF1, 0x17, 0x18, 0x19, 0x1A, 0x26,
	0x27, 0x28, 0x29, 0x2A, 0x35, 0x36, 0x37, 0x38,
	0x39, 0x3A, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
	0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
	0x59, 0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
	0x69, 0x6A, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
	0x79, 0x7A, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
	0x88, 0x89, 0x8A, 0x92, 0x93, 0x94, 0x95, 0x96,
	0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3, 0xA4, 0xA5,
	0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4,
	0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3,
	0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xD2,
	0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA,
	0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9,
	0xEA, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8,
	0xF9, 0xFA,
}

// appendDHT appends DHT markers with standard Huffman tables.
func appendDHT(buf []byte, b *MinimalJPEGBuilder) []byte {
	// Always include DC and AC luminance tables (table class 0 = DC, 1 = AC; table id 0)
	buf = appendSingleDHT(buf, 0x00, stdDCLuminanceBits, stdDCLuminanceVals) // DC table 0
	buf = appendSingleDHT(buf, 0x10, stdACLuminanceBits, stdACLuminanceVals)  // AC table 0

	if b.numComp == 3 {
		// Include DC and AC chrominance tables (table id 1)
		buf = appendSingleDHT(buf, 0x01, stdDCChrominanceBits, stdDCChrominanceVals) // DC table 1
		buf = appendSingleDHT(buf, 0x11, stdACChrominanceBits, stdACChrominanceVals) // AC table 1
	}

	return buf
}

func appendSingleDHT(buf []byte, classAndID byte, bits [17]byte, vals []byte) []byte {
	payloadLen := 2 + 1 + 16 + len(vals) // length field (2) + class/id (1) + 16 bits counts + symbols
	marker := make([]byte, payloadLen+2) // +2 for the 0xFF 0xC4 marker bytes
	marker[0] = 0xFF
	marker[1] = 0xC4 // M_DHT
	binary.BigEndian.PutUint16(marker[2:], uint16(payloadLen))
	marker[4] = classAndID // table class (high nibble) and table id (low nibble)
	copy(marker[5:21], bits[1:17])
	copy(marker[21:], vals)
	return append(buf, marker...)
}

// appendSOS appends the SOS marker and the compressed scan data.
func appendSOS(buf []byte, b *MinimalJPEGBuilder) []byte {
	// SOS header
	payloadLen := 2 + 1 + b.numComp*2 + 3 // length + numComp + comp entries + Ss/Se/AhAl
	sosMarker := make([]byte, 2+payloadLen)
	sosMarker[0] = 0xFF
	sosMarker[1] = 0xDA // M_SOS
	binary.BigEndian.PutUint16(sosMarker[2:], uint16(payloadLen))
	sosMarker[4] = byte(b.numComp)

	offset := 5
	for ci := 0; ci < b.numComp; ci++ {
		sosMarker[offset] = byte(ci + 1) // component ID
		offset++
		// DC table (high nibble) / AC table (low nibble)
		if ci == 0 {
			sosMarker[offset] = 0x00 // DC table 0, AC table 0
		} else {
			sosMarker[offset] = 0x11 // DC table 1, AC table 1
		}
		offset++
	}
	sosMarker[offset] = 0   // Ss = 0
	sosMarker[offset+1] = 63 // Se = 63
	sosMarker[offset+2] = 0   // Ah = 0, Al = 0

	buf = append(buf, sosMarker...)

	// Generate compressed scan data
	// We generate a DC-only block for each 8x8 block in the image.
	// The DC coefficient encodes the pixel value using a known quant table.
	// With quant=1 and level shift, DC coefficient = pixel_value - 128.

	// For this builder, we produce solid-color blocks with DC-only coefficients.
	// The scan data is encoded using the standard Huffman tables.
	scanData := b.generateScanData()
	buf = append(buf, scanData...)

	return buf
}

// zigzagOrder maps natural (row, col) to zigzag index.
var zigzagOrder = [8][8]int{
	{0, 1, 5, 6, 14, 15, 27, 28},
	{2, 4, 7, 13, 16, 26, 29, 42},
	{3, 8, 12, 17, 25, 30, 41, 43},
	{9, 11, 18, 24, 31, 40, 44, 53},
	{10, 19, 23, 32, 39, 45, 52, 54},
	{20, 22, 33, 38, 46, 51, 55, 60},
	{21, 34, 37, 47, 50, 56, 59, 61},
	{35, 36, 48, 49, 57, 58, 62, 63},
}

// generateScanData creates the compressed scan data using standard Huffman tables.
// We encode DC-only blocks (all AC coefficients are 0, signaled by EOB).
// Each block produces a uniform 8x8 region with the pixel value determined
// by the DC coefficient.
func (b *MinimalJPEGBuilder) generateScanData() []byte {
	var bitBuf uint32
	var bitsLeft int = 32
	var out []byte

	// Compute the number of MCU rows and columns
	maxH := 1
	maxV := 1
	for ci := 0; ci < b.numComp; ci++ {
		if b.hSamp[ci] > maxH {
			maxH = b.hSamp[ci]
		}
		if b.vSamp[ci] > maxV {
			maxV = b.vSamp[ci]
		}
	}

	// Number of MCU columns and rows
	mcuW := (b.width + maxH*8 - 1) / (maxH * 8)
	mcuH := (b.height + maxV*8 - 1) / (maxV * 8)

	// Previous DC values for differential coding
	prevDC := make([]int, b.numComp)

	for mcuRow := 0; mcuRow < mcuH; mcuRow++ {
		for mcuCol := 0; mcuCol < mcuW; mcuCol++ {
			for ci := 0; ci < b.numComp; ci++ {
				blocksX := b.hSamp[ci]
				blocksY := b.vSamp[ci]
				for by := 0; by < blocksY; by++ {
					for bx := 0; bx < blocksX; bx++ {
						// Compute pixel value for this block
						pixelVal := b.pixelValue(mcuCol, mcuRow, bx, by, ci)

						// DC coefficient: value is (pixel - 128) since JPEG uses level shift
						// With quant table = 1, dequantized value = dc_coeff * 1
						// ISLOW IDCT for DC-only block: output = (dc_coeff * quant) * (1/8) + 128
						// So dc_coeff = (pixel - 128) for quant=1
						dcCoeff := pixelVal - 128

						// Differential DC coding
						diff := dcCoeff - prevDC[ci]
						prevDC[ci] = dcCoeff

						// Encode DC using standard DC Huffman table
						if ci == 0 {
							bitBuf, bitsLeft = encodeDCStd(bitBuf, bitsLeft, &out, diff, true)
						} else {
							bitBuf, bitsLeft = encodeDCStd(bitBuf, bitsLeft, &out, diff, false)
						}

						// Encode AC: EOB (all zeros)
						if ci == 0 {
							bitBuf, bitsLeft = encodeEOBStdLuminance(bitBuf, bitsLeft, &out)
						} else {
							bitBuf, bitsLeft = encodeEOBStdChrominance(bitBuf, bitsLeft, &out)
						}
					}
				}
			}
		}
	}

	// Flush remaining bits (pad with 1 bits)
	if bitsLeft < 32 {
		bitBuf <<= uint(bitsLeft)
		out = append(out, byte(bitBuf>>24))
		if bitsLeft < 24 {
			out = append(out, byte(bitBuf>>16))
		}
		if bitsLeft < 16 {
			out = append(out, byte(bitBuf>>8))
		}
	}

	// Byte-stuff: replace any 0xFF in scan data with 0xFF 0x00
	stuffed := make([]byte, 0, len(out)*2)
	for _, b := range out {
		stuffed = append(stuffed, b)
		if b == 0xFF {
			stuffed = append(stuffed, 0x00)
		}
	}

	return stuffed
}

// pixelValue returns the pixel value for a specific block in the pattern.
func (b *MinimalJPEGBuilder) pixelValue(mcuCol, mcuRow, blockX, blockY, component int) int {
	// Compute the global pixel position for this block
	pixelX := mcuCol*8 + blockX*8
	pixelY := mcuRow*8 + blockY*8

	switch b.pattern {
	case "solid":
		// All luma = 128 (mid-gray), all chroma = 128 (neutral)
		if component == 0 {
			return 128
		}
		return 128

	case "gradient":
		// Horizontal gradient 0..255 for luma, 128 for chroma
		if component == 0 {
			if b.width <= 1 {
				return 128
			}
			return (pixelX * 255) / (b.width - 1)
		}
		return 128

	case "checkerboard":
		// 8x8 checkerboard: alternating 200 and 56 (light/dark gray)
		if component == 0 {
			if (pixelX/8+pixelY/8)%2 == 0 {
				return 200
			}
			return 56
		}
		return 128

	default:
		return 128
	}
}

// encodeDCStd encodes a DC differential value using the standard Huffman tables.
// Returns updated bitBuf and bitsLeft.
func encodeDCStd(bitBuf uint32, bitsLeft int, out *[]byte, diff int, luminance bool) (uint32, int) {
	// Determine the category and additional bits
	var category int
	var additionalBits uint
	var numAdditional int

	if diff == 0 {
		category = 0
	} else {
		absDiff := diff
		if absDiff < 0 {
			absDiff = -absDiff
		}
		// Find category: smallest n such that 2^(n-1) <= absVal < 2^n
		category = 0
		temp := absDiff
		for temp > 0 {
			category++
			temp >>= 1
		}

		numAdditional = category
		if diff > 0 {
			additionalBits = uint(diff)
		} else {
			// For negative values: additional bits = diff - 1 (in unsigned form)
			additionalBits = uint(diff - 1)
		}
	}

	// Encode using standard DC Huffman code
	var code uint32
	var codeLen int

	if luminance {
		code, codeLen = stdDCLuminanceEncode(category)
	} else {
		code, codeLen = stdDCChrominanceEncode(category)
	}

	// Write the Huffman code
	bitBuf, bitsLeft = writeBits(bitBuf, bitsLeft, out, code, codeLen)

	// Write the additional bits
	if numAdditional > 0 {
		bitBuf, bitsLeft = writeBits(bitBuf, bitsLeft, out, uint32(additionalBits), numAdditional)
	}

	return bitBuf, bitsLeft
}

// Standard DC luminance Huffman codes (from spec Annex K, Table K.3).
func stdDCLuminanceEncode(category int) (code uint32, length int) {
	codes := []struct {
		cat   int
		code  uint32
		len   int
	}{
		{0, 0b00, 2},
		{1, 0b010, 3},
		{2, 0b011, 3},
		{3, 0b100, 3},
		{4, 0b101, 3},
		{5, 0b110, 3},
		{6, 0b1110, 4},
		{7, 0b11110, 5},
		{8, 0b111110, 6},
		{9, 0b1111110, 7},
		{10, 0b11111110, 8},
		{11, 0b111111110, 9},
	}
	for _, c := range codes {
		if c.cat == category {
			return c.code, c.len
		}
	}
	return 0, 0
}

// Standard DC chrominance Huffman codes (from spec Annex K, Table K.4).
func stdDCChrominanceEncode(category int) (code uint32, length int) {
	codes := []struct {
		cat   int
		code  uint32
		len   int
	}{
		{0, 0b00, 2},
		{1, 0b01, 2},
		{2, 0b100, 3},
		{3, 0b101, 3},
		{4, 0b110, 3},
		{5, 0b1110, 4},
		{6, 0b11110, 5},
		{7, 0b111110, 6},
		{8, 0b1111110, 7},
		{9, 0b11111110, 8},
		{10, 0b111111110, 9},
		{11, 0b1111111110, 10},
	}
	for _, c := range codes {
		if c.cat == category {
			return c.code, c.len
		}
	}
	return 0, 0
}

// encodeEOBStdLuminance encodes an AC EOB using the standard luminance AC table.
// EOB symbol = 0x00, which maps to code 1010 (4 bits) in standard table.
func encodeEOBStdLuminance(bitBuf uint32, bitsLeft int, out *[]byte) (uint32, int) {
	// EOB (0x00) for standard AC luminance: code = 1010, length = 4
	return writeBits(bitBuf, bitsLeft, out, 0b1010, 4)
}

// encodeEOBStdChrominance encodes an AC EOB using the standard chrominance AC table.
// EOB symbol = 0x00, which maps to code 00 (2 bits) in standard table.
func encodeEOBStdChrominance(bitBuf uint32, bitsLeft int, out *[]byte) (uint32, int) {
	// EOB (0x00) for standard AC chrominance: code = 00, length = 2
	return writeBits(bitBuf, bitsLeft, out, 0b00, 2)
}

// writeBits writes n bits from code (MSB first) to the output buffer.
// Returns updated bitBuf and bitsLeft.
func writeBits(bitBuf uint32, bitsLeft int, out *[]byte, code uint32, n int) (uint32, int) {
	if n == 0 {
		return bitBuf, bitsLeft
	}
	bitBuf <<= uint(n)
	bitBuf |= code
	bitsLeft -= n

	for bitsLeft <= 24 {
		b := byte(bitBuf >> 24)
		*out = append(*out, b)
		if b == 0xFF {
			*out = append(*out, 0x00) // byte stuffing
		}
		bitBuf <<= 8
		bitsLeft += 8
	}

	return bitBuf, bitsLeft
}

// ExpectedPixels returns the expected pixel values for the configured pattern.
// For grayscale, returns w*h bytes. For color, returns w*h*3 bytes (RGB).
func (b *MinimalJPEGBuilder) ExpectedPixels() []byte {
	if b.numComp == 1 {
		return b.expectedGrayPixels()
	}
	return b.expectedColorPixels()
}

func (b *MinimalJPEGBuilder) expectedGrayPixels() []byte {
	pixels := make([]byte, b.width*b.height)
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			mcuCol := x / (8 * b.hSamp[0])
			mcuRow := y / (8 * b.vSamp[0])
			blkX := (x / 8) % b.hSamp[0]
			blkY := (y / 8) % b.vSamp[0]
			val := b.pixelValue(mcuCol, mcuRow, blkX, blkY, 0)
			if val < 0 {
				val = 0
			}
			if val > 255 {
				val = 255
			}
			pixels[y*b.width+x] = byte(val)
		}
	}
	return pixels
}

func (b *MinimalJPEGBuilder) expectedColorPixels() []byte {
	pixels := make([]byte, b.width*b.height*3)
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			mcuCol := x / (8 * b.hSamp[0])
			mcuRow := y / (8 * b.vSamp[0])
			blkX := (x / 8) % b.hSamp[0]
			blkY := (y / 8) % b.vSamp[0]

			yVal := b.pixelValue(mcuCol, mcuRow, blkX, blkY, 0)
			cbVal := b.pixelValue(mcuCol, mcuRow, blkX, blkY, 1)
			crVal := b.pixelValue(mcuCol, mcuRow, blkX, blkY, 2)

			// Convert YCbCr to RGB
			r, g, bb := ycbcrToRGB(yVal, cbVal, crVal)
			idx := (y*b.width + x) * 3
			pixels[idx] = r
			pixels[idx+1] = g
			pixels[idx+2] = bb
		}
	}
	return pixels
}

// ycbcrToRGB converts YCbCr to RGB using the standard JPEG formula.
func ycbcrToRGB(y, cb, cr int) (byte, byte, byte) {
	// R = Y + 1.402 * (Cr - 128)
	// G = Y - 0.344136 * (Cb - 128) - 0.714136 * (Cr - 128)
	// B = Y + 1.772 * (Cb - 128)
	r := float64(y) + 1.402*float64(cr-128)
	g := float64(y) - 0.344136*float64(cb-128) - 0.714136*float64(cr-128)
	bb := float64(y) + 1.772*float64(cb-128)

	return clampByteF(r), clampByteF(g), clampByteF(bb)
}

func clampByteF(v float64) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v + 0.5)
}
