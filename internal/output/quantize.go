package output

import (
	"fmt"
	"strings"
)

// DitherMode selects the dithering algorithm used while mapping pixels to a
// palette.
type DitherMode int

const (
	DitherDefault DitherMode = iota
	DitherNone
	DitherOrdered
	DitherFS
)

// QuantizeOptions controls CLI-level color quantization.
type QuantizeOptions struct {
	DesiredColors int
	Colormap      *Colormap
	Dither        DitherMode
}

// ParseDitherMode converts a libjpeg-style dither name.
func ParseDitherMode(s string) (DitherMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "default":
		return DitherDefault, nil
	case "none":
		return DitherNone, nil
	case "ordered":
		return DitherOrdered, nil
	case "fs", "floyd-steinberg", "floyd":
		return DitherFS, nil
	default:
		return DitherDefault, fmt.Errorf("unsupported dither mode %q", s)
	}
}

// QuantizeRows maps full pixel rows to palette-index rows and returns the
// colormap used for demapping or indexed output.
func QuantizeRows(rows [][]byte, info *ImageInfo, opts QuantizeOptions) ([][]byte, *Colormap, error) {
	if info == nil {
		return nil, nil, fmt.Errorf("quantize: nil image info")
	}
	if info.Width < 0 || info.Height < 0 {
		return nil, nil, fmt.Errorf("quantize: invalid dimensions %dx%d", info.Width, info.Height)
	}
	if len(rows) != info.Height {
		return nil, nil, fmt.Errorf("quantize: got %d rows, want %d", len(rows), info.Height)
	}

	components, err := quantizeComponents(info.ColorSpace)
	if err != nil {
		return nil, nil, err
	}
	rowStride := info.Width * components
	for i, row := range rows {
		if len(row) < rowStride {
			return nil, nil, fmt.Errorf("quantize: row %d has %d bytes, want at least %d", i, len(row), rowStride)
		}
	}

	cm, err := quantizeColormap(info, opts)
	if err != nil {
		return nil, nil, err
	}
	dither := opts.Dither
	if dither == DitherDefault {
		dither = DitherFS
	}

	indexed := make([][]byte, info.Height)
	for y := range indexed {
		indexed[y] = make([]byte, info.Width)
	}

	switch dither {
	case DitherNone:
		quantizeNearest(rows, indexed, info, cm, nil)
	case DitherOrdered:
		quantizeOrdered(rows, indexed, info, cm)
	case DitherFS:
		quantizeFloydSteinberg(rows, indexed, info, cm)
	default:
		return nil, nil, fmt.Errorf("quantize: unsupported dither mode %d", dither)
	}

	return indexed, cm, nil
}

func quantizeComponents(cs ColorSpace) (int, error) {
	switch cs {
	case ColorSpaceGrayscale:
		return 1, nil
	case ColorSpaceRGB:
		return 3, nil
	default:
		return 0, fmt.Errorf("quantize: unsupported color space %d", cs)
	}
}

func quantizeColormap(info *ImageInfo, opts QuantizeOptions) (*Colormap, error) {
	if opts.Colormap != nil {
		return normalizeColormap(opts.Colormap, info.ColorSpace)
	}

	desired := opts.DesiredColors
	if desired == 0 {
		desired = 256
	}
	if desired < 2 {
		return nil, fmt.Errorf("quantize: cannot quantize to fewer than 2 colors")
	}
	if desired > 256 {
		return nil, fmt.Errorf("quantize: cannot quantize to more than 256 colors")
	}

	switch info.ColorSpace {
	case ColorSpaceGrayscale:
		return makeGrayPalette(desired), nil
	case ColorSpaceRGB:
		return makeRGBPalette(desired), nil
	default:
		return nil, fmt.Errorf("quantize: unsupported color space %d", info.ColorSpace)
	}
}

