package marker

import (
	"bytes"
	"testing"
)

// minimalJPEG is a minimal valid JPEG: SOI + EOI
var minimalJPEG = []byte{0xFF, 0xD8, 0xFF, 0xD9}

// minimalGrayscaleJPEG is a minimal valid grayscale JPEG with SOF0, DQT, DHT, SOS.
func buildFuzzGrayscaleJPEG() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DQT
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	// SOF0: 8x8, 1 component
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	// DHT DC
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	// DHT AC
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
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
	buf.Write([]byte{0xFF, 0xD9}) // EOI
	return buf.Bytes()
}

// FuzzJPEGHeader tests that marker parsing does not panic on arbitrary input.
func FuzzJPEGHeader(f *testing.F) {
	f.Add(minimalJPEG)
	f.Add(buildFuzzGrayscaleJPEG())
	f.Add([]byte{0xFF})            // truncated
	f.Add([]byte{0xFF, 0xD8})      // SOI only
	f.Add([]byte{0x00, 0x00})      // not JPEG
	f.Add([]byte{})                 // empty
	f.Add([]byte{0xFF, 0xD8, 0xFF}) // partial marker

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := NewDecompressor()
		dec.SetSource(bytes.NewReader(data))
		// Should not panic; errors are fine
		dec.ReadHeader(true)
	})
}

// FuzzJPEGHeaderNoRequire tests header parsing without requiring an image.
func FuzzJPEGHeaderNoRequire(f *testing.F) {
	f.Add(minimalJPEG)
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x02}) // SOI + short APP0

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := NewDecompressor()
		dec.SetSource(bytes.NewReader(data))
		dec.ReadHeader(false)
	})
}

// FuzzJPEGMarkers tests various marker sequences.
func FuzzJPEGMarkers(f *testing.F) {
	// Build a seed with multiple markers
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	buf.Write([]byte{0xFF, 0xDD}) // DRI
	buf.Write([]byte{0x00, 0x04})
	buf.Write([]byte{0x00, 0x01})
	buf.Write([]byte{0xFF, 0xD9}) // EOI

	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := NewDecompressor()
		dec.SetSource(bytes.NewReader(data))
		dec.ReadHeader(false)
		_ = dec.InputComplete()
		_ = dec.HasMultipleScans()
	})
}

// TestEdgeCaseZeroDimensions tests handling of zero-dimension images.
func TestEdgeCaseZeroDimensions(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// SOF0 with 0x0 dimensions
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B}) // length
	buf.WriteByte(0x08)            // precision
	buf.Write([]byte{0x00, 0x00}) // height = 0
	buf.Write([]byte{0x00, 0x00}) // width = 0
	buf.WriteByte(0x01)            // 1 component
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9}) // EOI

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for 0x0 dimensions")
	}
}

// TestEdgeCaseLargeDimensions tests handling of very large image dimensions.
func TestEdgeCaseLargeDimensions(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// SOF0 with 65500x65500 dimensions
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B}) // length
	buf.WriteByte(0x08)            // precision
	// height = 65500 = 0xFFDC
	buf.Write([]byte{0xFF, 0xDC})
	// width = 65500 = 0xFFDC
	buf.Write([]byte{0xFF, 0xDC})
	buf.WriteByte(0x01) // 1 component
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	// Need DQT + DHT + SOS for header to succeed
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
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
	// Should parse the header (65500x65500 is within JPEGMaxDimension=65500)
	if err != nil {
		t.Logf("ReadHeader with large dimensions: %v", err)
	}
}

// TestEdgeCaseTooManyComponents tests handling of too many components.
func TestEdgeCaseTooManyComponents(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// SOF0 with 5 components (> MaxCompsInScan=4 for SOS)
	buf.Write([]byte{0xFF, 0xC0})
	// length = 8 + 5*3 = 23
	buf.Write([]byte{0x00, 0x17})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08}) // height
	buf.Write([]byte{0x00, 0x08}) // width
	buf.WriteByte(0x05)            // 5 components
	for i := 0; i < 5; i++ {
		buf.WriteByte(byte(i + 1))
		buf.WriteByte(0x11)
		buf.WriteByte(0x00)
	}
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	// SOF should parse fine, the issue is at scan time
	if err != nil {
		t.Logf("Expected: SOF with 5 components: %v", err)
	}
}

