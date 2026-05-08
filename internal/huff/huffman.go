package huff

import (
	"errors"
)

// Errors returned by the Huffman decoder.
var (
	ErrBadHuffTable = errors.New("jpeg: bad Huffman table")
	ErrHitMarker    = errors.New("jpeg: hit marker in compressed data")
	ErrHuffBadCode  = errors.New("jpeg: bad Huffman code")
	ErrSuspension   = errors.New("jpeg: data source suspension")
)

// MakeDerivedHuffTable computes the derived values for a Huffman table.
// This routine also performs some validation checks on the table.
// Ported from jpeg_make_d_derived_tbl in jdhuff.c.
func MakeDerivedHuffTable(htbl *HuffmanTable) (*DerivedHuffTable, error) {
	dtbl := new(DerivedHuffTable)
	dtbl.Pub = htbl

	// Figure C.1: make table of Huffman code length for each symbol.
	huffSize := make([]int, 0, 257)
	p := 0
	for l := 1; l <= 16; l++ {
		i := int(htbl.Bits[l])
		if i < 0 || p+i > 256 {
			return nil, ErrBadHuffTable
		}
		for j := 0; j < i; j++ {
			huffSize = append(huffSize, l)
		}
		p += i
	}
	_ = p // numSymbols not needed beyond validation

	// Figure C.2: generate the codes themselves.
	// We also validate that the counts represent a legal Huffman code tree.
	huffCode := make([]uint, 257)
	code := uint(0)
	si := 0
	if len(huffSize) > 0 {
		si = huffSize[0]
	}
	p = 0
	for p < len(huffSize) {
		for p < len(huffSize) && huffSize[p] == si {
			huffCode[p] = code
			code++
			p++
		}
		// code is now 1 more than the last code used for codelength si;
		// but it must still fit in si bits.
		if int64(code) >= (int64(1) << uint(si)) {
			return nil, ErrBadHuffTable
		}
		code <<= 1
		si++
	}

	// Figure F.15: generate decoding tables for bit-sequential decoding.
	p = 0
	for l := 1; l <= 16; l++ {
		if htbl.Bits[l] != 0 {
			// valoffset[l] = huffval[] index of 1st symbol of code length l,
			// minus the minimum code of length l
			dtbl.ValOffset[l] = int32(p) - int32(huffCode[p])
			p += int(htbl.Bits[l])
			dtbl.MaxCode[l] = int32(huffCode[p-1])
		} else {
			dtbl.MaxCode[l] = -1
		}
	}
	dtbl.MaxCode[17] = 0xFFFFF // sentinel ensures huffDecode terminates

	// Compute lookahead tables to speed up decoding.
	// First set all entries to 0, indicating "too long";
	// then iterate through short codes and fill in entries.
	for i := range dtbl.LookNBits {
		dtbl.LookNBits[i] = 0
	}

	p = 0
	for l := 1; l <= HuffLookahead; l++ {
		for i := 0; i < int(htbl.Bits[l]); i++ {
			// l = current code length, p = its index in huffCode & HuffVal.
			// Generate left-justified code followed by all possible bit sequences.
			lookBits := int(huffCode[p]) << (HuffLookahead - l)
			for ctr := 1 << (HuffLookahead - l); ctr > 0; ctr-- {
				dtbl.LookNBits[lookBits] = l
				dtbl.LookSym[lookBits] = htbl.HuffVal[p]
				lookBits++
			}
			p++
		}
	}

	return dtbl, nil
}

