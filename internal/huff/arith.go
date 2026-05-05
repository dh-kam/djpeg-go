package huff

import "errors"

// Arithmetic decoding errors.
var (
	ErrArithBadCode    = errors.New("jpeg: bad arithmetic code")
	ErrArithCantSuspend = errors.New("jpeg: arithmetic decoder cannot suspend")
	ErrNoArithTable    = errors.New("jpeg: no arithmetic table")
)

// DC and AC statistics bin counts.
const (
	DCStatBins = 64
	ACStatBins = 256
)

// ArithDecoder provides arithmetic entropy decoding for JPEG.
// Ported from jdarith.c + jaricom.c.
type ArithDecoder struct {
	State ArithEntropyState
}

// NewArithDecoder creates a new arithmetic decoder.
func NewArithDecoder() *ArithDecoder {
	d := &ArithDecoder{}
	// Initialize index for fixed probability estimation
	d.State.FixedBin[0] = 113
	// Mark tables unallocated
	for i := 0; i < NumArithTbls; i++ {
		d.State.DCStats[i] = nil
		d.State.ACStats[i] = nil
	}
	return d
}

// InitPass initializes the arithmetic decoder for a scan.
func (d *ArithDecoder) InitPass(progressiveMode bool, compsInScan int,
	compInfos []ComponentInfo, limSe int,
	Ss, Se, Ah, Al int) {

	entropy := &d.State

	// Allocate and initialize statistics areas for each component
	for ci := 0; ci < compsInScan; ci++ {
		compptr := compInfos[ci]

		if !progressiveMode || (Ss == 0 && Ah == 0) {
			tbl := compptr.DCTblNo
			if entropy.DCStats[tbl] == nil {
				entropy.DCStats[tbl] = make([]byte, DCStatBins)
			} else {
				for i := range entropy.DCStats[tbl] {
					entropy.DCStats[tbl][i] = 0
				}
			}
			entropy.LastDCVal[ci] = 0
			entropy.DCContext[ci] = 0
		}
		if (!progressiveMode && limSe != 0) || (progressiveMode && Ss != 0) {
			tbl := compptr.ACTblNo
			if entropy.ACStats[tbl] == nil {
				entropy.ACStats[tbl] = make([]byte, ACStatBins)
			} else {
				for i := range entropy.ACStats[tbl] {
					entropy.ACStats[tbl][i] = 0
				}
			}
		}
	}

	// Initialize arithmetic decoding variables
	entropy.C = 0
	entropy.A = 0
	entropy.Ct = -16 // force reading 2 initial bytes to fill C
}

// arithDecode performs the core arithmetic decoding routine.
// Returns 0 or 1 (binary decision).
// Ported from arith_decode in jdarith.c.
func (d *ArithDecoder) arithDecode(st []byte, src func() byte, unreadMarker *byte) int {
	e := &d.State

	// Renormalization & data input per section D.2.6
	for e.A < 0x8000 {
		e.Ct--
		if e.Ct < 0 {
			// Need to fetch next data byte
			var data byte
			if *unreadMarker != 0 {
				data = 0 // stuff zero data
			} else {
				data = src()
				if data == 0xFF {
					for {
						data = src()
						if data != 0xFF {
							break
						}
					}
					if data == 0 {
						data = 0xFF
					} else {
						*unreadMarker = data
						data = 0
					}
				}
			}
			e.C = (e.C << 8) | int32(data)
			e.Ct += 8
			if e.Ct < 0 {
				// Need more initial bytes
				e.Ct++
				if e.Ct == 0 {
					// Got 2 initial bytes -> re-init A and exit loop
					e.A = 0x8000
				}
			}
		}
		e.A <<= 1
	}

	// Fetch values from compact representation of Table D.3
	sv := int(st[0])
	qe := arithTab[sv&0x7F]
	nl := byte(qe & 0xFF)
	qe >>= 8
	nm := byte(qe & 0xFF)
	qe >>= 8

	// Decode & estimation procedures per sections D.2.4 & D.2.5
	temp := e.A - qe
	e.A = temp
	temp <<= uint(e.Ct)
	if e.C >= temp {
		e.C -= temp
		// Conditional LPS exchange
		if e.A < qe {
			e.A = qe
			st[0] = byte(sv&0x80) ^ nm
		} else {
			e.A = qe
			st[0] = byte(sv&0x80) ^ nl
			sv ^= 0x80
		}
	} else if e.A < 0x8000 {
		// Conditional MPS exchange
		if e.A < qe {
			st[0] = byte(sv&0x80) ^ nl
			sv ^= 0x80
		} else {
			st[0] = byte(sv&0x80) ^ nm
		}
	}

	return sv >> 7
}