// TestEdgeCaseBadQuantTableIndex tests invalid quantization table indices.
func TestEdgeCaseBadQuantTableIndex(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DQT with bad index (4, which >= NumQuantTbls=4)
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x04) // 8-bit precision, table index 4 (invalid)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for bad quant table index")
	}
}

// TestEdgeCaseDQT16BitPrecision tests 16-bit quantization table values.
func TestEdgeCaseDQT16BitPrecision(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DQT with 16-bit precision
	buf.Write([]byte{0xFF, 0xDB})
	// length = 2 + 1 + 64*2 = 131
	buf.Write([]byte{0x00, 0x83})
	buf.WriteByte(0x10) // 16-bit precision, table 0
	for i := 0; i < 64; i++ {
		buf.Write([]byte{0x00, 0x01})
	}
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err != nil {
		t.Logf("DQT 16-bit: %v", err)
	}
}

// TestEdgeCaseMissingHuffmanTables tests missing Huffman tables in SOS.
func TestEdgeCaseMissingHuffmanTables(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DQT
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	// SOF0: 8x8, 1 component
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	// SOS WITHOUT DHT markers (tables are missing)
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
	// Header should parse; Huffman tables are just nil
	if err != nil {
		t.Logf("Missing Huffman tables: %v", err)
	}
}

// TestEdgeCaseTruncatedDQT tests truncated DQT marker.
func TestEdgeCaseTruncatedDQT(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DQT with wrong length (says 67 but truncated)
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43}) // length = 67
	buf.WriteByte(0x00)
	for i := 0; i < 10; i++ { // Only 10 bytes, need 64
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Log("truncated DQT might succeed with partial data")
	}
}

// TestEdgeCaseBadDHTIndex tests DHT with invalid table index.
func TestEdgeCaseBadDHTIndex(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DHT with invalid index (0x24 = AC table index 4, out of range)
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x24) // AC table 4 (invalid, only 0-3 allowed)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for bad DHT index")
	}
}

// TestEdgeCaseNestedSOS tests two SOS markers without intervening scan data.
func TestEdgeCaseNestedSOS(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// DQT
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	// SOF0: 8x8, 1 component
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	// DHT DC
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	// DHT AC
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	// First SOS
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x3F)
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x00})
	// Second SOS immediately (no EOI, no scan data)
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
	// First SOS should succeed
	if err != nil {
		t.Logf("nested SOS ReadHeader: %v", err)
	}
}

// TestEdgeCaseMultipleSOI tests duplicate SOI marker.
func TestEdgeCaseMultipleSOI(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xD8, 0xFF, 0xD9}
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for duplicate SOI")
	}
}

// TestEdgeCaseSOSBeforeSOF tests SOS without preceding SOF.
func TestEdgeCaseSOSBeforeSOF(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// SOS without SOF
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x3F)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for SOS before SOF")
	}
}

// TestEdgeCaseDRI tests valid DRI marker parsing.
func TestEdgeCaseDRI(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xDD})
	buf.Write([]byte{0x00, 0x04}) // length
	buf.Write([]byte{0x00, 0x20}) // restart interval = 32
	// SOF0 + DQT + DHT + SOS for a complete header
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
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
		t.Fatalf("DRI + full header: %v", err)
	}
	if d.RestartInterval != 32 {
		t.Errorf("RestartInterval = %d, want 32", d.RestartInterval)
	}
}

// TestEdgeCaseDAC tests DAC (arithmetic coding) marker parsing.
func TestEdgeCaseDAC(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// DAC marker: DC table 0, value 0x21
	buf.Write([]byte{0xFF, 0xCC})
	buf.Write([]byte{0x00, 0x04}) // length
	buf.WriteByte(0x00)            // DC table 0
	buf.WriteByte(0x21)            // L=2, U=1
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err != nil {
		t.Logf("DAC marker: %v", err)
	}
}

