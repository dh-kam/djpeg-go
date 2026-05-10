package marker

import (
	"bytes"
	"testing"
)

func TestDecompressorSetSource(t *testing.T) {
	d := NewDecompressor()
	r := bytes.NewReader([]byte{0xFF, 0xD8})
	d.SetSource(r)

	if d.Src == nil {
		t.Error("Src should not be nil after SetSource")
	}
}

func TestDecompressorReset(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	// Verify state was populated
	if d.ImageWidth == 0 {
		t.Fatal("ImageWidth should be set after ReadHeader")
	}

	d.Reset()

	if d.GlobalState != DStateStart {
		t.Errorf("GlobalState after reset = %d, want %d", d.GlobalState, DStateStart)
	}
	if d.QuantTbls[0] == nil {
		t.Error("QuantTbls[0] should be preserved after reset")
	}
	if d.DCHuffTbls[0] == nil {
		t.Error("DCHuffTbls[0] should be preserved after reset")
	}
	if d.ACHuffTbls[0] == nil {
		t.Error("ACHuffTbls[0] should be preserved after reset")
	}
	if d.CompInfo != nil {
		t.Error("CompInfo should be nil after reset")
	}
}

func TestDecompressorIsProgressive(t *testing.T) {
	d := NewDecompressor()
	if d.IsProgressive() {
		t.Error("new decompressor should not be progressive")
	}

	d.ProgressiveMode = true
	if !d.IsProgressive() {
		t.Error("should be progressive after setting ProgressiveMode")
	}
}

func TestDecompressorIsBaselineJPEG(t *testing.T) {
	d := NewDecompressor()
	if d.IsBaselineJPEG() {
		t.Error("new decompressor should not be baseline")
	}

	d.IsBaselineFlag = true
	if !d.IsBaselineJPEG() {
		t.Error("should be baseline after setting flag")
	}
}

