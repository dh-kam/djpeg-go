package output

import (
	"fmt"
	"sort"
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
	OnePass       bool
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

	cm, err := quantizeColormap(rows, info, opts)
	if err != nil {
		return nil, nil, err
	}
	dither := opts.Dither
	if dither == DitherDefault {
		dither = DitherFS
	}
	if !opts.OnePass && opts.Colormap == nil && info.ColorSpace == ColorSpaceRGB && dither == DitherOrdered {
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

func quantizeColormap(rows [][]byte, info *ImageInfo, opts QuantizeOptions) (*Colormap, error) {
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
		if !opts.OnePass {
			return makeRGBMedianCutPalette(rows, info, desired)
		}
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

type rgbHistPoint struct {
	r, g, b          int
	count            int
	sumR, sumG, sumB int64
}

type rgbColorBox struct {
	points                 []rgbHistPoint
	count                  int
	sumR, sumG, sumB       int64
	minR, maxR, minG, maxG int
	minB, maxB             int
}

func makeRGBMedianCutPalette(rows [][]byte, info *ImageInfo, desired int) (*Colormap, error) {
	points := collectRGBHistogram(rows, info)
	if len(points) == 0 {
		return makeRGBPalette(desired), nil
	}
	if len(points) == 1 {
		p := points[0]
		return &Colormap{
			Maps:      [][]uint8{{byte(p.r)}, {byte(p.g)}, {byte(p.b)}},
			NumColors: 1,
		}, nil
	}

	boxes := []rgbColorBox{newRGBColorBox(points)}
	for len(boxes) < desired {
		idx := selectRGBSplitBox(boxes)
		if idx < 0 {
			break
		}
		left, right, ok := splitRGBBox(boxes[idx])
		if !ok {
			break
		}
		boxes[idx] = left
		boxes = append(boxes, right)
	}

	r := make([]byte, len(boxes))
	g := make([]byte, len(boxes))
	b := make([]byte, len(boxes))
	for i, box := range boxes {
		count := int64(box.count)
		r[i] = byte((box.sumR + count/2) / count)
		g[i] = byte((box.sumG + count/2) / count)
		b[i] = byte((box.sumB + count/2) / count)
	}
	return &Colormap{Maps: [][]uint8{r, g, b}, NumColors: len(boxes)}, nil
}

func collectRGBHistogram(rows [][]byte, info *ImageInfo) []rgbHistPoint {
	type rgbBin struct {
		count            int
		sumR, sumG, sumB int64
	}
	bins := make(map[uint16]*rgbBin, rgbHistogramCapacity(info.Width, info.Height))
	for _, row := range rows {
		for x := 0; x < info.Width; x++ {
			i := x * 3
			r := row[i]
			g := row[i+1]
			b := row[i+2]
			key := uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
			bin := bins[key]
			if bin == nil {
				bin = &rgbBin{}
				bins[key] = bin
			}
			bin.count++
			bin.sumR += int64(r)
			bin.sumG += int64(g)
			bin.sumB += int64(b)
		}
	}

	points := make([]rgbHistPoint, 0, len(bins))
	for _, bin := range bins {
		count := int64(bin.count)
		points = append(points, rgbHistPoint{
			r:     int((bin.sumR + count/2) / count),
			g:     int((bin.sumG + count/2) / count),
			b:     int((bin.sumB + count/2) / count),
			count: bin.count,
			sumR:  bin.sumR,
			sumG:  bin.sumG,
			sumB:  bin.sumB,
		})
	}
	sort.Slice(points, func(i, j int) bool {
		return rgbPointLess(points[i], points[j], 0)
	})
	return points
}

func newRGBColorBox(points []rgbHistPoint) rgbColorBox {
	box := rgbColorBox{
		points: points,
		minR:   255,
		minG:   255,
		minB:   255,
	}
	for _, p := range points {
		box.count += p.count
		box.sumR += p.sumR
		box.sumG += p.sumG
		box.sumB += p.sumB
		if p.r < box.minR {
			box.minR = p.r
		}
		if p.r > box.maxR {
			box.maxR = p.r
		}
		if p.g < box.minG {
			box.minG = p.g
		}
		if p.g > box.maxG {
			box.maxG = p.g
		}
		if p.b < box.minB {
			box.minB = p.b
		}
		if p.b > box.maxB {
			box.maxB = p.b
		}
	}
	return box
}

func selectRGBSplitBox(boxes []rgbColorBox) int {
	best := -1
	var bestScore int64
	for i, box := range boxes {
		if len(box.points) < 2 {
			continue
		}
		score := int64(box.splitRangeScore()) * int64(box.count)
		if best < 0 || score > bestScore || score == bestScore && box.count > boxes[best].count {
			best = i
			bestScore = score
		}
	}
	return best
}

func splitRGBBox(box rgbColorBox) (rgbColorBox, rgbColorBox, bool) {
	if len(box.points) < 2 {
		return rgbColorBox{}, rgbColorBox{}, false
	}
	axis := box.splitAxis()
	sort.Slice(box.points, func(i, j int) bool {
		return rgbPointLess(box.points[i], box.points[j], axis)
	})

	half := box.count / 2
	acc := 0
	split := 1
	for i, p := range box.points {
		acc += p.count
		if acc >= half {
			split = i + 1
			break
		}
	}
	if split <= 0 {
		split = 1
	}
	if split >= len(box.points) {
		split = len(box.points) / 2
	}
	if split <= 0 || split >= len(box.points) {
		return rgbColorBox{}, rgbColorBox{}, false
	}
	return newRGBColorBox(box.points[:split]), newRGBColorBox(box.points[split:]), true
}

func (box rgbColorBox) splitAxis() int {
	rRange := (box.maxR - box.minR) * 2
	gRange := (box.maxG - box.minG) * 3
	bRange := box.maxB - box.minB
	if gRange >= rRange && gRange >= bRange {
		return 1
	}
	if rRange >= bRange {
		return 0
	}
	return 2
}

func (box rgbColorBox) splitRangeScore() int {
	rRange := (box.maxR - box.minR) * 2
	gRange := (box.maxG - box.minG) * 3
	bRange := box.maxB - box.minB
	if gRange >= rRange && gRange >= bRange {
		return gRange
	}
	if rRange >= bRange {
		return rRange
	}
	return bRange
}

func rgbPointLess(a, b rgbHistPoint, axis int) bool {
	switch axis {
	case 1:
		if a.g != b.g {
			return a.g < b.g
		}
	case 2:
		if a.b != b.b {
			return a.b < b.b
		}
	default:
		if a.r != b.r {
			return a.r < b.r
		}
	}
	if a.r != b.r {
		return a.r < b.r
	}
	if a.g != b.g {
		return a.g < b.g
	}
	return a.b < b.b
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

func minInt2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func rgbHistogramCapacity(width, height int) int {
	if width <= 0 || height <= 0 {
		return 0
	}
	if width > maxInt()/height {
		return 32768
	}
	return minInt2(width*height, 32768)
}
