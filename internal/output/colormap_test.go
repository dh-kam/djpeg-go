package output

import (
	"bytes"
	"strings"
	"testing"
)

// TestReadColorMapGIF tests reading a color map from a GIF file.
func TestReadColorMapGIF(t *testing.T) {
	// Build a minimal GIF with a global color table of 4 entries
	var buf bytes.Buffer
	// Header
	buf.WriteString("GIF87a")
	// Logical Screen Descriptor: width=1, height=1, packed byte with GCT flag
	buf.WriteByte(0x01) // width low
	buf.WriteByte(0x00) // width high
	buf.WriteByte(0x01) // height low
	buf.WriteByte(0x00) // height high
	buf.WriteByte(0x81) // packed: GCT=1, color res=1, sort=0, GCT size=1 (4 entries)
	buf.WriteByte(0x00) // background color index
	buf.WriteByte(0x00) // pixel aspect ratio
	// Global Color Table: 4 entries * 3 bytes
	buf.Write([]byte{255, 0, 0})   // red
	buf.Write([]byte{0, 255, 0})   // green
	buf.Write([]byte{0, 0, 255})   // blue
	buf.Write([]byte{128, 128, 128}) // gray

	cm, err := ReadColorMap(&buf)
	if err != nil {
		t.Fatalf("ReadColorMap GIF: %v", err)
	}
	if cm.NumColors != 4 {
		t.Errorf("NumColors = %d, want 4", cm.NumColors)
	}
	if len(cm.Maps) != 3 {
		t.Fatalf("Maps length = %d, want 3", len(cm.Maps))
	}
	if cm.Maps[0][0] != 255 {
		t.Errorf("R[0] = %d, want 255", cm.Maps[0][0])
	}
}

func TestReadColorMapGIFNoGCT(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("GIF87a")
	// LSD without GCT flag
	buf.Write([]byte{0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for GIF without global color table")
	}
}

func TestReadColorMapGIFBadHeader(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("GIFXX") // bad header after 'G'

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for bad GIF header")
	}
}

// TestReadColorMapPPMRaw tests reading a color map from a raw (P6) PPM file.
func TestReadColorMapPPMRaw(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("P6\n2 2\n255\n")
	// 4 pixels, 3 unique colors
	buf.Write([]byte{255, 0, 0})     // red
	buf.Write([]byte{0, 255, 0})     // green
	buf.Write([]byte{0, 255, 0})     // green (duplicate)
	buf.Write([]byte{0, 0, 255})     // blue

	cm, err := ReadColorMap(&buf)
	if err != nil {
		t.Fatalf("ReadColorMap PPM: %v", err)
	}
	if cm.NumColors != 3 {
		t.Errorf("NumColors = %d, want 3 (unique colors)", cm.NumColors)
	}
}

// TestReadColorMapPPMText tests reading a color map from a text (P3) PPM file.
func TestReadColorMapPPMText(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("P3\n2 1\n255\n255 0 0 0 255 0\n")

	cm, err := ReadColorMap(&buf)
	if err != nil {
		t.Fatalf("ReadColorMap PPM text: %v", err)
	}
	if cm.NumColors != 2 {
		t.Errorf("NumColors = %d, want 2", cm.NumColors)
	}
}

// TestReadColorMapPPMBadFormat tests unsupported PPM format.
func TestReadColorMapPPMBadFormat(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("P5\n1 1\n255\n\x80") // P5 (PGM) not supported

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for unsupported PPM format")
	}
}

// TestReadColorMapPPMBadMaxval tests PPM with non-255 maxval.
func TestReadColorMapPPMBadMaxval(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("P6\n1 1\n65535\n")

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for maxval != 255")
	}
}

// TestReadColorMapPPMInvalidDimensions tests PPM with invalid dimensions.
func TestReadColorMapPPMInvalidDimensions(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("P6\n0 0\n255\n")

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for invalid dimensions")
	}
}

// TestReadColorMapUnknownFormat tests unrecognized format.
func TestReadColorMapUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("X unknown data")

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

// TestReadColorMapEmpty tests reading from empty reader.
func TestReadColorMapEmpty(t *testing.T) {
	_, err := ReadColorMap(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty reader")
	}
}

// TestReadPBMIntegerWithComment tests PPM integer parsing with comments.
func TestReadPBMIntegerWithComment(t *testing.T) {
	// Test indirectly through ReadColorMap - already covered by PPM tests above.
	// This test verifies the PPM header parser handles comments.
	var buf bytes.Buffer
	buf.WriteString("P6\n# This is a comment\n1 1\n# Another comment\n255\n")
	buf.Write([]byte{128, 0, 0}) // 1 pixel
	cm, err := ReadColorMap(&buf)
	if err != nil {
		t.Fatalf("ReadColorMap with comments: %v", err)
	}
	if cm.NumColors != 1 {
		t.Errorf("NumColors = %d, want 1", cm.NumColors)
	}
}

// TestWriteBE16 tests the big-endian 16-bit write helper.
func TestWriteBE16Helper(t *testing.T) {
	var buf bytes.Buffer
	writeBE16(&buf, 0x1234)
	if buf.Len() != 2 {
		t.Fatalf("writeBE16: buf len = %d, want 2", buf.Len())
	}
	if buf.Bytes()[0] != 0x12 || buf.Bytes()[1] != 0x34 {
		t.Errorf("writeBE16(0x1234) = %v, want [0x12 0x34]", buf.Bytes())
	}
}

// TestReadColorMapPPMTextTruncated tests truncated PPM text data.
func TestReadColorMapPPMTextTruncated(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("P3\n2 1\n255\n255 0") // incomplete data

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for truncated PPM text data")
	}
}

// TestReadColorMapPPMRawTruncated tests truncated PPM raw data.
func TestReadColorMapPPMRawTruncated(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("P6\n2 2\n255\n") // header says 4 pixels but no data

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for truncated PPM raw data")
	}
}

// TestReadColorMapGIFTruncated tests truncated GIF data.
func TestReadColorMapGIFTruncated(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("GIF87a")
	// No LSD

	_, err := ReadColorMap(&buf)
	if err == nil {
		t.Error("expected error for truncated GIF")
	}
}
