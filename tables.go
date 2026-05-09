package djpeg

// HuffmanTableClass identifies a DC or AC Huffman table.
type HuffmanTableClass int

const (
	HuffmanTableDC HuffmanTableClass = iota
	HuffmanTableAC
)

// QuantizationTable is a parsed JPEG quantization table.
type QuantizationTable struct {
	Values    [64]uint16
	SentTable bool
}

// HuffmanTable is a parsed JPEG Huffman table.
type HuffmanTable struct {
	Bits      [17]uint8
	Values    [256]uint8
	SentTable bool
}
