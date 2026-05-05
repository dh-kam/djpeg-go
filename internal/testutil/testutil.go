// Package testutil provides testing utilities for pixel-level validation
// of JPEG decoder output against reference implementations.
package testutil

// PixelComparator compares two image pixel buffers sample-by-sample and
// computes detailed difference statistics.
//
// ref and ours are raw pixel data laid out in row-major order with nc
// components (channels) per pixel. Dimensions w x h must be the same for
// both buffers.
type PixelComparator struct {
	ref    []byte
	ours   []byte
	w      int
	h      int
	nc     int

	// cached results
	computed   bool
	maxDiff    int
	avgDiff    float64
	diffCount  int
	histogram  map[int]int
	exact      bool
	diffPos    []DiffPosition
}

// DiffPosition records the coordinates of a single sample difference.
type DiffPosition struct {
	X     int // column (0..w-1)
	Y     int // row (0..h-1)
	C     int // component/channel (0..nc-1)
	Ref   int // expected value
	Ours  int // actual value
	Delta int // absolute difference
}

// NewPixelComparator creates a comparator for two pixel buffers.
// ref = reference pixel data, ours = decoder output.
// w, h = image dimensions, nc = number of components (1 for grayscale, 3 for RGB).
func NewPixelComparator(ref, ours []byte, w, h, nc int) *PixelComparator {
	return &PixelComparator{
		ref:  ref,
		ours: ours,
		w:    w,
		h:    h,
		nc:   nc,
	}
}

// compute runs the comparison once and caches all results.
func (c *PixelComparator) compute() {
	if c.computed {
		return
	}
	c.computed = true

	c.histogram = make(map[int]int)
	totalSamples := c.w * c.h * c.nc
	sumDiff := 0
	c.maxDiff = 0
	c.diffCount = 0
	c.diffPos = nil
	c.exact = true

	rowStride := c.w * c.nc

	for y := 0; y < c.h; y++ {
		for x := 0; x < c.w; x++ {
			for ch := 0; ch < c.nc; ch++ {
				idx := y*rowStride + x*c.nc + ch
				if idx >= len(c.ref) || idx >= len(c.ours) {
					continue
				}
				refVal := int(c.ref[idx])
				ourVal := int(c.ours[idx])
				d := refVal - ourVal
				if d < 0 {
					d = -d
				}
				c.histogram[d]++

				if d > 0 {
					c.exact = false
					c.diffCount++
					sumDiff += d
					if len(c.diffPos) < 1000 {
						c.diffPos = append(c.diffPos, DiffPosition{
							X:     x,
							Y:     y,
							C:     ch,
							Ref:   refVal,
							Ours:  ourVal,
							Delta: d,
						})
					}
				}
				if d > c.maxDiff {
					c.maxDiff = d
				}
			}
		}
	}

	if totalSamples > 0 {
		c.avgDiff = float64(sumDiff) / float64(totalSamples)
	}
}

// MaxDiff returns the maximum absolute sample difference (0 = exact match).
func (c *PixelComparator) MaxDiff() int {
	c.compute()
	return c.maxDiff
}

// AvgDiff returns the average absolute difference across all samples.
func (c *PixelComparator) AvgDiff() float64 {
	c.compute()
	return c.avgDiff
}

// DiffCount returns the number of samples that differ between ref and ours.
func (c *PixelComparator) DiffCount() int {
	c.compute()
	return c.diffCount
}

// DiffHistogram returns a map from absolute difference value to the count
// of samples with that difference. Key 0 means exact match.
func (c *PixelComparator) DiffHistogram() map[int]int {
	c.compute()
	return c.histogram
}

// IsExact returns true if every sample matches exactly (max diff == 0).
func (c *PixelComparator) IsExact() bool {
	c.compute()
	return c.exact
}

// DiffPositions returns up to 1000 positions where the samples differ.
func (c *PixelComparator) DiffPositions() []DiffPosition {
	c.compute()
	return c.diffPos
}

// TotalSamples returns the total number of sample values compared.
func (c *PixelComparator) TotalSamples() int {
	return c.w * c.h * c.nc
}

// PctExact returns the percentage of samples that match exactly.
func (c *PixelComparator) PctExact() float64 {
	c.compute()
	total := c.TotalSamples()
	if total == 0 {
		return 100.0
	}
	return 100.0 * float64(total-c.diffCount) / float64(total)
}
