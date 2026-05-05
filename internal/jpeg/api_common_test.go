package jpeg

import (
	"bytes"
	"testing"
)

func TestCreateDecompress(t *testing.T) {
	cinfo := CreateDecompress()
	if cinfo == nil {
		t.Fatal("CreateDecompress returned nil")
	}
	if cinfo.Err == nil {
		t.Error("Err should not be nil")
	}
	if !cinfo.IsDecompressor {
		t.Error("IsDecompressor should be true")
	}
	if cinfo.GlobalState != DStateStart {
		t.Errorf("GlobalState = %d, want %d", cinfo.GlobalState, DStateStart)
	}
	if cinfo.Mem == nil {
		t.Error("Mem should not be nil")
	}
}

func TestInitDecompressFromReader(t *testing.T) {
	r := bytes.NewReader([]byte{0xFF, 0xD8, 0xFF, 0xD9})
	cinfo := InitDecompressFromReader(r)
	if cinfo == nil {
		t.Fatal("InitDecompressFromReader returned nil")
	}
	if cinfo.Src == nil {
		t.Error("Src should not be nil")
	}
}

func TestInitDecompressFromBytes(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	cinfo := InitDecompressFromBytes(data)
	if cinfo == nil {
		t.Fatal("InitDecompressFromBytes returned nil")
	}
	if cinfo.Src == nil {
		t.Error("Src should not be nil")
	}
}

func TestInitDecompressFromBytesEmptyPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for empty byte data")
		}
	}()
	InitDecompressFromBytes([]byte{})
}

func TestJPEGDestroy(t *testing.T) {
	cinfo := CreateDecompress()
	JPEGDestroy(&cinfo.JPEGCommon)

	if cinfo.Mem != nil {
		t.Error("Mem should be nil after JPEGDestroy")
	}
	if cinfo.GlobalState != 0 {
		t.Errorf("GlobalState = %d, want 0 after destroy", cinfo.GlobalState)
	}
}

func TestJPEGDestroyNil(t *testing.T) {
	cinfo := &JPEGCommon{}
	JPEGDestroy(cinfo) // Should not panic when Mem is nil
}

func TestJPEGDestroyDecompress(t *testing.T) {
	cinfo := CreateDecompress()
	JPEGDestroyDecompress(cinfo)
	if cinfo.Mem != nil {
		t.Error("Mem should be nil after JPEGDestroyDecompress")
	}
}

func TestJPEGAbort(t *testing.T) {
	cinfo := CreateDecompress()
	cinfo.GlobalState = DStateScanning
	JPEGAbort(&cinfo.JPEGCommon)
	if cinfo.GlobalState != DStateStart {
		t.Errorf("GlobalState = %d, want %d after abort", cinfo.GlobalState, DStateStart)
	}
}

func TestJPEGAbortNilMem(t *testing.T) {
	cinfo := &JPEGCommon{}
	JPEGAbort(cinfo) // Should not panic
}

func TestJPEGAbortDecompress(t *testing.T) {
	cinfo := CreateDecompress()
	cinfo.GlobalState = DStateScanning
	JPEGAbortDecompress(cinfo)
	if cinfo.GlobalState != DStateStart {
		t.Errorf("GlobalState = %d, want %d after abort", cinfo.GlobalState, DStateStart)
	}
}

func TestJPEGStdHuffTableDC(t *testing.T) {
	cinfo := CreateDecompress()

	// Test DC luminance (tblno=0, isDC=true)
	htbl := JPEGStdHuffTable(cinfo, true, 0)
	if htbl == nil {
		t.Fatal("JPEGStdHuffTable DC luminance returned nil")
	}
	if htbl.Bits[1] != 0 || htbl.Bits[2] != 1 {
		t.Errorf("DC luminance bits[1..2] = %d,%d", htbl.Bits[1], htbl.Bits[2])
	}
	if cinfo.DCHuffTblPtrs[0] == nil {
		t.Error("DCHuffTblPtrs[0] should be set")
	}
	if htbl.SentTable {
		t.Error("SentTable should be false")
	}

	// Test DC chrominance (tblno=1, isDC=true)
	htbl2 := JPEGStdHuffTable(cinfo, true, 1)
	if htbl2 == nil {
		t.Fatal("JPEGStdHuffTable DC chrominance returned nil")
	}
	if cinfo.DCHuffTblPtrs[1] == nil {
		t.Error("DCHuffTblPtrs[1] should be set")
	}
}

func TestJPEGStdHuffTableAC(t *testing.T) {
	cinfo := CreateDecompress()

	// Test AC luminance (tblno=0, isDC=false)
	htbl := JPEGStdHuffTable(cinfo, false, 0)
	if htbl == nil {
		t.Fatal("JPEGStdHuffTable AC luminance returned nil")
	}
	if cinfo.ACHuffTblPtrs[0] == nil {
		t.Error("ACHuffTblPtrs[0] should be set")
	}

	// Test AC chrominance (tblno=1, isDC=false)
	htbl2 := JPEGStdHuffTable(cinfo, false, 1)
	if htbl2 == nil {
		t.Fatal("JPEGStdHuffTable AC chrominance returned nil")
	}
	if cinfo.ACHuffTblPtrs[1] == nil {
		t.Error("ACHuffTblPtrs[1] should be set")
	}
}

func TestJPEGStdHuffTableInvalidTblno(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid table number")
		}
	}()

	cinfo := CreateDecompress()
	JPEGStdHuffTable(cinfo, true, 5) // invalid tblno
}

func TestJPEGStdHuffTableReusesExisting(t *testing.T) {
	cinfo := CreateDecompress()

	// First call creates the table
	htbl1 := JPEGStdHuffTable(cinfo, true, 0)

	// Second call should return the same table pointer
	htbl2 := JPEGStdHuffTable(cinfo, true, 0)
	if htbl1 != htbl2 {
		t.Error("JPEGStdHuffTable should reuse existing table")
	}
}