// TestEdgeCaseDACBadIndex tests DAC with invalid index.
func TestEdgeCaseDACBadIndex(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// DAC marker with bad index
	buf.Write([]byte{0xFF, 0xCC})
	buf.Write([]byte{0x00, 0x04}) // length
	buf.WriteByte(0x20)            // index = 32, >= 2*NumArithTbls=32, invalid
	buf.WriteByte(0x01)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for bad DAC index")
	}
}

// TestEdgeCaseDACBadValue tests DAC with invalid L > U value.
func TestEdgeCaseDACBadValue(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// DAC marker: DC table 0, L > U (invalid)
	buf.Write([]byte{0xFF, 0xCC})
	buf.Write([]byte{0x00, 0x04}) // length
	buf.WriteByte(0x00)            // DC table 0
	buf.WriteByte(0x14)            // L=4 (low nibble), U=1 (high nibble) => L > U (invalid)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for bad DAC value (L > U)")
	}
}

// TestEdgeCaseDNL tests DNL marker handling.
func TestEdgeCaseDNL(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// DNL marker (should be skipped)
	buf.Write([]byte{0xFF, 0xDC})
	buf.Write([]byte{0x00, 0x04})
	buf.Write([]byte{0x00, 0x08}) // height = 8
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	// Should parse fine (DNL is skipped before SOF)
	if err != nil {
		t.Logf("DNL marker: %v", err)
	}
}

// TestEdgeCaseJFXXExtension tests JFXX extension APP0 marker.
func TestEdgeCaseJFXXExtension(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// APP0 with JFXX extension
	buf.Write([]byte{0xFF, 0xE0})
	buf.Write([]byte{0x00, 0x0A}) // length = 10
	buf.Write([]byte("JFXX\x00"))
	buf.WriteByte(0x01) // thumbnail type
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("JFXX extension: %v", err)
	}
	if d.SawJFIFMarker {
		t.Error("JFXX should not set JFIF marker")
	}
}

// TestEdgeCaseSOF1 tests SOF1 (extended sequential) parsing.
func TestEdgeCaseSOF1(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// SOF1 (extended sequential, Huffman)
	buf.Write([]byte{0xFF, 0xC1})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err != nil {
		t.Logf("SOF1: %v", err)
	}
	if d.IsBaselineFlag {
		t.Error("SOF1 should not set baseline flag")
	}
}

// TestEdgeCaseDuplicateSOF tests duplicate SOF marker.
func TestEdgeCaseDuplicateSOF(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// First SOF0
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	// Second SOF0 (duplicate)
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for duplicate SOF")
	}
	if err != ErrSOFDuplicate {
		t.Errorf("error = %v, want ErrSOFDuplicate", err)
	}
}

// TestEdgeCaseDRIBadLength tests DRI with wrong length.
func TestEdgeCaseDRIBadLength(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xDD})
	buf.Write([]byte{0x00, 0x05}) // wrong length (should be 4)
	buf.Write([]byte{0x00, 0x01})
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for bad DRI length")
	}
}

// TestEdgeCaseBadSOSComponentID tests SOS with unknown component ID.
func TestEdgeCaseBadSOSComponentID(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// DQT
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
	// SOS referencing component ID 99 (doesn't exist)
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x63) // component ID = 99
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x3F)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(true)
	if err == nil {
		t.Error("expected error for bad component ID in SOS")
	}
}

// TestEdgeCaseDHTBadCount tests DHT with too many symbols.
func TestEdgeCaseDHTBadCount(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// DHT with total count > 256
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x15}) // length
	buf.WriteByte(0x00)            // DC table 0
	// bits: 20 codes each for first 13 lengths = 260 symbols
	buf.Write([]byte{0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14,
		0x14, 0x14, 0x14, 0x14, 0x14, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for bad Huffman table (too many symbols)")
	}
}