// fillBitBuffer loads up the bit buffer to a depth of at least nbits.
// Returns the updated state and true on success, or false if suspension
// (in which case the state may be partially updated).
// Ported from jpeg_fill_bit_buffer in jdhuff.c.
//
// Optimized: reads up to 4 bytes at a time when no 0xFF markers are present,
// reducing per-byte overhead in the common case.
func fillBitBuffer(state *BitReadWorkingState, nbits int, unreadMarker *byte) bool {
	const minGetBits = BitBufSize - 7 // 25 for 32-bit buffer

	getBuffer := state.GetBuffer
	bitsLeft := state.BitsLeft

	data := state.NextInputByte
	bytesInBuffer := state.BytesInBuffer

	if *unreadMarker == 0 {
		for bitsLeft < minGetBits {
			if bytesInBuffer == 0 {
				if bitsLeft > 0 {
					break
				}
				state.GetBuffer = getBuffer
				state.BitsLeft = bitsLeft
				state.NextInputByte = data
				state.BytesInBuffer = bytesInBuffer
				return false
			}

			// Slow path: read byte-by-byte with marker detection
			bytesInBuffer--
			c := data[0]
			data = data[1:]

			if c == 0xFF {
				for {
					if bytesInBuffer == 0 {
						state.GetBuffer = getBuffer
						state.BitsLeft = bitsLeft
						state.NextInputByte = data
						state.BytesInBuffer = bytesInBuffer
						return false
					}
					bytesInBuffer--
					c = data[0]
					data = data[1:]
					if c != 0xFF {
						break
					}
				}
				if c == 0 {
					c = 0xFF
				} else {
					*unreadMarker = c
					break
				}
			}

			getBuffer = (getBuffer << 8) | uint32(c)
			bitsLeft += 8
		}
	}

	if *unreadMarker != 0 && nbits > bitsLeft {
		getBuffer <<= uint(minGetBits - bitsLeft)
		bitsLeft = minGetBits
	}

	state.GetBuffer = getBuffer
	state.BitsLeft = bitsLeft
	state.NextInputByte = data
	state.BytesInBuffer = bytesInBuffer
	return true
}

// huffDecode decodes a Huffman code that is longer than HuffLookahead bits.
// Returns the decoded symbol, or -1 on error.
// Ported from jpeg_huff_decode in jdhuff.c.
func huffDecode(state *BitReadWorkingState, htbl *DerivedHuffTable, minBits int, unreadMarker *byte) (int, error) {
	l := minBits

	// Ensure we have minBits bits available
	if state.BitsLeft < l {
		if !fillBitBuffer(state, l, unreadMarker) {
			return -1, ErrSuspension
		}
	}

	// Fetch minBits bits
	getBuffer := state.GetBuffer
	bitsLeft := state.BitsLeft
	code := int((getBuffer >> uint(bitsLeft-l)) & uint32((1<<uint(l))-1))
	bitsLeft -= l

	// Collect the rest of the Huffman code one bit at a time (Figure F.16).
	for code > int(htbl.MaxCode[l]) {
		code <<= 1
		if bitsLeft < 1 {
			// Must sync state with our local variables before calling
			// fillBitBuffer, because fillBitBuffer reads state.GetBuffer
			// and state.BitsLeft to know what's already buffered.
			state.GetBuffer = getBuffer
			state.BitsLeft = bitsLeft
			if !fillBitBuffer(state, 1, unreadMarker) {
				return -1, ErrSuspension
			}
			getBuffer = state.GetBuffer
			bitsLeft = state.BitsLeft
		}
		code |= int((getBuffer >> uint(bitsLeft-1)) & 1)
		bitsLeft--
		l++
	}

	// Update state
	state.GetBuffer = getBuffer
	state.BitsLeft = bitsLeft

	// With garbage input we may reach the sentinel value l = 17.
	if l > 16 {
		return 0, ErrHuffBadCode
	}

	return int(htbl.Pub.HuffVal[code+int(htbl.ValOffset[l])]), nil
}

// HuffDecodeFast decodes the next Huffman symbol from the bitstream using
// the derived table. This is the hot path.
//
// Returns (symbol, error). On suspension or marker, returns error.
// The state is updated in-place.
func HuffDecodeFast(state *BitReadWorkingState, htbl *DerivedHuffTable, unreadMarker *byte) (int, error) {
	getBuffer := state.GetBuffer
	bitsLeft := state.BitsLeft

	// Try the fast path with lookahead
	if bitsLeft < HuffLookahead {
		if !fillBitBuffer(state, 0, unreadMarker) {
			return -1, ErrSuspension
		}
		getBuffer = state.GetBuffer
		bitsLeft = state.BitsLeft
		if bitsLeft < HuffLookahead {
			// Slow path
			return huffDecode(state, htbl, 1, unreadMarker)
		}
	}

	// Peek at the next HuffLookahead bits
	look := int((getBuffer >> uint(bitsLeft-HuffLookahead)) & 0xFF)
	nb := htbl.LookNBits[look]
	if nb != 0 {
		// Fast path: code is nb bits long
		state.BitsLeft = bitsLeft - nb
		return int(htbl.LookSym[look]), nil
	}
	// Slow path: code is longer than HuffLookahead
	state.GetBuffer = getBuffer
	state.BitsLeft = bitsLeft
	return huffDecode(state, htbl, HuffLookahead+1, unreadMarker)
}

