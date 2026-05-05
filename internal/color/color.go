package color

// This file ports jdcolor.c from IJG libjpeg 9f to pure Go.
// It implements color space conversion for JPEG decompression:
//   - YCbCr -> RGB (most common)
//   - BG_YCC -> RGB (wide gamut)
//   - YCbCr -> Grayscale (just copy Y)
//   - Grayscale -> RGB (duplicate gray)
//   - RGB -> Grayscale
//   - RGB pass-through (separate planes to interleaved)
//   - YCCK -> CMYK
//   - CMYK -> YK (colorless output)
//   - Null conversion (just interleave)
//   - Subtract-green inverse transforms

// ColorConverter holds the state for color space conversion,
// including precomputed lookup tables for YCbCr->RGB conversion.
type ColorConverter struct {
	// Conversion function pointer (selected at init time)
	// Signature: func(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int)
	// inputBuf is indexed [component][row][col]
	ColorConvert func(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRow int)

	// Lookup tables for YCbCr -> RGB conversion (int32 for original buildYccRGBTable compat)
	CrRTab []int32 // Cr -> R contribution (already divided)
	CbBTab []int32 // Cb -> B contribution (already divided)
	CrGTab []int32 // Cr -> G contribution (still scaled)
	CbGTab []int32 // Cb -> G contribution (still scaled, includes ONE_HALF)

	// Native int typed tables for hot-path color conversion (avoids int32->int conversion)
	CrRTabInt []int // Cr -> R contribution
	CbBTabInt []int // Cb -> B contribution
	CrGTabInt []int // Cr -> G contribution (still scaled by 2^16)
	CbGTabInt []int // Cb -> G contribution (still scaled by 2^16)

	// Lookup tables for RGB -> Grayscale conversion
	RYTab []int32
	GYTab []int32
	BYTab []int32
}

// NewColorConverter creates and initializes a ColorConverter for the given
// decompression parameters.
func NewColorConverter(info *DecompressInfo) *ColorConverter {
	cc := &ColorConverter{}

	switch info.OutColorSpace {
	case JCS_GRAYSCALE:
		cc.initGrayscaleOutput(info)
	case JCS_RGB:
		cc.initRGBOutput(info)
	case JCS_BG_RGB:
		cc.initBGRGBOutput(info)
	case JCS_CMYK:
		cc.initCMYKOutput(info)
	case JCS_YCCK:
		cc.initYCCKOutput(info)
	default:
		cc.initNullConversion(info)
	}

	return cc
}

// initGrayscaleOutput sets up conversion to grayscale output.
func (cc *ColorConverter) initGrayscaleOutput(info *DecompressInfo) {
	switch info.JpegColorSpace {
	case JCS_GRAYSCALE, JCS_YCbCr, JCS_BG_YCC:
		cc.ColorConvert = cc.grayscaleConvert
		// For YCbCr->grayscale, mark chroma components as not needed
		for i := 1; i < info.NumComponents; i++ {
			info.CompInfo[i].ComponentNeeded = false
		}
	case JCS_RGB:
		cc.buildRGBYTable()
		switch info.ColorTransform {
		case JCT_NONE:
			cc.ColorConvert = cc.rgbGrayConvert
		case JCT_SUBTRACT_GREEN:
			cc.ColorConvert = cc.rgb1GrayConvert
		}
	}
}

// initRGBOutput sets up conversion to RGB output.
func (cc *ColorConverter) initRGBOutput(info *DecompressInfo) {
	switch info.JpegColorSpace {
	case JCS_GRAYSCALE:
		cc.ColorConvert = cc.grayRGBConvert
	case JCS_YCbCr:
		cc.buildYccRGBTable()
		cc.ColorConvert = cc.yccRGBConvert
	case JCS_BG_YCC:
		cc.buildBgyccRGBTable()
		cc.ColorConvert = cc.yccRGBConvert
	case JCS_RGB:
		switch info.ColorTransform {
		case JCT_NONE:
			cc.ColorConvert = cc.rgbConvert
		case JCT_SUBTRACT_GREEN:
			cc.ColorConvert = cc.rgb1RGBConvert
		}
	}
}

// initBGRGBOutput sets up conversion for BG_RGB output.
func (cc *ColorConverter) initBGRGBOutput(info *DecompressInfo) {
	switch info.ColorTransform {
	case JCT_NONE:
		cc.ColorConvert = cc.rgbConvert
	case JCT_SUBTRACT_GREEN:
		cc.ColorConvert = cc.rgb1RGBConvert
	}
}