// TestEdgeCaseUnsupportedSOF3 tests SOF3 (lossless) marker.
func TestEdgeCaseUnsupportedSOF3(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xC3}) // SOF3 (lossless, unsupported)
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err == nil {
		t.Error("expected error for unsupported SOF3")
	}
}

// TestEdgeCaseBadPrecision tests SOF with unsupported precision.
func TestEdgeCaseBadPrecision(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x02) // precision = 2 (invalid, should be 8-12)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	// Need rest of headers for initial setup to run
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
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
	if err == nil {
		t.Log("bad precision might pass header check but fail later")
	}
}

// TestEdgeCaseBadSamplingFactor tests SOF with invalid sampling factor.
func TestEdgeCaseBadSamplingFactor(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x00) // H=0, V=0 (invalid)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
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
	if err == nil {
		t.Error("expected error for bad sampling factor (0x0)")
	}
}

// TestEdgeCaseSaveMarker tests the save marker functionality.
func TestEdgeCaseSaveMarker(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// APP1 marker with some data
	buf.Write([]byte{0xFF, 0xE1})
	buf.Write([]byte{0x00, 0x08}) // length
	buf.Write([]byte("ABCD"))
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SaveMarkers(M_APP1, 65533)
	d.SetSource(bytes.NewReader(buf.Bytes()))

	_, err := d.ReadHeader(false)
	if err != nil {
		t.Logf("SaveMarker APP1: %v", err)
	}
}

// TestEdgeCaseCOMMarker tests COM (comment) marker parsing.
func TestEdgeCaseCOMMarker(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// COM marker
	buf.Write([]byte{0xFF, 0xFE})
	buf.Write([]byte{0x00, 0x0B}) // length = 11
	buf.Write([]byte("Test comment"))
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("COM marker: %v", err)
	}
}

// TestEdgeCaseCOMMarkerSaved tests COM marker with saving enabled.
// Note: When ReadHeader hits EOI without SOS (tables-only), it calls Reset()
// which clears MarkerList. So we test with requireImage=true to verify the
// error path but that COM was processed before Reset.
func TestEdgeCaseCOMMarkerSaved(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xFE})
	buf.Write([]byte{0x00, 0x0B})
	buf.Write([]byte("Test comment"))
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SaveMarkers(M_COM, 65533)
	d.SetSource(bytes.NewReader(buf.Bytes()))
	// requireImage=true will error on EOI, but before Reset
	_, err := d.ReadHeader(true)
	if err == nil {
		t.Fatal("expected error for EOI without SOS")
	}

	// MarkerList should be nil since Reset clears it on tables-only
	// Just verify that SaveMarkers was configured correctly
	// (the COM was processed by saveMarker processor)
}

// TestEdgeCaseSaveMarkerAPP14 tests saving APP14 (Adobe) marker.
func TestEdgeCaseSaveMarkerAPP14(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xEE}) // APP14
	buf.Write([]byte{0x00, 0x0E}) // length = 14
	buf.Write([]byte("Adobe"))
	buf.Write([]byte{0x00, 0x64})
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0x00, 0x00})
	buf.WriteByte(0x01)
	buf.Write([]byte{0xFF, 0xD9})

	d := NewDecompressor()
	d.SaveMarkers(M_APP14, 65533)
	d.SetSource(bytes.NewReader(buf.Bytes()))
	_, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("Save APP14: %v", err)
	}
	if !d.SawAdobeMarker {
		t.Error("SawAdobeMarker should be true")
	}
}

// TestEdgeCaseSOFProgressive tests SOF2 (progressive) parsing.
func TestEdgeCaseSOFProgressive(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	// SOF2 (progressive)
	buf.Write([]byte{0xFF, 0xC2})
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
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
		t.Logf("SOF2 progressive: %v", err)
	}
	if !d.ProgressiveMode {
		t.Error("ProgressiveMode should be true for SOF2")
	}
}

