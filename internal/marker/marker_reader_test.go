package marker

import (
	"bytes"
	"io"
	"testing"
)

// buildMinimalJPEG creates a minimal valid JPEG byte stream.
// SOI + optional markers + EOI
func buildMinimalJPEG() []byte {
	return []byte{0xFF, 0xD8, 0xFF, 0xD9} // SOI + EOI
}

// buildSOF0Grayscale creates a minimal JPEG with SOF0 for 8x8 grayscale.
func buildSOF0Grayscale() []byte {
	var buf bytes.Buffer
	// SOI
	buf.Write([]byte{0xFF, 0xD8})
	// DQT (quantization table 0, 8-bit precision, all 1s for simplicity)
	buf.Write([]byte{0xFF, 0xDB})
	// Length: 2 (length field) + 1 (precision+index) + 64 (values) = 67
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00) // 8-bit precision, table 0
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01) // all quant values = 1
	}
	// SOF0: 8x8, 1 component, 8-bit precision
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B}) // length = 11
	buf.WriteByte(0x08)            // precision = 8
	buf.Write([]byte{0x00, 0x08}) // height = 8
	buf.Write([]byte{0x00, 0x08}) // width = 8
	buf.WriteByte(0x01)            // 1 component
	buf.WriteByte(0x01)            // component ID = 1
	buf.WriteByte(0x11)            // sampling: H=1, V=1
	buf.WriteByte(0x00)            // quant table 0
	// DHT: minimal DC table (category 0 -> code 00)
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14}) // length = 20 (2 + 1 + 16 + 1)
	buf.WriteByte(0x00)            // DC table 0
	// bits[1..16]: 1 code of length 2, rest 0
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	// huffval: symbol 0
	buf.Write([]byte{0x00})
	// DHT: minimal AC table
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14}) // length = 20 (2 + 1 + 16 + 1)
	buf.WriteByte(0x10)            // AC table 0
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	// huffval: symbol 0x00 (EOB)
	buf.Write([]byte{0x00})
	// SOS
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08}) // length = 8
	buf.WriteByte(0x01)            // 1 component
	buf.WriteByte(0x01)            // component ID = 1
	buf.WriteByte(0x00)            // DC table 0, AC table 0
	buf.WriteByte(0x00)            // Ss = 0
	buf.WriteByte(0x3F)            // Se = 63
	buf.WriteByte(0x00)            // Ah = 0, Al = 0
	// Compressed data (just zeros for one MCU)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})
	// EOI
	buf.Write([]byte{0xFF, 0xD9})
	return buf.Bytes()
}

func TestNewDecompressor(t *testing.T) {
	d := NewDecompressor()
	if d == nil {
		t.Fatal("NewDecompressor returned nil")
	}
	if d.GlobalState != DStateStart {
		t.Errorf("GlobalState = %d, want %d", d.GlobalState, DStateStart)
	}
	if d.marker == nil {
		t.Error("marker should be initialized")
	}
	if d.inputCtl == nil {
		t.Error("inputCtl should be initialized")
	}
	for i := 0; i < NumQuantTbls; i++ {
		if d.QuantTbls[i] != nil {
			t.Errorf("QuantTbls[%d] should be nil initially", i)
		}
	}
	for i := 0; i < NumHuffTbls; i++ {
		if d.DCHuffTbls[i] != nil {
			t.Errorf("DCHuffTbls[%d] should be nil initially", i)
		}
		if d.ACHuffTbls[i] != nil {
			t.Errorf("ACHuffTbls[%d] should be nil initially", i)
		}
	}
}

func TestReadHeaderTablesOnly(t *testing.T) {
	data := buildMinimalJPEG() // SOI + EOI
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	retcode, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}
	if retcode != JPEGHeaderTablesOnly {
		t.Errorf("ReadHeader returned %d, want %d", retcode, JPEGHeaderTablesOnly)
	}
}

