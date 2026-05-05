package huff

import "testing"

func TestMakeDerivedHuffTable(t *testing.T) {
	// Standard DC luminance table
	htbl := &HuffmanTable{
		Bits: [17]uint8{0, 0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0},
	}
	for i, v := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11} {
		htbl.HuffVal[i] = v
	}

	dtbl, err := MakeDerivedHuffTable(htbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable error: %v", err)
	}
	if dtbl == nil {
		t.Fatal("MakeDerivedHuffTable returned nil")
	}
	if dtbl.Pub != htbl {
		t.Error("Pub should point back to original table")
	}

	// Verify MaxCode entries are set
	hasCode := false
	for l := 1; l <= 16; l++ {
		if dtbl.MaxCode[l] >= 0 {
			hasCode = true
		}
	}
	if !hasCode {
		t.Error("expected at least one non-negative MaxCode entry")
	}

	// Verify sentinel
	if dtbl.MaxCode[17] != 0xFFFFF {
		t.Errorf("MaxCode[17] = %d, want 0xFFFFF", dtbl.MaxCode[17])
	}

	// Verify lookahead table has entries
	nonZero := 0
	for _, nb := range dtbl.LookNBits {
		if nb != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Error("expected non-zero entries in LookNBits")
	}
}

func TestMakeDerivedHuffTableSimple(t *testing.T) {
	// Simple table: 1 code of length 1, symbol = 42
	htbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	htbl.HuffVal[0] = 42

	dtbl, err := MakeDerivedHuffTable(htbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable error: %v", err)
	}

	// Code 0 (1 bit) should map to symbol 42
	// LookNBits should show 1 bit for both possible 8-bit prefixes
	// that start with bit 0 (codes 0-127) and 1 bit for codes 128-255
	// With code 0 (1 bit), all lookBits values where top bit is 0 should
	// have LookSym=42, LookNBits=1
	for look := 0; look < 128; look++ {
		if dtbl.LookSym[look] != 42 || dtbl.LookNBits[look] != 1 {
			t.Errorf("LookSym[%d]=%d, LookNBits[%d]=%d; want 42, 1",
				look, dtbl.LookSym[look], look, dtbl.LookNBits[look])
		}
	}
}

func TestMakeDerivedHuffTableInvalid(t *testing.T) {
	tests := []struct {
		name string
		tbl  *HuffmanTable
	}{
		{
			name: "too many symbols",
			tbl: func() *HuffmanTable {
				t := &HuffmanTable{}
				// Set bits to total > 256
				for i := 1; i <= 16; i++ {
					t.Bits[i] = 17
				}
				return t
			}(),
		},
		{
			name: "overflow code at length",
			tbl: func() *HuffmanTable {
				t := &HuffmanTable{}
				// 2 codes of length 1 => codes 0 and 1
				// Then 1 code of length 2 => code 10, but after shift code space = 100 = 4
				// which is >= 4 = 1 << 2, so should fail
				t.Bits[1] = 2
				t.Bits[2] = 1
				t.HuffVal[0] = 0
				t.HuffVal[1] = 1
				t.HuffVal[2] = 2
				return t
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MakeDerivedHuffTable(tt.tbl)
			if err == nil {
				t.Error("expected error for invalid table")
			}
			if err != ErrBadHuffTable {
				t.Errorf("error = %v, want ErrBadHuffTable", err)
			}
		})
	}
}

func TestHuffExtend(t *testing.T) {
	tests := []struct {
		x, s int
		want int
	}{
		// s=1: bmask[0]=0, bmask[1]=1. x<=0 means x-1. x=0 => -1, x=1 => 1
		{0, 1, -1},
		{1, 1, 1},
		// s=2: bmask[1]=1, bmask[2]=3. x<=1 means x-3.
		{0, 2, -3},
		{1, 2, -2},
		{2, 2, 2},
		{3, 2, 3},
		// s=3: bmask[2]=3, bmask[3]=7. x<=3 means x-7.
		{0, 3, -7},
		{3, 3, -4},
		{4, 3, 4},
		{7, 3, 7},
		// s=8:
		{0, 8, -255},
		{127, 8, -128},
		{128, 8, 128},
		{255, 8, 255},
		// s=11 (DC coefficient max category):
		{0, 11, -2047},
		{1023, 11, -1024},
		{1024, 11, 1024},
		{2047, 11, 2047},
	}
	for _, tt := range tests {
		got := HuffExtend(tt.x, tt.s)
		if got != tt.want {
			t.Errorf("HuffExtend(%d, %d) = %d, want %d", tt.x, tt.s, got, tt.want)
		}
	}
}