func normalizeColormap(cm *Colormap, cs ColorSpace) (*Colormap, error) {
	if cm.NumColors <= 0 {
		return nil, fmt.Errorf("quantize: empty colormap")
	}
	if cm.NumColors > 256 {
		return nil, fmt.Errorf("quantize: colormap has %d colors, max is 256", cm.NumColors)
	}
	if len(cm.Maps) == 0 {
		return nil, fmt.Errorf("quantize: colormap has no channels")
	}
	for i, m := range cm.Maps {
		if len(m) < cm.NumColors {
			return nil, fmt.Errorf("quantize: colormap channel %d has %d entries, want %d", i, len(m), cm.NumColors)
		}
	}

	switch cs {
	case ColorSpaceGrayscale:
		gray := make([]byte, cm.NumColors)
		if len(cm.Maps) >= 3 {
			for i := 0; i < cm.NumColors; i++ {
				gray[i] = rgbToGray(cm.Maps[0][i], cm.Maps[1][i], cm.Maps[2][i])
			}
		} else {
			copy(gray, cm.Maps[0][:cm.NumColors])
		}
		return &Colormap{Maps: [][]uint8{gray}, NumColors: cm.NumColors}, nil
	case ColorSpaceRGB:
		r := make([]byte, cm.NumColors)
		g := make([]byte, cm.NumColors)
		b := make([]byte, cm.NumColors)
		if len(cm.Maps) >= 3 {
			copy(r, cm.Maps[0][:cm.NumColors])
			copy(g, cm.Maps[1][:cm.NumColors])
			copy(b, cm.Maps[2][:cm.NumColors])
		} else {
			copy(r, cm.Maps[0][:cm.NumColors])
			copy(g, cm.Maps[0][:cm.NumColors])
			copy(b, cm.Maps[0][:cm.NumColors])
		}
		return &Colormap{Maps: [][]uint8{r, g, b}, NumColors: cm.NumColors}, nil
	default:
		return nil, fmt.Errorf("quantize: unsupported color space %d", cs)
	}
}

func makeGrayPalette(n int) *Colormap {
	m := make([]byte, n)
	for i := 0; i < n; i++ {
		m[i] = byte((i*255 + (n-1)/2) / (n - 1))
	}
	return &Colormap{Maps: [][]uint8{m}, NumColors: n}
}

func makeRGBPalette(desired int) *Colormap {
	rLevels, gLevels, bLevels := chooseRGBLevels(desired)
	numColors := rLevels * gLevels * bLevels
	r := make([]byte, 0, numColors)
	g := make([]byte, 0, numColors)
	b := make([]byte, 0, numColors)
	for ri := 0; ri < rLevels; ri++ {
		rv := levelValue(ri, rLevels)
		for gi := 0; gi < gLevels; gi++ {
			gv := levelValue(gi, gLevels)
			for bi := 0; bi < bLevels; bi++ {
				r = append(r, rv)
				g = append(g, gv)
				b = append(b, levelValue(bi, bLevels))
			}
		}
	}
	return &Colormap{Maps: [][]uint8{r, g, b}, NumColors: numColors}
}

func chooseRGBLevels(desired int) (int, int, int) {
	bestR, bestG, bestB := 2, 1, 1
	bestProduct := 2
	bestScore := minInt()
	maxLevel := 16
	for r := 1; r <= maxLevel; r++ {
		for g := 1; g <= maxLevel; g++ {
			for b := 1; b <= maxLevel; b++ {
				product := r * g * b
				if product < 2 || product > desired {
					continue
				}
				score := product*1000 + g*8 - absInt(r-g)*4 - absInt(g-b)*4
				if product > bestProduct || product == bestProduct && score > bestScore {
					bestR, bestG, bestB = r, g, b
					bestProduct = product
					bestScore = score
				}
			}
		}
	}
	return bestR, bestG, bestB
}

func levelValue(i, levels int) byte {
	if levels <= 1 {
		return 128
	}
	return byte((i*255 + (levels-1)/2) / (levels - 1))
}

func quantizeNearest(rows, indexed [][]byte, info *ImageInfo, cm *Colormap, adjusted func(x, y int, p []int)) {
	cache := make(map[uint32]byte)
	pixel := make([]int, 3)
	for y, row := range rows {
		out := indexed[y]
		if info.ColorSpace == ColorSpaceGrayscale {
			for x := 0; x < info.Width; x++ {
				pixel[0] = int(row[x])
				if adjusted != nil {
					adjusted(x, y, pixel[:1])
				}
				out[x] = nearestGray(pixel[0], cm)
			}
			continue
		}

		for x := 0; x < info.Width; x++ {
			i := x * 3
			pixel[0] = int(row[i])
			pixel[1] = int(row[i+1])
			pixel[2] = int(row[i+2])
			if adjusted != nil {
				adjusted(x, y, pixel)
				out[x] = nearestRGB(pixel[0], pixel[1], pixel[2], cm)
				continue
			}
			key := uint32(row[i])<<16 | uint32(row[i+1])<<8 | uint32(row[i+2])
			if idx, ok := cache[key]; ok {
				out[x] = idx
				continue
			}
			idx := nearestRGB(pixel[0], pixel[1], pixel[2], cm)
			cache[key] = idx
			out[x] = idx
		}
	}
}