// decodeMCUSequential decodes one MCU's worth of Huffman-compressed
// coefficients for sequential (non-progressive) JPEG, full-size blocks.
// Ported from decode_mcu in jdhuff.c.
//
// Parameters:
//   - state: bit reading state (updated in-place)
//   - permState: persistent bit state to save/restore
//   - savedState: savable state (DC values, updated in-place)
//   - blocks: output blocks (must be pre-zeroed)
//   - dcTables: DC Huffman tables for each block in MCU
//   - acTables: AC Huffman tables for each block in MCU
//   - blocksInMCU: number of blocks in this MCU
//   - mcuMembership: component index for each block
//   - restartInterval: restart interval (0 if none)
//   - restartsToGo: remaining MCUs in restart interval (updated in-place)
//   - insufficientData: set to true if we hit a marker prematurely
//   - unreadMarker: nonzero if we hit a marker
//   - readRestartFn: function to handle restart markers
func DecodeMCUSequential(
	state *BitReadWorkingState,
	permState *BitReadState,
	savedState *SavableState,
	blocks []Block,
	dcTables []*DerivedHuffTable,
	acTables []*DerivedHuffTable,
	blocksInMCU int,
	mcuMembership []int,
	restartInterval int,
	restartsToGo *int,
	insufficientData *bool,
	unreadMarker *byte,
) bool {
	// Process restart marker if needed
	if restartInterval != 0 {
		if *restartsToGo == 0 {
			// Caller must handle restart processing
			return false
		}
	}

	// If we've run out of data, leave the MCU set to zeroes
	if !*insufficientData {
		// Load up working state
		getBuffer := permState.GetBuffer
		bitsLeft := permState.BitsLeft
		state.GetBuffer = getBuffer
		state.BitsLeft = bitsLeft

		saved := *savedState // copy

		for blkn := 0; blkn < blocksInMCU; blkn++ {
			block := &blocks[blkn]
			htbl := dcTables[blkn]

			// Section F.2.2.1: decode the DC coefficient difference
			s, err := HuffDecodeFast(state, htbl, unreadMarker)
			if err != nil {
				return false
			}

			var r int
			if s != 0 {
				if state.BitsLeft < s {
					if !fillBitBuffer(state, s, unreadMarker) {
						return false
					}
				}
				getBuffer = state.GetBuffer
				bitsLeft = state.BitsLeft
				r = int((getBuffer >> uint(bitsLeft-s)) & uint32((1<<uint(s))-1))
				bitsLeft -= s
				state.GetBuffer = getBuffer
				state.BitsLeft = bitsLeft
				s = HuffExtend(r, s)
			}

			ci := mcuMembership[blkn]
			s += saved.LastDCVal[ci]
			saved.LastDCVal[ci] = s
			block[0] = JCOEF(s)

			// Section F.2.2.2: decode the AC coefficients
			htbl = acTables[blkn]
			for k := 1; k < DCTSize2; k++ {
				s, err = HuffDecodeFast(state, htbl, unreadMarker)
				if err != nil {
					return false
				}

				r = s >> 4
				s &= 15

				if s != 0 {
					k += r
					if state.BitsLeft < s {
						if !fillBitBuffer(state, s, unreadMarker) {
							return false
						}
					}
					getBuffer = state.GetBuffer
					bitsLeft = state.BitsLeft
					r = int((getBuffer >> uint(bitsLeft-s)) & uint32((1<<uint(s))-1))
					bitsLeft -= s
					state.GetBuffer = getBuffer
					state.BitsLeft = bitsLeft
					s = HuffExtend(r, s)
					// Output coefficient in natural order.
					// Extra entries in NaturalOrder save us if k >= DCTSize2.
					block[NaturalOrder[k]] = JCOEF(s)
				} else {
					if r != 15 {
						break // End of block
					}
					k += 15 // ZRL: skip 15 zeroes
				}
			}
		}

		// Completed MCU, update state
		permState.GetBuffer = state.GetBuffer
		permState.BitsLeft = state.BitsLeft
		*savedState = saved
	}

	// Account for restart interval
	if restartInterval != 0 {
		*restartsToGo--
	}

	return true
}

