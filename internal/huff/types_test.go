package huff

import "testing"

func TestNewRangeLimitTable(t *testing.T) {
	tbl := NewRangeLimitTable()
	if tbl == nil {
		t.Fatal("NewRangeLimitTable returned nil")
	}

	// RangeSubset = 384, RangeCenter = 512, RangeMask = 1023

	// Index 0..383: clamp to 0
	if tbl[0] != 0 {
		t.Errorf("tbl[0] = %d, want 0", tbl[0])
	}
	if tbl[383] != 0 {
		t.Errorf("tbl[383] = %d, want 0", tbl[383])
	}

	// Index 384..639: identity mapping (0..255)
	if tbl[384] != 0 {
		t.Errorf("tbl[384] = %d, want 0", tbl[384])
	}
	if tbl[512] != 128 {
		t.Errorf("tbl[512] = %d, want 128", tbl[512])
	}
	if tbl[639] != 255 {
		t.Errorf("tbl[639] = %d, want 255", tbl[639])
	}

	// Index 640..1023: clamp to 255
	if tbl[640] != 255 {
		t.Errorf("tbl[640] = %d, want 255", tbl[640])
	}
	if tbl[1023] != 255 {
		t.Errorf("tbl[1023] = %d, want 255", tbl[1023])
	}

	// Safety entries 1024..1279: 0..255
	if tbl[1024] != 0 {
		t.Errorf("tbl[1024] = %d, want 0", tbl[1024])
	}
	if tbl[1279] != 255 {
		t.Errorf("tbl[1279] = %d, want 255", tbl[1279])
	}
}

func TestNewRangeLimitTableGradient(t *testing.T) {
	tbl := NewRangeLimitTable()

	// Index 384..639: identity mapping (0..255)
	for i := 0; i <= 255; i++ {
		idx := RangeSubset + i // 384 + i
		want := JSAMPLE(i)
		if tbl[idx] != want {
			t.Errorf("tbl[%d] = %d, want %d", idx, tbl[idx], want)
		}
	}

	// Safety entries 1024..1279: identity mapping (0..255)
	for i := 0; i <= 255; i++ {
		want := JSAMPLE(i)
		if tbl[1024+i] != want {
			t.Errorf("tbl[%d] = %d, want %d", 1024+i, tbl[1024+i], want)
		}
	}
}

func TestRangeLimitTableWrapping(t *testing.T) {
	tbl := NewRangeLimitTable()

	// The IDCT uses: value & RangeMask (1023)
	// RangeSubset=384, RangeCenter=512

	// A value of 512 (RangeCenter) should map to 128
	if tbl[512&RangeMask] != 128 {
		t.Errorf("tbl[512 & 1023] = %d, want 128", tbl[512&RangeMask])
	}

	// A large positive value like 700: 700 & 1023 = 700 -> tbl[700] = 255 (clamp)
	if tbl[700&RangeMask] != 255 {
		t.Errorf("tbl[700 & 1023] = %d, want 255", tbl[700&RangeMask])
	}

	// A value of 384 should map to 0
	if tbl[384] != 0 {
		t.Errorf("tbl[384] = %d, want 0", tbl[384])
	}

	// A value of 639 should map to 255
	if tbl[639] != 255 {
		t.Errorf("tbl[639] = %d, want 255", tbl[639])
	}
}

func TestClip(t *testing.T) {
	tests := []struct {
		v    int
		want JSAMPLE
	}{
		{0, 0},
		{128, 128},
		{255, 255},
		{256, 255},
		{1000, 255},
		{-1, 0},
		{-100, 0},
		{50, 50},
	}
	for _, tt := range tests {
		got := Clip(tt.v)
		if got != tt.want {
			t.Errorf("Clip(%d) = %d, want %d", tt.v, got, tt.want)
		}
	}
}