// DecodeMCUSequential decodes one MCU's worth of arithmetic-compressed
// coefficients for sequential JPEG.
// Ported from decode_mcu in jdarith.c.
//
// The src function is called to get the next byte from the data stream.
func (d *ArithDecoder) DecodeMCUSequential(
	blocks []Block,
	blocksInMCU int,
	mcuMembership []int,
	compInfos []ComponentInfo,
	Ss, Se, limSe int,
	arithDCL, arithDCU [NumArithTbls]uint8,
	arithACK [NumArithTbls]int,
	src func() byte,
	unreadMarker *byte,
) bool {
	entropy := &d.State

	if entropy.Ct == -1 {
		return true // if error, do nothing
	}

	for blkn := 0; blkn < blocksInMCU; blkn++ {
		block := &blocks[blkn]
		ci := mcuMembership[blkn]
		compptr := compInfos[ci]

		// DC coefficient decoding
		tbl := compptr.DCTblNo
		st := entropy.DCStats[tbl][entropy.DCContext[ci]:]

		if d.arithDecode(st, src, unreadMarker) == 0 {
			entropy.DCContext[ci] = 0
		} else {
			sign := d.arithDecode(st[1:], src, unreadMarker)
			st = st[2:]
			if sign != 0 {
				st = st[1:]
			} else {
				st = st[:1]
			}

			m := 0
			if d.arithDecode(st, src, unreadMarker) != 0 {
				st = entropy.DCStats[tbl][20:]
				for d.arithDecode(st, src, unreadMarker) != 0 {
					m <<= 1
					if m == 0x8000 {
						entropy.Ct = -1
						return true
					}
					st = st[1:]
				}
			}

			dcL := int(arithDCL[tbl])
			dcU := int(arithDCU[tbl])
			if m < (1<<uint(dcL))>>1 {
				entropy.DCContext[ci] = 0
			} else if m > (1<<uint(dcU))>>1 {
				entropy.DCContext[ci] = 12 + (sign * 4)
			} else {
				entropy.DCContext[ci] = 4 + (sign * 4)
			}

			v := m
			st = st[14:]
			for m >>= 1; m != 0; m >>= 1 {
				if d.arithDecode(st, src, unreadMarker) != 0 {
					v |= m
				}
			}
			v++
			if sign != 0 {
				v = -v
			}
			entropy.LastDCVal[ci] += v
		}

		block[0] = JCOEF(entropy.LastDCVal[ci])

		// AC coefficient decoding
		if limSe == 0 {
			continue
		}
		tbl = compptr.ACTblNo
		k := 0

		for k < limSe {
			st := entropy.ACStats[tbl][3*k:]
			if d.arithDecode(st, src, unreadMarker) != 0 {
				break // EOB
			}
			for {
				k++
				if d.arithDecode(st[1:], src, unreadMarker) != 0 {
					break
				}
				st = st[3:]
				if k >= limSe {
					entropy.Ct = -1
					return true
				}
			}

			sign := d.arithDecode(entropy.FixedBin[:], src, unreadMarker)
			st = st[2:]

			m := 0
			if d.arithDecode(st, src, unreadMarker) != 0 {
				if d.arithDecode(st, src, unreadMarker) != 0 {
					m <<= 1
					acK := arithACK[tbl]
					if k <= acK {
						st = entropy.ACStats[tbl][189:]
					} else {
						st = entropy.ACStats[tbl][217:]
					}
					for d.arithDecode(st, src, unreadMarker) != 0 {
						m <<= 1
						if m == 0x8000 {
							entropy.Ct = -1
							return true
						}
						st = st[1:]
					}
				}
			}

			v := m
			st = st[14:]
			for m >>= 1; m != 0; m >>= 1 {
				if d.arithDecode(st, src, unreadMarker) != 0 {
					v |= m
				}
			}
			v++
			if sign != 0 {
				v = -v
			}
			block[NaturalOrder[k]] = JCOEF(v)
			k++
		}
	}

	return true
}

// ResetStats resets all statistics areas after a restart marker.
func (d *ArithDecoder) ResetStats(compsInScan int, compInfos []ComponentInfo,
	progressiveMode bool, Ss, Ah, limSe int) {
	entropy := &d.State
	for ci := 0; ci < compsInScan; ci++ {
		compptr := compInfos[ci]
		if !progressiveMode || (Ss == 0 && Ah == 0) {
			for i := range entropy.DCStats[compptr.DCTblNo] {
				entropy.DCStats[compptr.DCTblNo][i] = 0
			}
			entropy.LastDCVal[ci] = 0
			entropy.DCContext[ci] = 0
		}
		if (!progressiveMode && limSe != 0) || (progressiveMode && Ss != 0) {
			for i := range entropy.ACStats[compptr.ACTblNo] {
				entropy.ACStats[compptr.ACTblNo][i] = 0
			}
		}
	}
	entropy.C = 0
	entropy.A = 0
	entropy.Ct = -16
}
