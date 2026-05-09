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

// ArithmeticConditioningTable is one JPEG arithmetic conditioning table entry.
// It maps to libjpeg's arith_dc_L, arith_dc_U, and arith_ac_K arrays at the
// same index.
type ArithmeticConditioningTable struct {
	DCLower uint8
	DCUpper uint8
	ACK     uint8
}