// initCMYKOutput sets up conversion for CMYK output (from YCCK).
func (cc *ColorConverter) initCMYKOutput(info *DecompressInfo) {
	if info.JpegColorSpace == JCS_YCCK {
		cc.buildYccRGBTable()
		cc.ColorConvert = cc.ycckCMYKConvert
	} else {
		cc.initNullConversion(info)
	}
}

// initYCCKOutput sets up conversion for YCCK output (from CMYK).
func (cc *ColorConverter) initYCCKOutput(info *DecompressInfo) {
	if info.JpegColorSpace == JCS_CMYK {
		cc.buildRGBYTable()
		cc.ColorConvert = cc.cmykYKConvert
	} else {
		cc.initNullConversion(info)
	}
}

// initNullConversion sets up a pass-through that just interleaves.
func (cc *ColorConverter) initNullConversion(info *DecompressInfo) {
	cc.ColorConvert = cc.nullConvert
}

// buildYccRGBTable builds lookup tables for standard YCbCr -> RGB conversion.
// Uses fixed-point arithmetic with SCALEBITS=16.
//   R = Y + 1.402 * (Cr - 128)
//   G = Y - 0.344136286 * (Cb - 128) - 0.714136286 * (Cr - 128)
//   B = Y + 1.772 * (Cb - 128)
func (cc *ColorConverter) buildYccRGBTable() {
	cc.CrRTab = make([]int32, MaxJSample+1)
	cc.CbBTab = make([]int32, MaxJSample+1)
	cc.CrGTab = make([]int32, MaxJSample+1)
	cc.CbGTab = make([]int32, MaxJSample+1)

	// Native int typed tables (avoid int32->int conversion in hot path)
	cc.CrRTabInt = make([]int, MaxJSample+1)
	cc.CbBTabInt = make([]int, MaxJSample+1)
	cc.CrGTabInt = make([]int, MaxJSample+1)
	cc.CbGTabInt = make([]int, MaxJSample+1)

	for i := 0; i <= MaxJSample; i++ {
		x := int32(i - CenterJSample)
		// Cr=>R value: nearest int to 1.402 * x
		cc.CrRTab[i] = (fix(1.402) * x + OneHalf) >> ScaleBits
		// Cb=>B value: nearest int to 1.772 * x
		cc.CbBTab[i] = (fix(1.772) * x + OneHalf) >> ScaleBits
		// Cr=>G value: scaled-up -0.714136286 * x (NOT yet shifted)
		cc.CrGTab[i] = (-fix(0.714136286)) * x
		// Cb=>G value: scaled-up -0.344136286 * x + ONE_HALF
		cc.CbGTab[i] = (-fix(0.344136286))*x + OneHalf

		// Populate int-typed tables
		cc.CrRTabInt[i] = int(cc.CrRTab[i])
		cc.CbBTabInt[i] = int(cc.CbBTab[i])
		cc.CrGTabInt[i] = int(cc.CrGTab[i])
		cc.CbGTabInt[i] = int(cc.CbGTab[i])
	}
}

// buildBgyccRGBTable builds lookup tables for BG_YCC -> RGB conversion (wide gamut).
// Uses 2x coefficients: K=4 for bg-sYCC.
func (cc *ColorConverter) buildBgyccRGBTable() {
	cc.CrRTab = make([]int32, MaxJSample+1)
	cc.CbBTab = make([]int32, MaxJSample+1)
	cc.CrGTab = make([]int32, MaxJSample+1)
	cc.CbGTab = make([]int32, MaxJSample+1)

	for i := 0; i <= MaxJSample; i++ {
		x := int32(i - CenterJSample)
		// Cr=>R value: nearest int to 2.804 * x
		cc.CrRTab[i] = (fix(2.804)*x + OneHalf) >> ScaleBits
		// Cb=>B value: nearest int to 3.544 * x
		cc.CbBTab[i] = (fix(3.544)*x + OneHalf) >> ScaleBits
		// Cr=>G value: scaled-up -1.428272572 * x
		cc.CrGTab[i] = (-fix(1.428272572)) * x
		// Cb=>G value: scaled-up -0.688272572 * x + ONE_HALF
		cc.CbGTab[i] = (-fix(0.688272572))*x + OneHalf
	}
}

