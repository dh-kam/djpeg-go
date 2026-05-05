package jpeg

import "io"

// Common API functions ported from IJG libjpeg 9f (jcomapi.c).
//
// These functions operate on both compression and decompression objects.
// In the Go port, we focus on the decompression path.

// ---------------------------------------------------------------------------
// jpeg_abort
// ---------------------------------------------------------------------------

// JPEGAbort aborts processing of a JPEG operation without destroying the object.
// In Go, this resets state and clears image-pool allocations.
func JPEGAbort(cinfo *JPEGCommon) {
	if cinfo.Mem == nil {
		return
	}

	// Reset state for possible reuse
	if cinfo.IsDecompressor {
		cinfo.GlobalState = DStateStart
	} else {
		cinfo.GlobalState = CStateStart
	}
}

// JPEGAbortDecompress aborts decompression.
func JPEGAbortDecompress(cinfo *JPEGDecompress) {
	JPEGAbort(&cinfo.JPEGCommon)
}

// ---------------------------------------------------------------------------
// jpeg_destroy
// ---------------------------------------------------------------------------

// JPEGDestroy destroys a JPEG object, releasing all resources.
// In Go, we just clear references and let the GC collect.
func JPEGDestroy(cinfo *JPEGCommon) {
	if cinfo.Mem != nil {
		SelfDestruct(cinfo)
	}
	cinfo.Mem = nil
	cinfo.GlobalState = 0 // Mark destroyed
}

// JPEGDestroyDecompress destroys a decompression object.
func JPEGDestroyDecompress(cinfo *JPEGDecompress) {
	JPEGDestroy(&cinfo.JPEGCommon)
}

// ---------------------------------------------------------------------------
// Table allocation helpers
// ---------------------------------------------------------------------------

// JPEGAllocQuantTable allocates and returns a new quantization table.
func JPEGAllocQuantTable() *JQuantTbl {
	return &JQuantTbl{}
}

// JPEGAllocHuffTable allocates and returns a new Huffman table.
func JPEGAllocHuffTable() *JHuffTbl {
	return &JHuffTbl{}
}

// ---------------------------------------------------------------------------
// Standard Huffman tables (from jcomapi.c jpeg_std_huff_table)
// ---------------------------------------------------------------------------

var (
	bitsDCLuminance = [17]uint8{0, 0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	valDCLuminance  = []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	bitsDCChrominance = [17]uint8{0, 0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0}
	valDCChrominance  = []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	bitsACLuminance = [17]uint8{0, 0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 0x7d}
	valACLuminance  = []uint8{
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
		0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
		0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
		0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
		0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
		0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
		0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
		0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
		0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
		0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
		0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
		0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
		0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
		0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
		0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
		0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
		0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
		0xf9, 0xfa,
	}

	bitsACChrominance = [17]uint8{0, 0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 0x77}
	valACChrominance  = []uint8{
		0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
		0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
		0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
		0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
		0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
		0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
		0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
		0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
		0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
		0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
		0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
		0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
		0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
		0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
		0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
		0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
		0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
		0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
		0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
		0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
		0xf9, 0xfa,
	}
)

// JPEGStdHuffTable sets up the standard Huffman table for the given parameters.
// isDC selects DC vs AC tables; tblno selects luminance (0) or chrominance (1).
// Returns the filled-in Huffman table.
func JPEGStdHuffTable(cinfo *JPEGDecompress, isDC bool, tblno int) *JHuffTbl {
	var bits *[17]uint8
	var val []uint8

	switch tblno {
	case 0:
		if isDC {
			bits = &bitsDCLuminance
			val = valDCLuminance
		} else {
			bits = &bitsACLuminance
			val = valACLuminance
		}
	case 1:
		if isDC {
			bits = &bitsDCChrominance
			val = valDCChrominance
		} else {
			bits = &bitsACChrominance
			val = valACChrominance
		}
	default:
		ErrExitDecompress(cinfo, ErrNoHuffTable, tblno)
		return nil
	}

	var htbl *JHuffTbl
	if isDC {
		if cinfo.DCHuffTblPtrs[tblno] == nil {
			cinfo.DCHuffTblPtrs[tblno] = JPEGAllocHuffTable()
		}
		htbl = cinfo.DCHuffTblPtrs[tblno]
	} else {
		if cinfo.ACHuffTblPtrs[tblno] == nil {
			cinfo.ACHuffTblPtrs[tblno] = JPEGAllocHuffTable()
		}
		htbl = cinfo.ACHuffTblPtrs[tblno]
	}

	// Copy the number-of-symbols-of-each-code-length counts
	htbl.Bits = *bits

	// Validate the counts and compute number of symbols
	nsymbols := 0
	for len := 1; len <= 16; len++ {
		nsymbols += int(bits[len])
	}
	if nsymbols > 256 {
		ErrExitDecompress(cinfo, ErrBadHuffTable)
	}

	// Copy symbol values
	for i := 0; i < 256; i++ {
		htbl.HuffVal[i] = 0
	}
	if nsymbols > 0 {
		for i := 0; i < nsymbols && i < len(val); i++ {
			htbl.HuffVal[i] = val[i]
		}
	}

	htbl.SentTable = false
	return htbl
}

// ---------------------------------------------------------------------------
// Decompression object creation and initialization
// ---------------------------------------------------------------------------

// CreateDecompress creates and initializes a new JPEGDecompress struct.
// This replaces the C jpeg_CreateDecompress function.
func CreateDecompress() *JPEGDecompress {
	cinfo := &JPEGDecompress{
		JPEGCommon: JPEGCommon{
			Err:           StdError(),
			IsDecompressor: true,
			GlobalState:    DStateStart,
		},
	}

	InitMemoryMgr(&cinfo.JPEGCommon)
	return cinfo
}

// InitDecompressFromReader creates a decompressor that reads from an io.Reader.
func InitDecompressFromReader(r io.Reader) *JPEGDecompress {
	cinfo := CreateDecompress()
	SetupSource(cinfo, r)
	return cinfo
}

// InitDecompressFromBytes creates a decompressor that reads from a byte slice.
func InitDecompressFromBytes(data []byte) *JPEGDecompress {
	cinfo := CreateDecompress()
	SetupMemSource(cinfo, data)
	return cinfo
}