func TestBlockType(t *testing.T) {
	// Verify Block is 64 elements
	var b Block
	if len(b) != DCTSize2 {
		t.Errorf("Block length = %d, want %d", len(b), DCTSize2)
	}

	// Verify zero-initialized
	for i, v := range b {
		if v != 0 {
			t.Errorf("Block[%d] = %d, want 0", i, v)
		}
	}
}

func TestHuffmanTableType(t *testing.T) {
	ht := &HuffmanTable{}
	if len(ht.Bits) != 17 {
		t.Errorf("Bits length = %d, want 17", len(ht.Bits))
	}
	if len(ht.HuffVal) != 256 {
		t.Errorf("HuffVal length = %d, want 256", len(ht.HuffVal))
	}
	if ht.SentTable {
		t.Error("SentTable should be false by default")
	}
}

func TestDerivedHuffTableType(t *testing.T) {
	var dt DerivedHuffTable
	if len(dt.MaxCode) != 18 {
		t.Errorf("MaxCode length = %d, want 18", len(dt.MaxCode))
	}
	if len(dt.ValOffset) != 17 {
		t.Errorf("ValOffset length = %d, want 17", len(dt.ValOffset))
	}
	if len(dt.LookNBits) != (1 << HuffLookahead) {
		t.Errorf("LookNBits length = %d, want %d", len(dt.LookNBits), 1<<HuffLookahead)
	}
	if len(dt.LookSym) != (1 << HuffLookahead) {
		t.Errorf("LookSym length = %d, want %d", len(dt.LookSym), 1<<HuffLookahead)
	}
}

func TestBitReadWorkingStateType(t *testing.T) {
	var s BitReadWorkingState
	if s.BytesInBuffer != 0 {
		t.Error("BytesInBuffer should be 0 by default")
	}
	if s.GetBuffer != 0 {
		t.Error("GetBuffer should be 0 by default")
	}
	if s.BitsLeft != 0 {
		t.Error("BitsLeft should be 0 by default")
	}
}

func TestSavableStateType(t *testing.T) {
	var s SavableState
	if s.EOBRUN != 0 {
		t.Error("EOBRUN should be 0 by default")
	}
	for i, v := range s.LastDCVal {
		if v != 0 {
			t.Errorf("LastDCVal[%d] = %d, want 0", i, v)
		}
	}
}

func TestNaturalOrderLength(t *testing.T) {
	// NaturalOrder should have at least 80 entries (64 + 16 guard)
	if len(NaturalOrder) < 80 {
		t.Errorf("NaturalOrder length = %d, want >= 80", len(NaturalOrder))
	}
	// First entry should be 0 (DC coefficient)
	if NaturalOrder[0] != 0 {
		t.Errorf("NaturalOrder[0] = %d, want 0", NaturalOrder[0])
	}
	// Last valid entry should be 63
	if NaturalOrder[63] != 63 {
		t.Errorf("NaturalOrder[63] = %d, want 63", NaturalOrder[63])
	}
	// Guard entries should be 63
	for i := 64; i < 80; i++ {
		if NaturalOrder[i] != 63 {
			t.Errorf("NaturalOrder[%d] = %d, want 63 (guard)", i, NaturalOrder[i])
		}
	}
}

func TestConstants(t *testing.T) {
	if DCTSize != 8 {
		t.Errorf("DCTSize = %d, want 8", DCTSize)
	}
	if DCTSize2 != 64 {
		t.Errorf("DCTSize2 = %d, want 64", DCTSize2)
	}
	if RangeCenter != 512 {
		t.Errorf("RangeCenter = %d, want 512", RangeCenter)
	}
	if RangeMask != 1023 {
		t.Errorf("RangeMask = %d, want 1023", RangeMask)
	}
	if MaxJSample != 255 {
		t.Errorf("MaxJSample = %d, want 255", MaxJSample)
	}
	if HuffLookahead != 8 {
		t.Errorf("HuffLookahead = %d, want 8", HuffLookahead)
	}
	if BitBufSize != 32 {
		t.Errorf("BitBufSize = %d, want 32", BitBufSize)
	}
}
