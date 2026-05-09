package djpeg

// Component describes one JPEG image component from the decompressor header.
type Component struct {
	ID                     int
	Index                  int
	HSampFactor            int
	VSampFactor            int
	QuantizationTableIndex int
	DCHuffmanTableIndex    int
	ACHuffmanTableIndex    int
	WidthInBlocks          int
	HeightInBlocks         int
	DownsampledWidth       int
	DownsampledHeight      int
	DCTHScaledSize         int
	DCTVScaledSize         int
	ComponentNeeded        bool
}

// RawComponent is a decoded downsampled component plane returned by
// ReadRawData. Pix contains Height rows with Stride bytes per row. Width is the
// logical downsampled component width; Stride may include right-edge padding.
type RawComponent struct {
	Component Component
	Width     int
	Height    int
	Stride    int
	Pix       []byte
}

// CoefficientBlock is one 8x8 block of quantized DCT coefficients in natural
// row-major order.
type CoefficientBlock [64]int16

// CoefficientComponent is one component's coefficient block array returned by
// ReadCoefficients. Blocks are stored row-major with WidthInBlocks blocks per
// row and HeightInBlocks rows.
type CoefficientComponent struct {
	Component      Component
	WidthInBlocks  int
	HeightInBlocks int
	Blocks         []CoefficientBlock
}

// ScanParameters exposes the current SOS/per-scan decompressor parameters.
// These map to libjpeg fields such as comps_in_scan, Ss, Se, Ah, Al, and the
// derived MCU geometry for the current scan.
type ScanParameters struct {
	ComponentsInScan    int
	MCUsPerRow          int
	MCURowsInScan       int
	BlocksInMCU         int
	SpectralStart       int
	SpectralEnd         int
	ApproxHigh          int
	ApproxLow           int
	LimitingSpectralEnd int
}