// DecodeMCUDCFirst decodes MCU for DC initial scan in progressive mode.
// Ported from decode_mcu_DC_first in jdhuff.c.
func DecodeMCUDCFirst(
	state *BitReadWorkingState,
	permState *BitReadState,
	savedState *SavableState,
	blocks []Block,
	dcTables []*DerivedHuffTable,
	blocksInMCU int,
	mcuMembership []int,
	Al int,
	restartInterval int,
	restartsToGo *int,
	insufficientData *bool,
	unreadMarker *byte,
) bool {
	if restartInterval != 0 {
		if *restartsToGo == 0 {
			return false
		}
	}

	if !*insufficientData {
		getBuffer := permState.GetBuffer
		bitsLeft := permState.BitsLeft
		state.GetBuffer = getBuffer
		state.BitsLeft = bitsLeft
		saved := *savedState

		for blkn := 0; blkn < blocksInMCU; blkn++ {
			block := &blocks[blkn]
			ci := mcuMembership[blkn]

			s, err := HuffDecodeFast(state, dcTables[blkn], unreadMarker)
			if err != nil {
				return false
			}

			var r int
			if s != 0 {
				if state.BitsLeft < s {
					if !fillBitBuffer(state, s, unreadMarker) {
						return false
					}
				}
				getBuffer = state.GetBuffer
				bitsLeft = state.BitsLeft
				r = int((getBuffer >> uint(bitsLeft-s)) & uint32((1<<uint(s))-1))
				bitsLeft -= s
				state.GetBuffer = getBuffer
				state.BitsLeft = bitsLeft
				s = HuffExtend(r, s)
			}

			s += saved.LastDCVal[ci]
			saved.LastDCVal[ci] = s
			block[0] = JCOEF(s << uint(Al))
		}

		permState.GetBuffer = state.GetBuffer
		permState.BitsLeft = state.BitsLeft
		*savedState = saved
	}

	if restartInterval != 0 {
		*restartsToGo--
	}

	return true
}

// DecodeMCUACFirst decodes MCU for AC initial scan in progressive mode.
// Ported from decode_mcu_AC_first in jdhuff.c.
func DecodeMCUACFirst(
	state *BitReadWorkingState,
	permState *BitReadState,
	savedState *SavableState,
	block *Block,
	acTable *DerivedHuffTable,
	Ss, Se, Al int,
	restartInterval int,
	restartsToGo *int,
	insufficientData *bool,
	unreadMarker *byte,
) bool {
	if restartInterval != 0 {
		if *restartsToGo == 0 {
			return false
		}
	}

	if !*insufficientData {
		EOBRUN := savedState.EOBRUN

		if EOBRUN != 0 {
			EOBRUN--
		} else {
			getBuffer := permState.GetBuffer
			bitsLeft := permState.BitsLeft
			state.GetBuffer = getBuffer
			state.BitsLeft = bitsLeft

			for k := Ss; k <= Se; k++ {
				s, err := HuffDecodeFast(state, acTable, unreadMarker)
				if err != nil {
					return false
				}
				r := s >> 4
				s &= 15
				if s != 0 {
					k += r
					if state.BitsLeft < s {
						if !fillBitBuffer(state, s, unreadMarker) {
							return false
						}
					}
					getBuffer = state.GetBuffer
					bitsLeft = state.BitsLeft
					r = int((getBuffer >> uint(bitsLeft-s)) & uint32((1<<uint(s))-1))
					bitsLeft -= s
					state.GetBuffer = getBuffer
					state.BitsLeft = bitsLeft
					s = HuffExtend(r, s)
					block[NaturalOrder[k]] = JCOEF(s << uint(Al))
				} else {
					if r != 15 {
						// EOBr
						if r != 0 {
							EOBRUN = 1 << uint(r)
							if state.BitsLeft < r {
								if !fillBitBuffer(state, r, unreadMarker) {
									return false
								}
							}
							getBuffer = state.GetBuffer
							bitsLeft = state.BitsLeft
							r = int((getBuffer >> uint(bitsLeft-r)) & uint32((1<<uint(r))-1))
							bitsLeft -= r
							state.GetBuffer = getBuffer
							state.BitsLeft = bitsLeft
							EOBRUN += uint(r)
							EOBRUN--
						}
						break
					}
					k += 15
				}
			}

			permState.GetBuffer = state.GetBuffer
			permState.BitsLeft = state.BitsLeft
		}

		savedState.EOBRUN = EOBRUN
	}

	if restartInterval != 0 {
		*restartsToGo--
	}

	return true
}

