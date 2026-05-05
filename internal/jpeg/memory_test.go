package jpeg

import "testing"

func TestAllocSmall(t *testing.T) {
	buf := AllocSmall(JPoolPermanent, 100)
	if len(buf) != 100 {
		t.Errorf("AllocSmall returned slice of length %d, want 100", len(buf))
	}
	// All bytes should be zero-initialized
	for i, b := range buf {
		if b != 0 {
			t.Errorf("AllocSmall: buf[%d] = %d, want 0", i, b)
		}
	}
}

func TestAllocSArray(t *testing.T) {
	samplesPerRow := 32
	numrows := 8
	arr := AllocSArray(samplesPerRow, numrows)

	if len(arr) != numrows {
		t.Fatalf("AllocSArray returned %d rows, want %d", len(arr), numrows)
	}
	for i, row := range arr {
		if len(row) != samplesPerRow {
			t.Errorf("AllocSArray row %d: length %d, want %d", i, len(row), samplesPerRow)
		}
	}
}

func TestAllocBArray(t *testing.T) {
	blocksPerRow := 8
	numrows := 4
	arr := AllocBArray(blocksPerRow, numrows)

	if len(arr) != numrows {
		t.Fatalf("AllocBArray returned %d rows, want %d", len(arr), numrows)
	}
	for i, row := range arr {
		if len(row) != blocksPerRow {
			t.Errorf("AllocBArray row %d: length %d, want %d", i, len(row), blocksPerRow)
		}
	}
}

func TestJPEGAllocQuantTable(t *testing.T) {
	qt := JPEGAllocQuantTable()
	if qt == nil {
		t.Fatal("JPEGAllocQuantTable returned nil")
	}
	// Verify zero-initialized
	for i, v := range qt.QuantVal {
		if v != 0 {
			t.Errorf("QuantVal[%d] = %d, want 0", i, v)
		}
	}
	if qt.SentTable {
		t.Error("SentTable should be false initially")
	}
}

func TestJPEGAllocHuffTable(t *testing.T) {
	ht := JPEGAllocHuffTable()
	if ht == nil {
		t.Fatal("JPEGAllocHuffTable returned nil")
	}
	// Verify zero-initialized
	for i, v := range ht.Bits {
		if v != 0 {
			t.Errorf("Bits[%d] = %d, want 0", i, v)
		}
	}
	for i, v := range ht.HuffVal {
		if v != 0 {
			t.Errorf("HuffVal[%d] = %d, want 0", i, v)
		}
	}
	if ht.SentTable {
		t.Error("SentTable should be false initially")
	}
}

func TestVirtSArray(t *testing.T) {
	samplesPerRow := 64
	numrows := 16
	maxAccess := 4

	vsa := RequestVirtSArray(samplesPerRow, numrows, maxAccess)
	if vsa == nil {
		t.Fatal("RequestVirtSArray returned nil")
	}
	if vsa.RowsInArray != numrows {
		t.Errorf("RowsInArray = %d, want %d", vsa.RowsInArray, numrows)
	}
	if vsa.SamplesPerRow != samplesPerRow {
		t.Errorf("SamplesPerRow = %d, want %d", vsa.SamplesPerRow, samplesPerRow)
	}
	if vsa.MaxAccess != maxAccess {
		t.Errorf("MaxAccess = %d, want %d", vsa.MaxAccess, maxAccess)
	}
	if vsa.PreZero != true {
		t.Error("PreZero should be true")
	}
	if vsa.MemBuffer != nil {
		t.Error("MemBuffer should be nil before realization")
	}

	// Realize
	RealizeVirtSArray(vsa)
	if vsa.MemBuffer == nil {
		t.Fatal("MemBuffer should not be nil after realization")
	}
	if vsa.RowsInMem != numrows {
		t.Errorf("RowsInMem = %d, want %d", vsa.RowsInMem, numrows)
	}

	// Double-realize should be a no-op
	RealizeVirtSArray(vsa)

	// Access rows
	rows := AccessVirtSArray(vsa, 0, maxAccess, true)
	if len(rows) != maxAccess {
		t.Errorf("AccessVirtSArray returned %d rows, want %d", len(rows), maxAccess)
	}

	// Verify pre-zeroed
	for r, row := range rows {
		for c, val := range row {
			if val != 0 {
				t.Errorf("row %d col %d = %d, want 0 (pre-zeroed)", r, c, val)
			}
		}
	}
}