func TestDecompressorValidateHeader(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(d *Decompressor)
		wantErr bool
	}{
		{
			name: "valid header",
			setup: func(d *Decompressor) {
				d.NumComponents = 1
				d.ImageWidth = 8
				d.ImageHeight = 8
				d.DataPrecision = 8
				d.CompInfo = []ComponentInfo{
					{QuantTblNo: 0},
				}
				d.QuantTbls[0] = &QuantTable{}
			},
			wantErr: false,
		},
		{
			name: "no components",
			setup: func(d *Decompressor) {
				d.NumComponents = 0
				d.ImageWidth = 8
				d.ImageHeight = 8
				d.DataPrecision = 8
			},
			wantErr: true,
		},
		{
			name: "zero width",
			setup: func(d *Decompressor) {
				d.NumComponents = 1
				d.ImageWidth = 0
				d.ImageHeight = 8
				d.DataPrecision = 8
			},
			wantErr: true,
		},
		{
			name: "zero height",
			setup: func(d *Decompressor) {
				d.NumComponents = 1
				d.ImageWidth = 8
				d.ImageHeight = 0
				d.DataPrecision = 8
			},
			wantErr: true,
		},
		{
			name: "bad precision",
			setup: func(d *Decompressor) {
				d.NumComponents = 1
				d.ImageWidth = 8
				d.ImageHeight = 8
				d.DataPrecision = 12
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDecompressor()
			tt.setup(d)
			err := d.ValidateHeader()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecompressorGetQuantTable(t *testing.T) {
	d := NewDecompressor()
	d.QuantTbls[0] = &QuantTable{}

	qt := d.GetQuantTable(0)
	if qt == nil {
		t.Error("GetQuantTable(0) should not be nil")
	}

	qt = d.GetQuantTable(1)
	if qt != nil {
		t.Error("GetQuantTable(1) should be nil")
	}

	qt = d.GetQuantTable(-1)
	if qt != nil {
		t.Error("GetQuantTable(-1) should be nil")
	}

	qt = d.GetQuantTable(NumQuantTbls)
	if qt != nil {
		t.Errorf("GetQuantTable(%d) should be nil", NumQuantTbls)
	}
}

func TestDecompressorGetHuffTable(t *testing.T) {
	d := NewDecompressor()
	d.DCHuffTbls[0] = &HuffTable{}
	d.ACHuffTbls[1] = &HuffTable{}

	ht := d.GetHuffTable(0, true)
	if ht == nil {
		t.Error("GetHuffTable(0, DC) should not be nil")
	}

	ht = d.GetHuffTable(1, false)
	if ht == nil {
		t.Error("GetHuffTable(1, AC) should not be nil")
	}

	ht = d.GetHuffTable(0, false)
	if ht != nil {
		t.Error("GetHuffTable(0, AC) should be nil")
	}

	ht = d.GetHuffTable(-1, true)
	if ht != nil {
		t.Error("GetHuffTable(-1, DC) should be nil")
	}
}

func TestDecompressorComponent(t *testing.T) {
	d := NewDecompressor()
	d.NumComponents = 2
	d.CompInfo = make([]ComponentInfo, 2)
	d.CompInfo[0].ComponentID = 1
	d.CompInfo[1].ComponentID = 2

	c := d.Component(0)
	if c == nil {
		t.Fatal("Component(0) should not be nil")
	}
	if c.ComponentID != 1 {
		t.Errorf("Component(0).ComponentID = %d, want 1", c.ComponentID)
	}

	c = d.Component(1)
	if c == nil {
		t.Fatal("Component(1) should not be nil")
	}

	c = d.Component(-1)
	if c != nil {
		t.Error("Component(-1) should be nil")
	}

	c = d.Component(2)
	if c != nil {
		t.Error("Component(2) should be nil (out of range)")
	}
}

func TestDecompressorInputComplete(t *testing.T) {
	d := NewDecompressor()
	// State before any processing
	if d.InputComplete() {
		t.Error("InputComplete should be false for new decompressor")
	}
}

func TestDecompressorHasMultipleScans(t *testing.T) {
	d := NewDecompressor()
	// Not in a valid state
	if d.HasMultipleScans() {
		t.Error("HasMultipleScans should be false before header read")
	}
}

func TestDecompressorSaveMarkers(t *testing.T) {
	d := NewDecompressor()
	// Should not panic
	d.SaveMarkers(M_COM, 0)
	d.SaveMarkers(M_COM, 65533)
	d.SaveMarkers(M_APP0, 0)
	d.SaveMarkers(M_APP1, 65533)
	d.SaveMarkers(M_APP14, 0)
}

func TestDecompressorSetMarkerProcessor(t *testing.T) {
	d := NewDecompressor()
	d.SetMarkerProcessor(M_COM, func(d *Decompressor) error {
		return nil
	})
	// The processor should be set (verified indirectly)
	if d.marker.processCOM == nil {
		t.Error("processCOM should not be nil after SetMarkerProcessor")
	}
}

func TestDecompressorStartDecompress(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	err = d.StartDecompress()
	if err != nil {
		t.Fatalf("StartDecompress error: %v", err)
	}
	if d.GlobalState != DStateScanning {
		t.Errorf("GlobalState = %d, want %d", d.GlobalState, DStateScanning)
	}
}

func TestDecompressorReadScanlines(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	err = d.StartDecompress()
	if err != nil {
		t.Fatalf("StartDecompress error: %v", err)
	}

	scanlines := make([][]uint8, 1)
	scanlines[0] = make([]uint8, d.OutputWidth)
	n, err := d.ReadScanlines(scanlines)
	if err != ErrNotImpl {
		t.Fatalf("ReadScanlines error = %v, want ErrNotImpl", err)
	}
	if n != 0 {
		t.Errorf("ReadScanlines returned %d, want 0", n)
	}
}

func TestDecompressorReadScanlinesWithReader(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}
	if err := d.StartDecompress(); err != nil {
		t.Fatalf("StartDecompress error: %v", err)
	}

	var calls int
	d.SetScanlineReader(func(d *Decompressor, scanlines [][]uint8) (int, error) {
		calls++
		if len(scanlines) == 0 {
			return 0, nil
		}
		for i := range scanlines[0] {
			scanlines[0][i] = 0x7f
		}
		d.OutputScanline++
		return 1, nil
	})

	scanlines := make([][]uint8, 1)
	scanlines[0] = make([]uint8, d.OutputWidth)
	n, err := d.ReadScanlines(scanlines)
	if err != nil {
		t.Fatalf("ReadScanlines error: %v", err)
	}
	if n != 1 || calls != 1 || d.OutputScanline != 1 {
		t.Fatalf("ReadScanlines n=%d calls=%d output_scanline=%d, want 1/1/1",
			n, calls, d.OutputScanline)
	}
	for i, sample := range scanlines[0] {
		if sample != 0x7f {
			t.Fatalf("scanline[%d] = 0x%02x, want 0x7f", i, sample)
		}
	}
}

func TestDecompressorInputCompleteAfterEOI(t *testing.T) {
	data := buildMinimalJPEG()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(false)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	if !d.InputComplete() {
		t.Error("InputComplete should be true after tables-only data")
	}
}

func TestDecompressorHasMultipleScansAfterHeader(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	// Single component, non-progressive should not have multiple scans
	if d.HasMultipleScans() {
		t.Error("HasMultipleScans should be false for single-component non-progressive")
	}
}

func TestDecompressorStartInputPass(t *testing.T) {
	data := buildSOF0Grayscale()
	d := NewDecompressor()
	d.SetSource(bytes.NewReader(data))

	_, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}

	err = d.StartInputPass()
	if err != nil {
		t.Fatalf("StartInputPass error: %v", err)
	}

	if d.BlocksInMCU != 1 {
		t.Errorf("BlocksInMCU = %d, want 1 for single component", d.BlocksInMCU)
	}
}

func TestDecompressorStartDecompressBadState(t *testing.T) {
	d := NewDecompressor()
	d.GlobalState = DStateScanning

	err := d.StartDecompress()
	if err == nil {
		t.Fatal("expected error for bad state")
	}
	if err != ErrBadState {
		t.Errorf("error = %v, want ErrBadState", err)
	}
}

func TestDecompressorFinishDecompressBadState(t *testing.T) {
	d := NewDecompressor()
	d.GlobalState = DStateStart

	err := d.FinishDecompress()
	if err == nil {
		t.Fatal("expected error for bad state")
	}
}

func TestDecompressorReadHeaderFullYCbCr(t *testing.T) {
	// Build a minimal 3-component (YCbCr) JPEG
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI

	// DQT
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43}) // length = 67
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}

	// SOF0: 8x8, 3 components (YCbCr subsampled 2x2)
	buf.Write([]byte{0xFF, 0xC0})
	buf.Write([]byte{0x00, 0x11}) // length = 17
	buf.WriteByte(0x08)           // precision
	buf.Write([]byte{0x00, 0x08}) // height
	buf.Write([]byte{0x00, 0x08}) // width
	buf.WriteByte(0x03)           // 3 components
	// Y: ID=1, H=2 V=2, quant table 0
	buf.WriteByte(0x01)
	buf.WriteByte(0x22)
	buf.WriteByte(0x00)
	// Cb: ID=2, H=1 V=1, quant table 0
	buf.WriteByte(0x02)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	// Cr: ID=3, H=1 V=1, quant table 0
	buf.WriteByte(0x03)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)

	// SOS
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x0C}) // length = 12
	buf.WriteByte(0x03)           // 3 components
	buf.WriteByte(0x01)           // Y
	buf.WriteByte(0x00)           // DC=0, AC=0
	buf.WriteByte(0x02)           // Cb
	buf.WriteByte(0x00)           // DC=0, AC=0
	buf.WriteByte(0x03)           // Cr
	buf.WriteByte(0x00)           // DC=0, AC=0
	buf.WriteByte(0x00)           // Ss
	buf.WriteByte(0x3F)           // Se
	buf.WriteByte(0x00)           // Ah/Al
	// Some dummy scan data
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0xFF, 0xD9}) // EOI

	d := NewDecompressor()
	d.SetSource(bytes.NewReader(buf.Bytes()))

	retcode, err := d.ReadHeader(true)
	if err != nil {
		t.Fatalf("ReadHeader error: %v", err)
	}
	if retcode != JPEGHeaderOK {
		t.Errorf("retcode = %d, want %d", retcode, JPEGHeaderOK)
	}

	if d.NumComponents != 3 {
		t.Errorf("NumComponents = %d, want 3", d.NumComponents)
	}
	if d.ImageWidth != 8 || d.ImageHeight != 8 {
		t.Errorf("dimensions = %dx%d, want 8x8", d.ImageWidth, d.ImageHeight)
	}
	if d.OutColorSpace != CSRGB {
		t.Errorf("OutColorSpace = %d, want CSRGB", d.OutColorSpace)
	}
}