func TestReadHeaderNotJPEG(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00} // not JPEG
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(false)
	if err == nil {
		t.Fatal("expected error for non-JPEG data")
	}
	if err != ErrNoSOI {
		t.Errorf("error = %v, want ErrNoSOI", err)
	}
}

func TestReadHeaderTruncated(t *testing.T) {
	data := []byte{0xFF} // truncated SOI
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(false)
	if err == nil {
		t.Fatal("expected error for truncated data")
	}
}

func TestReadHeaderValidSOF0(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	retcode, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}
	if retcode != JPEGHeaderOK {
		t.Errorf("ReadHeader returned %d, want %d (JPEGHeaderOK)", retcode, JPEGHeaderOK)
	}

	if d.ImageWidth != 8 {
		t.Errorf("ImageWidth = %d, want 8", d.ImageWidth)
	}
	if d.ImageHeight != 8 {
		t.Errorf("ImageHeight = %d, want 8", d.ImageHeight)
	}
	if d.NumComponents != 1 {
		t.Errorf("NumComponents = %d, want 1", d.NumComponents)
	}
	if d.DataPrecision != 8 {
		t.Errorf("DataPrecision = %d, want 8", d.DataPrecision)
	}
}

func TestReadHeaderSetsDefaults(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	// Default decompress params should be set
	if d.OutputGamma != 1.0 {
		t.Errorf("OutputGamma = %f, want 1.0", d.OutputGamma)
	}
	if d.DCTMethod != DCTISlow {
		t.Errorf("DCTMethod = %d, want %d", d.DCTMethod, DCTISlow)
	}
}

func TestReadHeaderRequiresImage(t *testing.T) {
	data := buildMinimalJPEG() // SOI + EOI (tables only)
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true) // requireImage = true
	if err == nil {
		t.Fatal("expected error for tables-only when image is required")
	}
	if err != ErrNoImage {
		t.Errorf("error = %v, want ErrNoImage", err)
	}
}

func TestReadHeaderBadState(t *testing.T) {
	d := NewDecompressor()
	d.GlobalState = DStateScanning // wrong state for ReadHeader

	_, err := d.ReadHeader(false)
	if err == nil {
		t.Fatal("expected error for bad state")
	}
	if err != ErrBadState {
		t.Errorf("error = %v, want ErrBadState", err)
	}
}

func TestGetSOIDuplicate(t *testing.T) {
	// SOI + SOI should produce ErrSOIDuplicate
	data := []byte{0xFF, 0xD8, 0xFF, 0xD8}
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(false)
	if err == nil {
		t.Fatal("expected error for duplicate SOI")
	}
	if err != ErrSOIDuplicate {
		t.Errorf("error = %v, want ErrSOIDuplicate", err)
	}
}

func TestReadHeaderDQT(t *testing.T) {
	// Build JPEG with DQT followed by SOS (not tables-only)
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI

	// DQT marker: table 0, 8-bit values
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43}) // length = 67
	buf.WriteByte(0x00)            // 8-bit precision, table 0
	for i := 0; i < 64; i++ {
		buf.WriteByte(byte(i + 1))
	}

	// SOF0: 8x8, 1 component
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B}) // length = 11
	buf.WriteByte(0x08)            // precision = 8
	buf.Write([]byte{0x00, 0x08}) // height = 8
	buf.Write([]byte{0x00, 0x08}) // width = 8
	buf.WriteByte(0x01)            // 1 component
	buf.WriteByte(0x01)            // component ID
	buf.WriteByte(0x11)            // sampling
	buf.WriteByte(0x00)            // quant table 0

	// DHT: minimal DC table
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})

	// DHT: minimal AC table
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})

	// SOS
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x3F)
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))

	retcode, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}
	if retcode != JPEGHeaderOK {
		t.Errorf("retcode = %d, want %d", retcode, JPEGHeaderOK)
	}

	if d.QuantTbls[0] == nil {
		t.Fatal("QuantTbls[0] should be set after DQT")
	}
	if d.QuantTbls[0].QuantVal[0] != 1 {
		t.Errorf("QuantVal[0] = %d, want 1", d.QuantTbls[0].QuantVal[0])
	}
}