func quantizeOrdered(rows, indexed [][]byte, info *ImageInfo, cm *Colormap) {
	bayer := [4][4]int{
		{0, 8, 2, 10},
		{12, 4, 14, 6},
		{3, 11, 1, 9},
		{15, 7, 13, 5},
	}
	quantizeNearest(rows, indexed, info, cm, func(x, y int, p []int) {
		adjust := (bayer[y&3][x&3] - 8) * 4
		for i := range p {
			p[i] = clamp8(p[i] + adjust)
		}
	})
}

func quantizeFloydSteinberg(rows, indexed [][]byte, info *ImageInfo, cm *Colormap) {
	width := info.Width
	if info.ColorSpace == ColorSpaceGrayscale {
		curr := make([]int, width+2)
		next := make([]int, width+2)
		for y, row := range rows {
			out := indexed[y]
			for x := 0; x < width; x++ {
				old := clamp8(int(row[x]) + divRound(curr[x+1], 16))
				idx := nearestGray(old, cm)
				out[x] = idx
				newValue := int(cm.Maps[0][idx])
				err := old - newValue
				curr[x+2] += err * 7
				next[x] += err * 3
				next[x+1] += err * 5
				next[x+2] += err
			}
			curr, next = next, curr
			clear(next)
		}
		return
	}

	curr := make([][3]int, width+2)
	next := make([][3]int, width+2)
	for y, row := range rows {
		out := indexed[y]
		for x := 0; x < width; x++ {
			i := x * 3
			oldR := clamp8(int(row[i]) + divRound(curr[x+1][0], 16))
			oldG := clamp8(int(row[i+1]) + divRound(curr[x+1][1], 16))
			oldB := clamp8(int(row[i+2]) + divRound(curr[x+1][2], 16))
			idx := nearestRGB(oldR, oldG, oldB, cm)
			out[x] = idx
			errR := oldR - int(cm.Maps[0][idx])
			errG := oldG - int(cm.Maps[1][idx])
			errB := oldB - int(cm.Maps[2][idx])
			addRGBError(curr, x+2, errR, errG, errB, 7)
			addRGBError(next, x, errR, errG, errB, 3)
			addRGBError(next, x+1, errR, errG, errB, 5)
			addRGBError(next, x+2, errR, errG, errB, 1)
		}
		curr, next = next, curr
		clear(next)
	}
}

func addRGBError(buf [][3]int, idx, r, g, b, weight int) {
	buf[idx][0] += r * weight
	buf[idx][1] += g * weight
	buf[idx][2] += b * weight
}

func nearestGray(v int, cm *Colormap) byte {
	v = clamp8(v)
	bestIdx := 0
	bestDist := maxInt()
	m := cm.Maps[0]
	for i := 0; i < cm.NumColors; i++ {
		d := absInt(v - int(m[i]))
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return byte(bestIdx)
}

func nearestRGB(r, g, b int, cm *Colormap) byte {
	r = clamp8(r)
	g = clamp8(g)
	b = clamp8(b)
	bestIdx := 0
	bestDist := maxInt()
	mr := cm.Maps[0]
	mg := cm.Maps[1]
	mb := cm.Maps[2]
	for i := 0; i < cm.NumColors; i++ {
		dr := r - int(mr[i])
		dg := g - int(mg[i])
		db := b - int(mb[i])
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	return byte(bestIdx)
}

func rgbToGray(r, g, b byte) byte {
	const (
		redScale   = 19595
		greenScale = 38470
		blueScale  = 7471
		oneHalf    = 1 << 15
	)
	y := redScale*int(r) + greenScale*int(g) + blueScale*int(b) + oneHalf
	return byte(y >> 16)
}

func clamp8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func divRound(v, denom int) int {
	if v >= 0 {
		return (v + denom/2) / denom
	}
	return -((-v + denom/2) / denom)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func minInt() int {
	return -maxInt() - 1
}
