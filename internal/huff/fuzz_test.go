package huff

import (
	"testing"
)

// FuzzHuffmanDecode tests that Huffman decoding doesn't panic on arbitrary input.
func FuzzHuffmanDecode(f *testing.F) {
	// Build a simple DC table: 1 code of length 1, symbol 0
	dcTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	dcTbl.HuffVal[0] = 0

	// Build a simple AC table: 1 code of length 1, symbol 0 (EOB)
	acTbl := &HuffmanTable{
		Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	acTbl.HuffVal[0] = 0

	dcDerived, err := MakeDerivedHuffTable(dcTbl)
	if err != nil {
		f.Fatalf("MakeDerivedHuffTable DC: %v", err)
	}
	acDerived, err := MakeDerivedHuffTable(acTbl)
	if err != nil {
		f.Fatalf("MakeDerivedHuffTable AC: %v", err)
	}

	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xFF, 0x00, 0xFF, 0x00})
	f.Add([]byte{0xAA, 0x55, 0xAA, 0x55})
	f.Add([]byte{})
	f.Add([]byte{0x80})

	f.Fuzz(func(t *testing.T, data []byte) {
		var state BitReadWorkingState
		if len(data) > 0 {
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

			// Should not panic
			_ = DecodeMCUSequential(
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
		}
	})
}

// FuzzHuffmanTableBuild tests that table building doesn't panic on arbitrary data.
func FuzzHuffmanTableBuild(f *testing.F) {
	// Standard DC luminance table as seed
	f.Add([]byte{0, 0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, bits []byte) {
		var bitsArr [17]byte
		copy(bitsArr[:], bits)
		htbl := &HuffmanTable{Bits: bitsArr}
		// Fill huffval with valid-looking data
		for i := 0; i < 256; i++ {
			htbl.HuffVal[i] = byte(i)
		}
		// Should not panic
		_, _ = MakeDerivedHuffTable(htbl)
	})
}

// FuzzHuffDecodeFast tests the fast decode path with random data.
func FuzzHuffDecodeFast(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{0x12, 0x34, 0x56, 0x78})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		// Build a table with 2 codes of length 2
		htbl := &HuffmanTable{
			Bits: [17]uint8{0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		}
		htbl.HuffVal[0] = 0
		htbl.HuffVal[1] = 1

		dtbl, err := MakeDerivedHuffTable(htbl)
		if err != nil {
			return
		}

		var state BitReadWorkingState
		InitBitReader(&state, data)
		var unreadMarker byte

		// Try decoding a few symbols - should not panic
		for i := 0; i < 10; i++ {
			_, err := HuffDecodeFast(&state, dtbl, &unreadMarker)
			if err != nil {
				break
			}
		}
	})
}

// FuzzDecodeMCUDCFirst tests DC first decode with random data.
func FuzzDecodeMCUDCFirst(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x80, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		dcTbl := &HuffmanTable{
			Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		}
		dcTbl.HuffVal[0] = 0

		dcDerived, err := MakeDerivedHuffTable(dcTbl)
		if err != nil {
			return
		}

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

		_ = DecodeMCUDCFirst(
			&state, &permState, &savedState, blocks,
			[]*DerivedHuffTable{dcDerived},
			1, mcuMembership,
			0, 0,
			&restartsToGo, &insufficientData, &unreadMarker,
		)
	})
}

// FuzzDecodeMCUDCRefine tests DC refine decode with random data.
func FuzzDecodeMCUDCRefine(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		var state BitReadWorkingState
		InitBitReader(&state, data)
		var unreadMarker byte
		fillBitBuffer(&state, 0, &unreadMarker)

		permState := BitReadState{
			GetBuffer: state.GetBuffer,
			BitsLeft:  state.BitsLeft,
		}

		blocks := make([]Block, 1)
		restartsToGo := 1

		_ = DecodeMCUDCRefine(
			&state, &permState, blocks,
			1, 1, 0,
			&restartsToGo, &unreadMarker,
		)
	})
}

// FuzzDecodeMCUACFirst tests AC first decode with random data.
func FuzzDecodeMCUACFirst(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x80, 0x80, 0x80, 0x80})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		acTbl := &HuffmanTable{
			Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		}
		acTbl.HuffVal[0] = 0x00 // EOB

		acDerived, err := MakeDerivedHuffTable(acTbl)
		if err != nil {
			return
		}

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

		_ = DecodeMCUACFirst(
			&state, &permState, &savedState, block,
			acDerived,
			1, 63, 0,
			0,
			&restartsToGo, &insufficientData, &unreadMarker,
		)
	})
}