// buildRGBYTable builds lookup tables for RGB -> Grayscale conversion.
// Y = 0.299 * R + 0.587 * G + 0.114 * B
func (cc *ColorConverter) buildRGBYTable() {
	cc.RYTab = make([]int32, MaxJSample+1)
	cc.GYTab = make([]int32, MaxJSample+1)
	cc.BYTab = make([]int32, MaxJSample+1)

	for i := int32(0); i <= MaxJSample; i++ {
		cc.RYTab[i] = fix(0.299) * i
		cc.GYTab[i] = fix(0.587) * i
		cc.BYTab[i] = fix(0.114)*i + OneHalf
	}
}

// ---- Conversion functions ----
// All conversion functions have the signature:
//   func(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int)
// inputBuf is indexed [component][row][col]

// yccRGBConvert converts YCbCr to RGB using precomputed lookup tables.
// This is the most common conversion path.
func (cc *ColorConverter) yccRGBConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0]) / RGBPixelSize
	rl := cc.rangeLimitSlice()

	for row := 0; row < numRows; row++ {
		inY := inputBuf[0][inputRow]
		inCb := inputBuf[1][inputRow]
		inCr := inputBuf[2][inputRow]
		inputRow++
		out := outputBuf[row]

		for col := 0; col < numCols; col++ {
			y := int(inY[col])
			cb := int(inCb[col])
			cr := int(inCr[col])

			// R = Y + Cr_r_tab[Cr]
			r := y + int(cc.CrRTab[cr])
			// G = Y + ((Cb_g_tab[Cb] + Cr_g_tab[Cr]) >> SCALEBITS)
			g := y + int((cc.CbGTab[cb]+cc.CrGTab[cr])>>ScaleBits)
			// B = Y + Cb_b_tab[Cb]
			b := y + int(cc.CbBTab[cb])

			idx := col * RGBPixelSize
			out[idx+RGBRed] = rl[r]
			out[idx+RGBGreen] = rl[g]
			out[idx+RGBBlue] = rl[b]
		}
	}
}

// grayscaleConvert copies the Y component directly (for YCbCr or grayscale input).
func (cc *ColorConverter) grayscaleConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0])
	for row := 0; row < numRows; row++ {
		copy(outputBuf[row][:numCols], inputBuf[0][inputRow][:numCols])
		inputRow++
	}
}

// grayRGBConvert converts grayscale to RGB by duplicating the gray value.
func (cc *ColorConverter) grayRGBConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0]) / RGBPixelSize
	for row := 0; row < numRows; row++ {
		in := inputBuf[0][inputRow]
		inputRow++
		out := outputBuf[row]
		for col := 0; col < numCols; col++ {
			v := in[col]
			idx := col * RGBPixelSize
			out[idx+RGBRed] = v
			out[idx+RGBGreen] = v
			out[idx+RGBBlue] = v
		}
	}
}

// rgbConvert copies RGB planes to interleaved RGB (no color transform).
func (cc *ColorConverter) rgbConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0]) / RGBPixelSize
	for row := 0; row < numRows; row++ {
		inR := inputBuf[0][inputRow]
		inG := inputBuf[1][inputRow]
		inB := inputBuf[2][inputRow]
		inputRow++
		out := outputBuf[row]
		for col := 0; col < numCols; col++ {
			idx := col * RGBPixelSize
			out[idx+RGBRed] = inR[col]
			out[idx+RGBGreen] = inG[col]
			out[idx+RGBBlue] = inB[col]
		}
	}
}

// rgb1RGBConvert handles [R-G, G, B-G] -> RGB with modulo inverse color transform.
func (cc *ColorConverter) rgb1RGBConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0]) / RGBPixelSize
	for row := 0; row < numRows; row++ {
		inR := inputBuf[0][inputRow]
		inG := inputBuf[1][inputRow]
		inB := inputBuf[2][inputRow]
		inputRow++
		out := outputBuf[row]
		for col := 0; col < numCols; col++ {
			r := int(inR[col])
			g := int(inG[col])
			b := int(inB[col])
			idx := col * RGBPixelSize
			out[idx+RGBRed] = byte((r + g - CenterJSample) & MaxJSample)
			out[idx+RGBGreen] = byte(g)
			out[idx+RGBBlue] = byte((b + g - CenterJSample) & MaxJSample)
		}
	}
}