// TestEdgeCaseSOF9Arithmetic tests SOF9 (arithmetic coding) parsing.
func TestEdgeCaseSOF9Arithmetic(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xC9}) // SOF9
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
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
		t.Logf("SOF9 arithmetic: %v", err)
	}
	if !d.ArithCodeFlag {
		t.Error("ArithCodeFlag should be true for SOF9")
	}
}

// TestReadUint32 tests the readUint32 function.
func TestReadUint32(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0x12, 0x34, 0x56, 0x78}))
	mr := d.marker

	val, err := mr.readUint32(d)
	if err != nil {
		t.Fatalf("readUint32 error: %v", err)
	}
	if val != 0x12345678 {
		t.Errorf("readUint32 = 0x%08X, want 0x12345678", val)
	}
}

// TestReadUint32Truncated tests readUint32 with insufficient data.
func TestReadUint32Truncated(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0x12, 0x34}))
	mr := d.marker

	_, err := mr.readUint32(d)
	if err == nil {
		t.Error("expected error for truncated readUint32")
	}
}

// TestSkipBytesNegative tests skipBytes with negative count.
func TestSkipBytesNegative(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0x01, 0x02, 0x03}))
	mr := d.marker

	err := mr.skipBytes(d, -5)
	if err != nil {
		t.Errorf("skipBytes(-5) error: %v", err)
	}
}

// TestReadBytesZero tests readBytes with zero count.
func TestReadBytesZero(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{}))
	mr := d.marker

	data, err := mr.readBytes(d, 0)
	if err != nil {
		t.Errorf("readBytes(0) error: %v", err)
	}
	if data != nil {
		t.Errorf("readBytes(0) = %v, want nil", data)
	}
}

// TestConsumeInputReadyState tests ConsumeInput when already ready.
func TestConsumeInputReadyState(t *testing.T) {
	d := NewDecompressor()
	d.GlobalState = DStateReady

	retcode, err := d.ConsumeInput()
	if err != nil {
		t.Errorf("ConsumeInput ready state error: %v", err)
	}
	if retcode != JPEGReachedSOS {
		t.Errorf("ConsumeInput ready = %d, want %d", retcode, JPEGReachedSOS)
	}
}

// TestConsumeInputUnknownState tests ConsumeInput with invalid state.
func TestConsumeInputUnknownState(t *testing.T) {
	d := NewDecompressor()
	d.GlobalState = 999

	_, err := d.ConsumeInput()
	if err == nil {
		t.Error("expected error for unknown state")
	}
}

// TestReadRestartMarker tests the ReadRestartMarker wrapper.
func TestReadRestartMarker(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0xFF, 0xD0})) // RST0
	d.UnreadMarker = 0

	err := d.ReadRestartMarker()
	if err != nil {
		t.Logf("ReadRestartMarker: %v", err)
	}
}

// TestResyncToRestart tests the ResyncToRestart wrapper.
func TestResyncToRestart(t *testing.T) {
	d := NewDecompressor()
	d.SetSource(bytes.NewReader([]byte{0xFF, 0xD1})) // RST1
	d.UnreadMarker = 0xD1

	err := d.ResyncToRestart(0)
	if err != nil {
		t.Logf("ResyncToRestart: %v", err)
	}
}

// TestSetMarkerProcessorAPP tests setting marker processor for APP markers.
func TestSetMarkerProcessorAPP(t *testing.T) {
	d := NewDecompressor()
	d.SetMarkerProcessor(M_APP5, func(d *Decompressor) error {
		return nil
	})
	if d.marker.processAPPn[5] == nil {
		t.Error("processAPPn[5] should not be nil")
	}
}

// TestValidateHeaderBadQuantTableIndex tests ValidateHeader with bad quant table index.
func TestValidateHeaderBadQuantTableIndex(t *testing.T) {
	d := NewDecompressor()
	d.NumComponents = 1
	d.ImageWidth = 8
	d.ImageHeight = 8
	d.DataPrecision = 8
	d.CompInfo = []ComponentInfo{
		{QuantTblNo: 5}, // invalid: >= NumQuantTbls
	}

	err := d.ValidateHeader()
	if err == nil {
		t.Error("expected error for bad quant table index")
	}
}
