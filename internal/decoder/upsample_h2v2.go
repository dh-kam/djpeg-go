package decoder

// h2v2FancyUpsample performs fancy (triangle filter) upsampling for the 4:2:0 case.
// This is the optimized two-pass approach:
//   Pass 1: horizontal fancy upsampling into temp buffers for 3 context rows
//   Pass 2: vertical fancy interpolation + color conversion
//
// The horizontal fancy upsampling produces 2 output pixels per chroma sample:
//   even col: (3*src[cCol] + src[cCol-1] + 2) >> 2  (interpolate toward left neighbor)
//   odd col:  (3*src[cCol] + src[cCol+1] + 2) >> 2  (interpolate toward right neighbor)
// Boundary samples use nearest-neighbor replication.

// hFancyUpsampleRow performs horizontal fancy upsampling on one chroma row.
// src has chromaWidth entries; dst gets outputWidth entries.
// Each src entry produces two dst entries (even and odd output columns).
func hFancyUpsampleRow(src []byte, dst []int16, outputWidth int) {
	srcLen := len(src)
	if srcLen == 0 {
		return
	}
	lastC := srcLen - 1
	s0 := int16(src[0])

	// Left boundary pair: cCol=0
	// col 0 (even): replicate src[0]
	// col 1 (odd): interpolate (3*src[0] + src[1] + 2) >> 2
	dst[0] = s0
	if outputWidth > 1 {
		if lastC > 0 {
			dst[1] = (3*s0 + int16(src[1]) + 2) >> 2
		} else {
			dst[1] = s0
		}
	}

	// Interior pairs: cCol = 1 .. lastC-1
	// Process two output pixels per iteration:
	//   even: (3*cur + prev + 2) >> 2
	//   odd:  (3*cur + next + 2) >> 2
	cCol := 1
	for cCol < lastC {
		base := cCol << 1
		if base >= outputWidth {
			break
		}
		prev := int16(src[cCol-1])
		cur := int16(src[cCol])
		next := int16(src[cCol+1])
		dst[base] = (3*cur + prev + 2) >> 2
		if base+1 < outputWidth {
			dst[base+1] = (3*cur + next + 2) >> 2
		}
		cCol++
	}

	// Right boundary pair: cCol=lastC
	// col base (even): interpolate (3*cur + prev + 2) >> 2
	// col base+1 (odd): replicate src[lastC]
	if lastC > 0 {
		base := lastC << 1
		if base < outputWidth {
			cur := int16(src[lastC])
			prev := int16(src[lastC-1])
			dst[base] = (3*cur + prev + 2) >> 2
			if base+1 < outputWidth {
				dst[base+1] = cur
			}
		}
	}
}