// FuzzDecodeMCUACRefine tests AC refine decode with random data.
func FuzzDecodeMCUACRefine(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xFF, 0x00, 0xFF, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		acTbl := &HuffmanTable{
			Bits: [17]uint8{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		}
		acTbl.HuffVal[0] = 0x00

		acDerived, err := MakeDerivedHuffTable(acTbl)
		if err != nil {
			return
		}

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

		_ = DecodeMCUACRefine(
			&state, &permState, &savedState, block,
			acDerived,
			1, 63, 0,
			0,
			&restartsToGo, &insufficientData, &unreadMarker,
		)
	})
}

// FuzzFillBitBuffer tests bit buffer filling with random data.
func FuzzFillBitBuffer(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xFF, 0x00, 0xFF, 0x00})
	f.Add([]byte{0xFF, 0xD9, 0x00, 0x00}) // EOI marker in data

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		var state BitReadWorkingState
		InitBitReader(&state, data)
		var unreadMarker byte

		// Should not panic
		fillBitBuffer(&state, 0, &unreadMarker)

		// Try to extract some bits
		if state.BitsLeft >= 8 {
			_ = GetBits(&state, 8)
		}
	})
}

// FuzzArithDecoder tests arithmetic decoder initialization.
func FuzzArithDecoder(f *testing.F) {
	f.Add(false)
	f.Add(true)

	f.Fuzz(func(t *testing.T, progressive bool) {
		dec := NewArithDecoder()
		compInfos := []ComponentInfo{
			{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
		}
		dec.InitPass(progressive, 1, compInfos, 63, 0, 63, 0, 0)
	})
}

// TestArithDecoderInit tests basic arithmetic decoder initialization.
func TestArithDecoderInit(t *testing.T) {
	dec := NewArithDecoder()
	if dec == nil {
		t.Fatal("NewArithDecoder returned nil")
	}
	if dec.State.FixedBin[0] != 113 {
		t.Errorf("FixedBin[0] = %d, want 113", dec.State.FixedBin[0])
	}
}

// TestArithDecoderInitPass tests InitPass.
func TestArithDecoderInitPass(t *testing.T) {
	dec := NewArithDecoder()
	compInfos := []ComponentInfo{
		{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
	}
	dec.InitPass(false, 1, compInfos, 63, 0, 63, 0, 0)
	if dec.State.DCStats[0] == nil {
		t.Error("DCStats[0] should be allocated")
	}
	if dec.State.ACStats[0] == nil {
		t.Error("ACStats[0] should be allocated")
	}
	if dec.State.Ct != -16 {
		t.Errorf("Ct = %d, want -16", dec.State.Ct)
	}
}

// TestArithDecoderInitPassProgressive tests InitPass in progressive mode.
func TestArithDecoderInitPassProgressive(t *testing.T) {
	dec := NewArithDecoder()
	compInfos := []ComponentInfo{
		{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
	}
	dec.InitPass(true, 1, compInfos, 63, 0, 63, 0, 0)
	// DC pass (Ss=0, Ah=0)
	if dec.State.DCStats[0] == nil {
		t.Error("DCStats[0] should be allocated for DC pass")
	}
}

// TestArithDecoderInitPassProgressiveAC tests InitPass for AC scan.
func TestArithDecoderInitPassProgressiveAC(t *testing.T) {
	dec := NewArithDecoder()
	compInfos := []ComponentInfo{
		{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
	}
	dec.InitPass(true, 1, compInfos, 63, 1, 63, 0, 0)
	// AC pass (Ss=1)
	if dec.State.ACStats[0] == nil {
		t.Error("ACStats[0] should be allocated for AC pass")
	}
}

// TestArithDecoderResetStats tests ResetStats.
func TestArithDecoderResetStats(t *testing.T) {
	dec := NewArithDecoder()
	compInfos := []ComponentInfo{
		{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
	}
	dec.InitPass(false, 1, compInfos, 63, 0, 63, 0, 0)

	// Modify some state
	dec.State.LastDCVal[0] = 42
	dec.State.DCContext[0] = 5

	dec.ResetStats(1, compInfos, false, 0, 0, 63)
	if dec.State.LastDCVal[0] != 0 {
		t.Errorf("LastDCVal[0] = %d, want 0 after reset", dec.State.LastDCVal[0])
	}
	if dec.State.DCContext[0] != 0 {
		t.Errorf("DCContext[0] = %d, want 0 after reset", dec.State.DCContext[0])
	}
	if dec.State.C != 0 || dec.State.A != 0 {
		t.Error("C and A should be reset to 0")
	}
}

// TestArithDecoderResetStatsProgressive tests ResetStats in progressive mode.
func TestArithDecoderResetStatsProgressive(t *testing.T) {
	dec := NewArithDecoder()
	compInfos := []ComponentInfo{
		{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
	}
	dec.InitPass(true, 1, compInfos, 63, 0, 63, 0, 0)
	dec.State.LastDCVal[0] = 42

	dec.ResetStats(1, compInfos, true, 0, 0, 63)
	if dec.State.LastDCVal[0] != 0 {
		t.Errorf("LastDCVal[0] = %d, want 0 after reset", dec.State.LastDCVal[0])
	}
}

// TestArithDecoderSequential tests the sequential decode path.
func TestArithDecoderSequential(t *testing.T) {
	dec := NewArithDecoder()
	compInfos := []ComponentInfo{
		{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
	}
	dec.InitPass(false, 1, compInfos, 63, 0, 63, 0, 0)

	// Feed initial bytes for the arithmetic decoder
	data := make([]byte, 256)
	for i := range data {
		data[i] = 0x00
	}
	idx := 0
	src := func() byte {
		if idx < len(data) {
			b := data[idx]
			idx++
			return b
		}
		return 0
	}

	var arithDCL [NumArithTbls]uint8
	var arithDCU [NumArithTbls]uint8
	var arithACK [NumArithTbls]int
	var unreadMarker byte

	blocks := make([]Block, 1)
	mcuMembership := []int{0}

	ok := dec.DecodeMCUSequential(blocks, 1, mcuMembership, compInfos, 0, 63, 63, arithDCL, arithDCU, arithACK, src, &unreadMarker)
	if !ok {
		t.Error("DecodeMCUSequential should return true")
	}
}

// TestArithDecoderSequentialError tests sequential decode with Ct=-1 (error state).
func TestArithDecoderSequentialError(t *testing.T) {
	dec := NewArithDecoder()
	compInfos := []ComponentInfo{
		{DCTblNo: 0, ACTblNo: 0, ComponentNeeded: true},
	}
	dec.InitPass(false, 1, compInfos, 63, 0, 63, 0, 0)
	dec.State.Ct = -1 // Force error state

	src := func() byte { return 0 }
	var arithDCL [NumArithTbls]uint8
	var arithDCU [NumArithTbls]uint8
	var arithACK [NumArithTbls]int
	var unreadMarker byte

	blocks := make([]Block, 1)
	mcuMembership := []int{0}

	ok := dec.DecodeMCUSequential(blocks, 1, mcuMembership, compInfos, 0, 63, 63, arithDCL, arithDCU, arithACK, src, &unreadMarker)
	if !ok {
		t.Error("DecodeMCUSequential should return true even in error state")
	}
}

// TestIDCTISlowNegativeDC tests IDCT with negative DC coefficient.

// TestIDCTISlowMultipleCoefficients tests IDCT with several non-zero coefficients.

// TestIDCTIFastMultipleCoefficients tests IDCT IFAST with multiple coefficients.

// TestIDCTFloatMultipleCoefficients tests IDCT Float with multiple coefficients.

// TestDecodeMCUSequentialWithZRL tests sequential decode with ZRL (zero run length).

// TestDecodeMCUACFirstWithEOBRUN tests AC first with EOBRUN.

// TestDecodeMCUDCFirstWithRestartInterval tests DC first with restart interval.

// TestDecodeMCUACFirstWithRestartInterval tests AC first with restart interval.

// TestDecodeMCUACRefineWithRestartInterval tests AC refine with restart interval.

// TestDecodeMCUDCRefineWithRestartInterval tests DC refine with restart interval.

// TestDecodeMCUACFirstInsufficientData tests AC first with insufficient data flag.

// TestDecodeMCUACRefineInsufficientData tests AC refine with insufficient data.

// TestDecodeMCUDCFirstInsufficientData tests DC first with insufficient data.

// TestClipFunction tests the Clip helper.

// TestNewRangeLimitTable tests the range limit table creation.