func TestReadHeaderDHT(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI

	// DQT first (needed for SOS to work)
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}

	// SOF0
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)

	// DHT marker: DC table 0
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.WriteByte(0x00)

	// DHT: AC table 0
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.WriteByte(0x00)

	// SOS
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x3F)
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	if d.DCHuffTbls[0] == nil {
		t.Fatal("DCHuffTbls[0] should be set after DHT")
	}
	if d.DCHuffTbls[0].Bits[1] != 1 {
		t.Errorf("DCHuffTbls[0].Bits[1] = %d, want 1", d.DCHuffTbls[0].Bits[1])
	}
}

func TestReadHeaderDRI(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DRI marker
	buf.Write([]byte{0xFF, 0xDD})
	buf.Write([]byte{0x00, 0x04}) // length = 4
	buf.Write([]byte{0x00, 0x0A}) // restart interval = 10
	buf.Write([]byte{0xFF, 0xD9}) // EOI

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))

	_, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	if d.RestartInterval != 10 {
		t.Errorf("RestartInterval = %d, want 10", d.RestartInterval)
	}
}

func TestReadHeaderUnsupportedSOF(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	buf.WriteByte(0xFF)
	buf.WriteByte(0xC3) // SOF3 (unsupported)
	// minimal SOF data to not cause read errors before validation
	buf.Write([]byte{0x00, 0x08}) // length
	buf.WriteByte(0x08)            // precision
	buf.Write([]byte{0x00, 0x01}) // height
	buf.Write([]byte{0x00, 0x01}) // width
	buf.WriteByte(0x00)            // 0 components
	buf.Write([]byte{0xFF, 0xD9}) // EOI

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))

	_, err := d.ReadHeader(false)
	if err == nil {
		t.Fatal("expected error for unsupported SOF")
	}
	if err != ErrSOFUnsupported {
		t.Errorf("error = %v, want ErrSOFUnsupported", err)
	}
}

func TestReadHeaderJFIF(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI

	// APP0 (JFIF) marker
	buf.Write([]byte{0xFF, 0xE0})
	buf.Write([]byte{0x00, 0x10}) // length = 16
	buf.Write([]byte("JFIF\x00"))  // identifier
	buf.WriteByte(0x01)             // major version
	buf.WriteByte(0x02)             // minor version
	buf.WriteByte(0x01)             // density units (dots per inch)
	buf.Write([]byte{0x00, 0x48})  // X density = 72
	buf.Write([]byte{0x00, 0x48})  // Y density = 72
	buf.WriteByte(0x00)             // thumbnail width
	buf.WriteByte(0x00)             // thumbnail height

	buf.Write([]byte{0xFF, 0xD9}) // EOI

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))

	_, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	if !d.SawJFIFMarker {
		t.Error("SawJFIFMarker should be true")
	}
	if d.JFIFMajorVersion != 1 {
		t.Errorf("JFIFMajorVersion = %d, want 1", d.JFIFMajorVersion)
	}
	if d.JFIFMinorVersion != 2 {
		t.Errorf("JFIFMinorVersion = %d, want 2", d.JFIFMinorVersion)
	}
	if d.XDensity != 72 {
		t.Errorf("XDensity = %d, want 72", d.XDensity)
	}
	if d.YDensity != 72 {
		t.Errorf("YDensity = %d, want 72", d.YDensity)
	}
}