// rgbGrayConvert converts RGB to grayscale using lookup tables.
func (cc *ColorConverter) rgbGrayConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0])
	for row := 0; row < numRows; row++ {
		inR := inputBuf[0][inputRow]
		inG := inputBuf[1][inputRow]
		inB := inputBuf[2][inputRow]
		inputRow++
		out := outputBuf[row]
		for col := 0; col < numCols; col++ {
			y := cc.RYTab[inR[col]] + cc.GYTab[inG[col]] + cc.BYTab[inB[col]]
			out[col] = byte(y >> ScaleBits)
		}
	}
}

// rgb1GrayConvert converts [R-G, G, B-G] to grayscale.
func (cc *ColorConverter) rgb1GrayConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0])
	for row := 0; row < numRows; row++ {
		inR := inputBuf[0][inputRow]
		inG := inputBuf[1][inputRow]
		inB := inputBuf[2][inputRow]
		inputRow++
		out := outputBuf[row]
		for col := 0; col < numCols; col++ {
			r := int(inR[col])
			g := int(inG[col])
			b := int(inB[col])
			y := cc.RYTab[(r+g-CenterJSample)&MaxJSample] +
				cc.GYTab[g] +
				cc.BYTab[(b+g-CenterJSample)&MaxJSample]
			out[col] = byte(y >> ScaleBits)
		}
	}
}

// ycckCMYKConvert converts YCCK to CMYK.
// YCbCr -> RGB with inversion: C=1-R, M=1-G, Y=1-B, K passes through.
func (cc *ColorConverter) ycckCMYKConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0]) / 4
	rl := cc.rangeLimitSlice()

	for row := 0; row < numRows; row++ {
		inY := inputBuf[0][inputRow]
		inCb := inputBuf[1][inputRow]
		inCr := inputBuf[2][inputRow]
		inK := inputBuf[3][inputRow]
		inputRow++
		out := outputBuf[row]

		for col := 0; col < numCols; col++ {
			y := int(inY[col])
			cb := int(inCb[col])
			cr := int(inCr[col])

			r := y + int(cc.CrRTab[cr])
			g := y + int((cc.CbGTab[cb]+cc.CrGTab[cr])>>ScaleBits)
			b := y + int(cc.CbBTab[cb])

			idx := col * 4
			out[idx+0] = rl[MaxJSample-r] // C = 1-R
			out[idx+1] = rl[MaxJSample-g] // M = 1-G
			out[idx+2] = rl[MaxJSample-b] // Y = 1-B
			out[idx+3] = inK[col]          // K passes through
		}
	}
}

// cmykYKConvert converts CMYK to YK for colorless output.
func (cc *ColorConverter) cmykYKConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	numCols := len(outputBuf[0]) / 2
	for row := 0; row < numRows; row++ {
		inC := inputBuf[0][inputRow]
		inM := inputBuf[1][inputRow]
		inY := inputBuf[2][inputRow]
		inK := inputBuf[3][inputRow]
		inputRow++
		out := outputBuf[row]

		for col := 0; col < numCols; col++ {
			y := cc.RYTab[MaxJSample-int(inC[col])] +
				cc.GYTab[MaxJSample-int(inM[col])] +
				cc.BYTab[MaxJSample-int(inY[col])]
			idx := col * 2
			out[idx+0] = byte(y >> ScaleBits)
			out[idx+1] = inK[col]
		}
	}
}

// nullConvert just interleaves the needed components.
func (cc *ColorConverter) nullConvert(inputBuf [][][]byte, inputRow int, outputBuf [][]byte, numRows int) {
	// Count needed components and figure output stride
	outComps := 0
	compNeeded := make([]bool, len(inputBuf))
	for ci := range inputBuf {
		compNeeded[ci] = true
		outComps++
	}
	numCols := len(outputBuf[0]) / outComps
	if outComps == 0 {
		numCols = len(outputBuf[0])
	}

	for row := 0; row < numRows; row++ {
		out := outputBuf[row]
		startPtr := 0
		for ci := 0; ci < len(inputBuf); ci++ {
			if !compNeeded[ci] {
				continue
			}
			in := inputBuf[ci][inputRow]
			outPos := startPtr
			for col := 0; col < numCols; col++ {
				out[outPos] = in[col]
				outPos += outComps
			}
			startPtr++
		}
		inputRow++
	}
}

// rangeLimitSlice returns a slice suitable for range-limit lookups.
// We offset so that index 0 corresponds to value -(MaxJSample+1),
// matching the libjpeg convention.
func (cc *ColorConverter) rangeLimitSlice() []byte {
	// Return the full table; callers index with value + MaxJSample + 1
	// This matches the rangeLimit helper in types.go
	return BuildSampleRangeLimit()
}
