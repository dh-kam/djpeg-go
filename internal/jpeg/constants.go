package jpeg

// Constants ported from IJG libjpeg 9f (jpeglib.h, jmorecfg.h, jpegint.h, jdct.h).

// DCT block dimensions.
const (
	DCTSize       = 8  // The basic DCT block is 8x8 coefficients.
	DCTSize2      = 64 // DCTSize squared; number of elements in a block.
	NumQuantTbls  = 4  // Quantization tables are numbered 0..3.
	NumHuffTbls   = 4  // Huffman tables are numbered 0..3.
	NumArithTbls  = 16 // Arithmetic coding tables are numbered 0..15.
	MaxCompsInScan = 4 // JPEG limit on number of components in one scan.
	MaxSampFactor = 4  // JPEG limit on sampling factors.
	CMaxBlocksInMCU = 10 // Compressor's limit on blocks per MCU.
	DMaxBlocksInMCU = 10 // Decompressor's limit on blocks per MCU.
	MaxComponents = 10 // Maximum number of image components.
)

// Sample precision (8-bit mode).
const (
	BitsInJSample = 8
	MaxJSample    = 255
	CenterJSample = 128
)

// JPEG marker codes.
const (
	JPEGRst0 = 0xD0 // RST0 marker code
	JPEGEoi  = 0xD9 // EOI marker code
	JPEGApp0 = 0xE0 // APP0 marker code
	JPEGCom  = 0xFE // COM marker code
)

// JPEG_MAX_DIMENSION is a tad under 64K to prevent overflows.
const JPEGMaxDimension = 65500

// Return codes for jpeg_read_header.
const (
	JPEGSuspended       = 0 // Suspended due to lack of input data
	JPEGHeaderOK        = 1 // Found valid image datastream
	JPEGHeaderTablesOnly = 2 // Found valid table-specs-only datastream
)

// Return codes for jpeg_consume_input.
const (
	// JPEGSuspended = 0 (shared with above)
	JPEGReachedSOS      = 1 // Reached start of new scan
	JPEGReachedEOI      = 2 // Reached end of image
	JPEGRowCompleted    = 3 // Completed one iMCU row
	JPEGScanCompleted   = 4 // Completed last iMCU row of a scan
)

// Memory pool IDs.
const (
	JPoolPermanent = 0 // Lasts until master record is destroyed.
	JPoolImage     = 1 // Lasts until done with image/datastream.
	JPoolNumPools  = 2
)

// Decompression state values (global_state field).
const (
	DStateStart    = 200 // After create_decompress
	DStateInHeader = 201 // Reading header markers, no SOS yet
	DStateReady    = 202 // Found SOS, ready for start_decompress
	DStatePreload  = 203 // Reading multiscan file in start_decompress
	DStatePreScan  = 204 // Performing dummy pass for 2-pass quant
	DStateScanning = 205 // start_decompress done, read_scanlines OK
	DStateRawOK    = 206 // start_decompress done, read_raw_data OK
	DStateBufImage = 207 // Expecting jpeg_start_output
	DStateBufPost  = 208 // Looking for SOS/EOI in jpeg_finish_output
	DStateRdCoefs  = 209 // Reading file in jpeg_read_coefficients
	DStateStopping = 210 // Looking for EOI in jpeg_finish_decompress
)

// Compression state values (global_state field).
const (
	CStateStart    = 100 // After create_compress
	CStateScanning = 101 // start_compress done, write_scanlines OK
	CStateRawOK    = 102 // start_compress done, write_raw_data OK
	CStateWrCoefs  = 103 // jpeg_write_coefficients done
)

// Buffer operating modes.
const (
	JBufPassThru    = iota // Plain stripwise operation
	JBufSaveSource         // Run source subobject only, save output
	JBufCrankDest          // Run dest subobject only, using saved data
	JBufSaveAndPass        // Run both subobjects, save output
)

// Range limiting constants.
const (
	RangeBits   = 2
	RangeCenter = CenterJSample << RangeBits
	RangeMask   = RangeCenter*2 - 1
	RangeSubset = RangeCenter - CenterJSample
)

// Input buffer size for data source.
const InputBufSize = 4096

// Error message string parameter maximum length.
const (
	JMSGLengthMax = 200
	JMSGStrParmMax = 80
)

// JColorSpace represents known color spaces.
type JColorSpace int

const (
	JCSUnknown   JColorSpace = iota // Error/unspecified
	JCSGrayscale                    // Monochrome
	JCSRGB                          // Red/green/blue, standard RGB (sRGB)
	JCSYCbCr                        // Y/Cb/Cr (also known as YUV), standard YCC
	JCSCMYK                         // C/M/Y/K
	JCSYCCK                         // Y/Cb/Cr/K
	JCSBGRGB                        // Big gamut red/green/blue, bg-sRGB
	JCSBGYCC                        // Big gamut Y/Cb/Cr, bg-sYCC
)

// JColorTransform represents supported color transforms.
type JColorTransform int

const (
	JCTNone          JColorTransform = 0
	JCTSubtractGreen JColorTransform = 1
)

// JDCTMethod represents DCT/IDCT algorithm options.
type JDCTMethod int

const (
	JDCTISlow  JDCTMethod = iota // Slow but accurate integer algorithm
	JDCTIFast                    // Faster, less accurate integer method
	JDCTFloat                    // Floating-point: accurate, fast on fast HW
)

// Default DCT method.
const JDCTDefault = JDCTISlow

// Fastest DCT method.
const JDCTFastest = JDCTIFast

// JDitherMode represents dithering options for decompression.
type JDitherMode int

const (
	JDitherNone   JDitherMode = iota // No dithering
	JDitherOrdered                   // Simple ordered dither
	JDitherFS                        // Floyd-Steinberg error diffusion dither
)

// RGB offsets (standard RGB order: R, G, B).
const (
	RGBRed       = 0
	RGBGreen     = 1
	RGBBlue      = 2
	RGBPixelSize = 3
)

// JBufMode represents operating modes for buffer controllers.
type JBufMode int