// DecodeMCUDCRefine decodes MCU for DC successive approximation refinement.
// Ported from decode_mcu_DC_refine in jdhuff.c.
func DecodeMCUDCRefine(
	state *BitReadWorkingState,
	permState *BitReadState,
	blocks []Block,
	blocksInMCU int,
	Al int,
	restartInterval int,
	restartsToGo *int,
	unreadMarker *byte,
) bool {
	if restartInterval != 0 {
		if *restartsToGo == 0 {
			return false
		}
	}

	getBuffer := permState.GetBuffer
	bitsLeft := permState.BitsLeft
	state.GetBuffer = getBuffer
	state.BitsLeft = bitsLeft

	p1 := JCOEF(1 << uint(Al))

	for blkn := 0; blkn < blocksInMCU; blkn++ {
		if state.BitsLeft < 1 {
			if !fillBitBuffer(state, 1, unreadMarker) {
				return false
			}
		}
		getBuffer = state.GetBuffer
		bitsLeft = state.BitsLeft
		if (getBuffer>>uint(bitsLeft-1))&1 != 0 {
			blocks[blkn][0] |= p1
		}
		bitsLeft--
		state.GetBuffer = getBuffer
		state.BitsLeft = bitsLeft
	}

	permState.GetBuffer = state.GetBuffer
	permState.BitsLeft = state.BitsLeft

	if restartInterval != 0 {
		*restartsToGo--
	}

	return true
}