func TestVirtSArrayWrite(t *testing.T) {
	vsa := RequestVirtSArray(8, 4, 2)
	RealizeVirtSArray(vsa)

	// Write data
	rows := AccessVirtSArray(vsa, 0, 2, true)
	rows[0][0] = 42
	rows[0][1] = 99

	// Read back
	rows2 := AccessVirtSArray(vsa, 0, 2, false)
	if rows2[0][0] != 42 {
		t.Errorf("expected 42, got %d", rows2[0][0])
	}
	if rows2[0][1] != 99 {
		t.Errorf("expected 99, got %d", rows2[0][1])
	}
}

func TestVirtSArrayAccessBounds(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for out-of-bounds access")
		}
	}()

	vsa := RequestVirtSArray(8, 4, 2)
	RealizeVirtSArray(vsa)
	// Access more rows than MaxAccess allows
	AccessVirtSArray(vsa, 0, 4, false)
}

func TestVirtBArray(t *testing.T) {
	blocksPerRow := 8
	numrows := 10
	maxAccess := 3

	vba := RequestVirtBArray(blocksPerRow, numrows, maxAccess)
	if vba == nil {
		t.Fatal("RequestVirtBArray returned nil")
	}
	if vba.RowsInArray != numrows {
		t.Errorf("RowsInArray = %d, want %d", vba.RowsInArray, numrows)
	}
	if vba.BlocksPerRow != blocksPerRow {
		t.Errorf("BlocksPerRow = %d, want %d", vba.BlocksPerRow, blocksPerRow)
	}
	if vba.MemBuffer != nil {
		t.Error("MemBuffer should be nil before realization")
	}

	// Realize
	RealizeVirtBArray(vba)
	if vba.MemBuffer == nil {
		t.Fatal("MemBuffer should not be nil after realization")
	}

	// Access rows
	rows := AccessVirtBArray(vba, 0, maxAccess, false)
	if len(rows) != maxAccess {
		t.Errorf("AccessVirtBArray returned %d rows, want %d", len(rows), maxAccess)
	}

	// Verify pre-zeroed
	for r, row := range rows {
		for b, block := range row {
			for c, val := range block {
				if val != 0 {
					t.Errorf("row %d block %d coef %d = %d, want 0 (pre-zeroed)",
						r, b, c, val)
				}
			}
		}
	}
}

func TestVirtBArrayWrite(t *testing.T) {
	vba := RequestVirtBArray(4, 4, 2)
	RealizeVirtBArray(vba)

	// Write data
	rows := AccessVirtBArray(vba, 0, 2, true)
	rows[0][0][0] = 100
	rows[0][0][1] = -50

	// Read back
	rows2 := AccessVirtBArray(vba, 0, 2, false)
	if rows2[0][0][0] != 100 {
		t.Errorf("expected 100, got %d", rows2[0][0][0])
	}
	if rows2[0][0][1] != -50 {
		t.Errorf("expected -50, got %d", rows2[0][0][1])
	}
}

func TestNewMemoryPools(t *testing.T) {
	mp := NewMemoryPools()
	if mp == nil {
		t.Fatal("NewMemoryPools returned nil")
	}
}

func TestInitMemoryMgr(t *testing.T) {
	cinfo := &JPEGCommon{}
	InitMemoryMgr(cinfo)
	if cinfo.Mem == nil {
		t.Fatal("Mem should not be nil after InitMemoryMgr")
	}
	if cinfo.Mem.MaxMemoryToUse != 1000000 {
		t.Errorf("MaxMemoryToUse = %d, want 1000000", cinfo.Mem.MaxMemoryToUse)
	}
}

func TestSelfDestruct(t *testing.T) {
	cinfo := &JPEGCommon{}
	InitMemoryMgr(cinfo)
	SelfDestruct(cinfo)
	if cinfo.Mem != nil {
		t.Error("Mem should be nil after SelfDestruct")
	}
}

func TestFreePool(t *testing.T) {
	// Should not panic (it's a no-op in Go)
	FreePool(JPoolPermanent)
	FreePool(JPoolImage)
}