func TestReadHeaderAdobe(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI

	// APP14 (Adobe) marker
	buf.Write([]byte{0xFF, 0xEE})
	buf.Write([]byte{0x00, 0x0E}) // length = 14
	buf.Write([]byte("Adobe"))     // identifier
	buf.Write([]byte{0x00, 0x64})  // version
	buf.Write([]byte{0x00, 0x00})  // flags0
	buf.Write([]byte{0x00, 0x00})  // flags1
	buf.WriteByte(0x01)             // transform = YCbCr

	buf.Write([]byte{0xFF, 0xD9}) // EOI

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))

	_, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	if !d.SawAdobeMarker {
		t.Error("SawAdobeMarker should be true")
	}
	if d.AdobeTransform != 1 {
		t.Errorf("AdobeTransform = %d, want 1", d.AdobeTransform)
	}
}

func TestReadHeaderEOF(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{}))

	_, err := d.ReadHeader(false)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestReadHeaderShortSOI(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0xFF}))

	_, err := d.ReadHeader(false)
	if err == nil {
		t.Fatal("expected error for truncated SOI")
	}
}

func TestMarkerReaderReadBytes(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0x01, 0x02, 0x03}))
	mr := d.marker

	data, err := mr.readBytes(d, 3)
	if err != nil {
		t.Fatalf("readBytes error: %v", err)
	}
	if !bytes.Equal(data, []byte{0x01, 0x02, 0x03}) {
		t.Errorf("readBytes = %v, want [1 2 3]", data)
	}
}

func TestMarkerReaderReadByte(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0xAB}))
	mr := d.marker

	b, err := mr.readByte(d)
	if err != nil {
		t.Fatalf("readByte error: %v", err)
	}
	if b != 0xAB {
		t.Errorf("readByte = 0x%02X, want 0xAB", b)
	}
}

func TestMarkerReaderReadUint16(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0x12, 0x34}))
	mr := d.marker

	val, err := mr.readUint16(d)
	if err != nil {
		t.Fatalf("readUint16 error: %v", err)
	}
	if val != 0x1234 {
		t.Errorf("readUint16 = 0x%04X, want 0x1234", val)
	}
}

func TestMarkerReaderSkipBytes(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0x01, 0x02, 0x03, 0x04, 0x05}))
	mr := d.marker

	err := mr.skipBytes(d, 2)
	if err != nil {
		t.Fatalf("skipBytes error: %v", err)
	}

	b, err := mr.readByte(d)
	if err != nil {
		t.Fatalf("readByte error: %v", err)
	}
	if b != 0x03 {
		t.Errorf("after skip, readByte = 0x%02X, want 0x03", b)
	}
}

func TestMarkerReaderTruncated(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0x01}))
	mr := d.marker

	_, err := mr.readBytes(d, 5)
	if err == nil {
		t.Fatal("expected error for truncated read")
	}
	if err != io.ErrUnexpectedEOF {
		t.Errorf("error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestGetSOFSetsComponentInfo(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	if len(d.CompInfo) != 1 {
		t.Fatalf("len(CompInfo) = %d, want 1", len(d.CompInfo))
	}
	if d.CompInfo[0].ComponentID != 1 {
		t.Errorf("ComponentID = %d, want 1", d.CompInfo[0].ComponentID)
	}
	if d.CompInfo[0].HSampFactor != 1 {
		t.Errorf("HSampFactor = %d, want 1", d.CompInfo[0].HSampFactor)
	}
	if d.CompInfo[0].VSampFactor != 1 {
		t.Errorf("VSampFactor = %d, want 1", d.CompInfo[0].VSampFactor)
	}
	if d.CompInfo[0].QuantTblNo != 0 {
		t.Errorf("QuantTblNo = %d, want 0", d.CompInfo[0].QuantTblNo)
	}
}

func TestJDivRoundUp(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{7, 8, 1},
		{8, 8, 1},
		{9, 8, 2},
		{1, 1, 1},
		{0, 1, 0},
		{15, 16, 1},
		{100, 7, 15},
	}
	for _, tt := range tests {
		got := JDivRoundUp(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("JDivRoundUp(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