func TestHuffExtendNegativeValue(t *testing.T) {
	// Additional tests: s=4, bmask[3]=7, bmask[4]=15
	// Values 0..7 are negative (add -15), values 8..15 are positive
	for x := 0; x <= 7; x++ {
		got := HuffExtend(x, 4)
		want := x - 15
		if got != want {
			t.Errorf("HuffExtend(%d, 4) = %d, want %d", x, got, want)
		}
	}
	for x := 8; x <= 15; x++ {
		got := HuffExtend(x, 4)
		if got != x {
			t.Errorf("HuffExtend(%d, 4) = %d, want %d", x, got, x)
		}
	}
}

func TestInitBitReader(t *testing.T) {
	var state BitReadWorkingState
	data := []byte{0xFF, 0x00, 0x12, 0x34}
	InitBitReader(&state, data)

	if state.BytesInBuffer != len(data) {
		t.Errorf("BytesInBuffer = %d, want %d", state.BytesInBuffer, len(data))
	}
	if state.GetBuffer != 0 {
		t.Errorf("GetBuffer = %d, want 0", state.GetBuffer)
	}
	if state.BitsLeft != 0 {
		t.Errorf("BitsLeft = %d, want 0", state.BitsLeft)
	}
}

func makeSimpleDHT() *DerivedHuffTable {
	// Build a simple Huffman table:
	// 2 codes of length 2: code 00 -> symbol 0, code 01 -> symbol 1
	htbl := &HuffmanTable{
		Bits: [17]uint8{0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	htbl.HuffVal[0] = 0
	htbl.HuffVal[1] = 1

	dtbl, err := MakeDerivedHuffTable(htbl)
	if err != nil {
		panic(err)
	}
	return dtbl
}

func TestHuffDecodeFast(t *testing.T) {
	dtbl := makeSimpleDHT()
	// Table: 2 codes of length 2
	// code 00 (2 bits) -> symbol 0
	// code 01 (2 bits) -> symbol 1

	// Bit stream: 00 01 00 = 0001_00xx
	// Left-justified in bytes: 0001_0000 = 0x10, pad zeros
	data := []byte{0x10, 0x00, 0x00, 0x00}

	var state BitReadWorkingState
	InitBitReader(&state, data)
	var unreadMarker byte

	// Decode symbol 0: bits = 00 -> symbol 0
	sym, err := HuffDecodeFast(&state, dtbl, &unreadMarker)
	if err != nil {
		t.Fatalf("HuffDecodeFast error: %v", err)
	}
	if sym != 0 {
		t.Errorf("first symbol = %d, want 0", sym)
	}

	// Decode symbol 1: bits = 01 -> symbol 1
	sym, err = HuffDecodeFast(&state, dtbl, &unreadMarker)
	if err != nil {
		t.Fatalf("HuffDecodeFast error: %v", err)
	}
	if sym != 1 {
		t.Errorf("second symbol = %d, want 1", sym)
	}

	// Decode symbol 0 again: bits = 00 -> symbol 0
	sym, err = HuffDecodeFast(&state, dtbl, &unreadMarker)
	if err != nil {
		t.Fatalf("HuffDecodeFast error: %v", err)
	}
	if sym != 0 {
		t.Errorf("third symbol = %d, want 0", sym)
	}
}

func TestDecodeMCUSequentialBasic(t *testing.T) {
	// Create a simple DC-only Huffman table: 1 code of length 2, symbol 0 (category 0 = value 0)
	dcTbl := &HuffmanTable{
		Bits: [17]uint8{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	dcTbl.HuffVal[0] = 0 // category 0 = DC coefficient is 0

	dcDerived, err := MakeDerivedHuffTable(dcTbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable DC: %v", err)
	}

	// AC table: EOB (0x00) code
	acTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	acTbl.HuffVal[0] = 0x00 // EOB

	acDerived, err := MakeDerivedHuffTable(acTbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable AC: %v", err)
	}

	// Compressed data: DC code "00" (2 bits) + AC EOB code "0" (1 bit) = 000xxxxx
	// Byte: 0000_0000 = 0x00
	data := []byte{0x00, 0x00, 0x00, 0x00}

	var state BitReadWorkingState
	InitBitReader(&state, data)

	// Pre-load the bit buffer manually by calling fillBitBuffer
	var unreadMarker byte
	fillBitBuffer(&state, 0, &unreadMarker)

	permState := BitReadState{
		GetBuffer: state.GetBuffer,
		BitsLeft:  state.BitsLeft,
	}

	savedState := SavableState{}
	blocks := make([]Block, 1)
	mcuMembership := []int{0}
	insufficientData := false
	restartsToGo := 1

	ok := DecodeMCUSequential(
		&state,
		&permState,
		&savedState,
		blocks,
		[]*DerivedHuffTable{dcDerived},
		[]*DerivedHuffTable{acDerived},
		1,           // blocksInMCU
		mcuMembership,
		0,           // restartInterval
		&restartsToGo,
		&insufficientData,
		&unreadMarker,
	)

	if !ok {
		t.Fatal("DecodeMCUSequential returned false")
	}

	// With DC category 0, DC value should be 0
	if blocks[0][0] != 0 {
		t.Errorf("block[0] = %d, want 0", blocks[0][0])
	}

	// All AC coefficients should be 0 (from EOB)
	for k := 1; k < DCTSize2; k++ {
		if blocks[0][k] != 0 {
			t.Errorf("block[%d] = %d, want 0 (AC should be zero from EOB)", k, blocks[0][k])
		}
	}
}

func TestDecodeMCUSequentialWithDCValue(t *testing.T) {
	// DC table: category 2 (value in -3..3 range) -> code length 2
	// AC table: EOB
	dcTbl := &HuffmanTable{
		Bits: [17]uint8{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	dcTbl.HuffVal[0] = 2 // category 2: 2 additional bits

	dcDerived, err := MakeDerivedHuffTable(dcTbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable DC: %v", err)
	}

	acTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	acTbl.HuffVal[0] = 0x00 // EOB

	acDerived, err := MakeDerivedHuffTable(acTbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable AC: %v", err)
	}

	// DC code "00" (2 bits) -> category 2
	// Additional bits: "11" (2 bits) -> value 3 (since HuffExtend(3,2)=3)
	// AC EOB code "0" (1 bit)
	// Bitstream: 00 11 0 = 00110_xxx
	// Byte: 0011_0000 = 0x30
	data := []byte{0x30, 0x00, 0x00, 0x00}

	var state BitReadWorkingState
	InitBitReader(&state, data)

	var unreadMarker byte
	fillBitBuffer(&state, 0, &unreadMarker)

	permState := BitReadState{
		GetBuffer: state.GetBuffer,
		BitsLeft:  state.BitsLeft,
	}

	savedState := SavableState{}
	blocks := make([]Block, 1)
	mcuMembership := []int{0}
	insufficientData := false
	restartsToGo := 1

	ok := DecodeMCUSequential(
		&state,
		&permState,
		&savedState,
		blocks,
		[]*DerivedHuffTable{dcDerived},
		[]*DerivedHuffTable{acDerived},
		1,
		mcuMembership,
		0,
		&restartsToGo,
		&insufficientData,
		&unreadMarker,
	)

	if !ok {
		t.Fatal("DecodeMCUSequential returned false")
	}

	// DC value should be 3
	if blocks[0][0] != 3 {
		t.Errorf("block[0] = %d, want 3", blocks[0][0])
	}
}

func TestDecodeMCUSequentialRestartInterval(t *testing.T) {
	dcTbl := &HuffmanTable{
		Bits: [17]uint8{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	dcTbl.HuffVal[0] = 0

	acTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	acTbl.HuffVal[0] = 0x00

	dcDerived, _ := MakeDerivedHuffTable(dcTbl)
	acDerived, _ := MakeDerivedHuffTable(acTbl)

	var state BitReadWorkingState
	var permState BitReadState
	savedState := SavableState{}
	blocks := make([]Block, 1)
	mcuMembership := []int{0}
	insufficientData := false
	var unreadMarker byte

	// Test that restartsToGo=0 returns false
	restartsToGo := 0
	ok := DecodeMCUSequential(
		&state, &permState, &savedState, blocks,
		[]*DerivedHuffTable{dcDerived}, []*DerivedHuffTable{acDerived},
		1, mcuMembership,
		10, // restartInterval > 0
		&restartsToGo, &insufficientData, &unreadMarker,
	)
	if ok {
		t.Error("expected false when restartsToGo=0")
	}
}

func TestDecodeMCUDCFirst(t *testing.T) {
	// DC table: 1 code of length 1, symbol 0 (category 0 = value 0)
	dcTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	dcTbl.HuffVal[0] = 0

	dcDerived, err := MakeDerivedHuffTable(dcTbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable: %v", err)
	}

	data := []byte{0x00, 0x00, 0x00, 0x00}
	var state BitReadWorkingState
	InitBitReader(&state, data)
	var unreadMarker byte
	fillBitBuffer(&state, 0, &unreadMarker)

	permState := BitReadState{
		GetBuffer: state.GetBuffer,
		BitsLeft:  state.BitsLeft,
	}

	savedState := SavableState{}
	blocks := make([]Block, 1)
	mcuMembership := []int{0}
	insufficientData := false
	restartsToGo := 1

	ok := DecodeMCUDCFirst(
		&state, &permState, &savedState, blocks,
		[]*DerivedHuffTable{dcDerived},
		1, mcuMembership,
		0, // Al = 0
		0, // restartInterval
		&restartsToGo, &insufficientData, &unreadMarker,
	)
	if !ok {
		t.Fatal("DecodeMCUDCFirst returned false")
	}
	// DC category 0, value 0, with Al=0
	if blocks[0][0] != 0 {
		t.Errorf("block[0] = %d, want 0", blocks[0][0])
	}
}

func TestDecodeMCUDCRefine(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00}
	var state BitReadWorkingState
	InitBitReader(&state, data)
	var unreadMarker byte
	fillBitBuffer(&state, 0, &unreadMarker)

	permState := BitReadState{
		GetBuffer: state.GetBuffer,
		BitsLeft:  state.BitsLeft,
	}

	blocks := make([]Block, 1)
	blocks[0][0] = 10 // initial DC value
	restartsToGo := 1

	ok := DecodeMCUDCRefine(
		&state, &permState, blocks,
		1, // blocksInMCU
		1, // Al = 1 (refine by adding bit 1 or 0 to position 1)
		0, // restartInterval
		&restartsToGo, &unreadMarker,
	)
	if !ok {
		t.Fatal("DecodeMCUDCRefine returned false")
	}
	// Bit is 0 (from data 0x00), so block[0][0] should remain 10
	if blocks[0][0] != 10 {
		t.Errorf("block[0] = %d, want 10 (bit was 0)", blocks[0][0])
	}
}

func TestDecodeMCUACFirst(t *testing.T) {
	// AC table: EOB (symbol 0x00) at code 0 (1 bit)
	acTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	acTbl.HuffVal[0] = 0x00 // EOB

	acDerived, err := MakeDerivedHuffTable(acTbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable: %v", err)
	}

	data := []byte{0x00, 0x00, 0x00, 0x00}
	var state BitReadWorkingState
	InitBitReader(&state, data)
	var unreadMarker byte
	fillBitBuffer(&state, 0, &unreadMarker)

	permState := BitReadState{
		GetBuffer: state.GetBuffer,
		BitsLeft:  state.BitsLeft,
	}

	savedState := SavableState{}
	block := &Block{}
	insufficientData := false
	restartsToGo := 1

	ok := DecodeMCUACFirst(
		&state, &permState, &savedState, block,
		acDerived,
		1, 63, 0, // Ss=1, Se=63, Al=0
		0, // restartInterval
		&restartsToGo, &insufficientData, &unreadMarker,
	)
	if !ok {
		t.Fatal("DecodeMCUACFirst returned false")
	}
	// EOB received, all AC should be 0
	for k := 1; k < DCTSize2; k++ {
		if block[k] != 0 {
			t.Errorf("block[%d] = %d, want 0 after EOB", k, block[k])
		}
	}
}

func TestDecodeMCUACRefine(t *testing.T) {
	// AC table: EOB (symbol 0x00) at code 0 (1 bit)
	acTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	acTbl.HuffVal[0] = 0x00 // EOB

	acDerived, err := MakeDerivedHuffTable(acTbl)
	if err != nil {
		t.Fatalf("MakeDerivedHuffTable: %v", err)
	}

	data := []byte{0x00, 0x00, 0x00, 0x00}
	var state BitReadWorkingState
	InitBitReader(&state, data)
	var unreadMarker byte
	fillBitBuffer(&state, 0, &unreadMarker)

	permState := BitReadState{
		GetBuffer: state.GetBuffer,
		BitsLeft:  state.BitsLeft,
	}

	savedState := SavableState{}
	block := &Block{}
	insufficientData := false
	restartsToGo := 1

	ok := DecodeMCUACRefine(
		&state, &permState, &savedState, block,
		acDerived,
		1, 63, 0, // Ss=1, Se=63, Al=0
		0, // restartInterval
		&restartsToGo, &insufficientData, &unreadMarker,
	)
	if !ok {
		t.Fatal("DecodeMCUACRefine returned false")
	}
	// All coefficients should remain 0 (block was all zero, EOB received)
	for k := 1; k < DCTSize2; k++ {
		if block[k] != 0 {
			t.Errorf("block[%d] = %d, want 0", k, block[k])
		}
	}
}

func TestDecodeMCUSequentialInsufficientData(t *testing.T) {
	// When insufficientData is true, DecodeMCUSequential should skip decoding
	dcTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	dcTbl.HuffVal[0] = 0

	acTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	acTbl.HuffVal[0] = 0x00

	dcDerived, _ := MakeDerivedHuffTable(dcTbl)
	acDerived, _ := MakeDerivedHuffTable(acTbl)

	var state BitReadWorkingState
	var permState BitReadState
	savedState := SavableState{}
	blocks := make([]Block, 1)
	mcuMembership := []int{0}
	insufficientData := true // all data lost
	var unreadMarker byte
	restartsToGo := 1

	ok := DecodeMCUSequential(
		&state, &permState, &savedState, blocks,
		[]*DerivedHuffTable{dcDerived}, []*DerivedHuffTable{acDerived},
		1, mcuMembership,
		0, &restartsToGo, &insufficientData, &unreadMarker,
	)
	if !ok {
		t.Fatal("DecodeMCUSequential should return true with insufficientData=true")
	}
	// Block should remain all zeros
	for k := 0; k < DCTSize2; k++ {
		if blocks[0][k] != 0 {
			t.Errorf("block[%d] = %d, want 0 (insufficientData)", k, blocks[0][k])
		}
	}
}

func TestGetBits(t *testing.T) {
	var state BitReadWorkingState
	// Load 0xA5 = 10100101 into getBuffer
	state.GetBuffer = 0xA5000000
	state.BitsLeft = 32

	// Get top 4 bits: 1010 = 10
	val := GetBits(&state, 4)
	if val != 0xA {
		t.Errorf("GetBits(4) = %d, want 10", val)
	}

	// Get next 4 bits: 0101 = 5
	val = GetBits(&state, 4)
	if val != 5 {
		t.Errorf("GetBits(4) = %d, want 5", val)
	}

	if state.BitsLeft != 24 {
		t.Errorf("BitsLeft = %d, want 24", state.BitsLeft)
	}
}

func TestPeekBits(t *testing.T) {
	var state BitReadWorkingState
	state.GetBuffer = 0xFF000000
	state.BitsLeft = 32

	// Peek 8 bits should give 0xFF without consuming
	val := PeekBits(&state, 8)
	if val != 0xFF {
		t.Errorf("PeekBits(8) = %d, want 255", val)
	}

	if state.BitsLeft != 32 {
		t.Errorf("PeekBits should not consume bits; BitsLeft = %d, want 32", state.BitsLeft)
	}
}