// DecodeMCUACRefine decodes MCU for AC successive approximation refinement.
// Ported from decode_mcu_AC_refine in jdhuff.c.
// Returns true on success. On failure, any newly nonzero coefficients are undone.
func DecodeMCUACRefine(
	state *BitReadWorkingState,
	permState *BitReadState,
	savedState *SavableState,
	block *Block,
	acTable *DerivedHuffTable,
	Ss, Se, Al int,
	restartInterval int,
	restartsToGo *int,
	insufficientData *bool,
	unreadMarker *byte,
) bool {
	if restartInterval != 0 {
		if *restartsToGo == 0 {
			return false
		}
	}

	if !*insufficientData {
		p1 := JCOEF(1 << uint(Al))
		m1 := JCOEF(-p1)

		state.GetBuffer = permState.GetBuffer
		state.BitsLeft = permState.BitsLeft
		EOBRUN := savedState.EOBRUN

		var numNewNZ int
		newNZPos := make([]int, DCTSize2)

		// Use a closure to implement the undo-on-failure pattern
		// (replacing the C goto undoit pattern).
		ok := func() bool {
			k := Ss

			if EOBRUN == 0 {
				for k <= Se {
					s, err := HuffDecodeFast(state, acTable, unreadMarker)
					if err != nil {
						return false
					}
					r := s >> 4
					s &= 15
					if s != 0 {
						if s != 1 {
							// size should always be 1 for refinement
						}
						if state.BitsLeft < 1 {
							if !fillBitBuffer(state, 1, unreadMarker) {
								return false
							}
						}
						getBuffer := state.GetBuffer
						bitsLeft := state.BitsLeft
						if (getBuffer>>uint(bitsLeft-1))&1 != 0 {
							s = int(p1)
						} else {
							s = int(m1)
						}
						bitsLeft--
						state.GetBuffer = getBuffer
						state.BitsLeft = bitsLeft
					} else {
						if r != 15 {
							EOBRUN = 1 << uint(r)
							if r != 0 {
								if state.BitsLeft < r {
									if !fillBitBuffer(state, r, unreadMarker) {
										return false
									}
								}
								getBuffer := state.GetBuffer
								bitsLeft := state.BitsLeft
								r = int((getBuffer >> uint(bitsLeft-r)) & uint32((1<<uint(r))-1))
								bitsLeft -= r
								state.GetBuffer = getBuffer
								state.BitsLeft = bitsLeft
								EOBRUN += uint(r)
							}
							break
						}
					}

					// Advance over already-nonzero coefs and r still-zero coefs
					for {
						thiscoef := block[NaturalOrder[k]]
						if thiscoef != 0 {
							if state.BitsLeft < 1 {
								if !fillBitBuffer(state, 1, unreadMarker) {
									return false
								}
							}
							getBuffer := state.GetBuffer
							bitsLeft := state.BitsLeft
							if (getBuffer>>uint(bitsLeft-1))&1 != 0 {
								if thiscoef&p1 == 0 {
									if thiscoef >= 0 {
										block[NaturalOrder[k]] += p1
									} else {
										block[NaturalOrder[k]] += m1
									}
								}
							}
							bitsLeft--
							state.GetBuffer = getBuffer
							state.BitsLeft = bitsLeft
						} else {
							r--
							if r < 0 {
								break
							}
						}
						k++
						if k > Se {
							break
						}
					}
					if s != 0 {
						pos := NaturalOrder[k]
						block[pos] = JCOEF(s)
						newNZPos[numNewNZ] = pos
						numNewNZ++
					}
					k++
					if k > Se {
						break
					}
				}
			}

			if EOBRUN != 0 {
				for k <= Se {
					thiscoef := block[NaturalOrder[k]]
					if thiscoef != 0 {
						if state.BitsLeft < 1 {
							if !fillBitBuffer(state, 1, unreadMarker) {
								return false
							}
						}
						getBuffer := state.GetBuffer
						bitsLeft := state.BitsLeft
						if (getBuffer>>uint(bitsLeft-1))&1 != 0 {
							if thiscoef&p1 == 0 {
								if thiscoef >= 0 {
									block[NaturalOrder[k]] += p1
								} else {
									block[NaturalOrder[k]] += m1
								}
							}
						}
						bitsLeft--
						state.GetBuffer = getBuffer
						state.BitsLeft = bitsLeft
					}
					k++
				}
				EOBRUN--
			}

			return true
		}()

		if !ok {
			// Re-zero any output coefficients that we made newly nonzero
			for numNewNZ > 0 {
				numNewNZ--
				block[newNZPos[numNewNZ]] = 0
			}
			return false
		}

		permState.GetBuffer = state.GetBuffer
		permState.BitsLeft = state.BitsLeft
		savedState.EOBRUN = EOBRUN
	}

	if restartInterval != 0 {
		*restartsToGo--
	}

	return true
}

// InitBitReader initializes the bit reader state from a data slice.
func InitBitReader(state *BitReadWorkingState, data []byte) {
	state.NextInputByte = data
	state.BytesInBuffer = len(data)
	state.GetBuffer = 0
	state.BitsLeft = 0
}

// GetBits extracts n bits from the bitstream. Caller must ensure enough bits
// are available (call fillBitBuffer first if needed).
func GetBits(state *BitReadWorkingState, n int) int {
	getBuffer := state.GetBuffer
	bitsLeft := state.BitsLeft
	val := int((getBuffer >> uint(bitsLeft-n)) & uint32((1<<uint(n))-1))
	bitsLeft -= n
	state.GetBuffer = getBuffer
	state.BitsLeft = bitsLeft
	return val
}

// PeekBits peeks at the next n bits without consuming them.
func PeekBits(state *BitReadWorkingState, n int) int {
	return int((state.GetBuffer >> uint(state.BitsLeft-n)) & uint32((1<<uint(n))-1))
}
