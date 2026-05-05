package testutil

import (
	"fmt"
	"strings"
)

// AccuracyReport generates a detailed accuracy comparison report between
// a reference image and the decoder output.
type AccuracyReport struct {
	ImageName    string
	Width        int
	Height       int
	Components   int
	MaxDiff      int
	AvgDiff      float64
	PctExact     float64
	DiffCount    int
	TotalSamples int
	DiffPositions []DiffPosition
	Histogram    map[int]int
}

// GenerateAccuracyReport creates an AccuracyReport from a PixelComparator.
func GenerateAccuracyReport(comparator *PixelComparator, imageName string, w, h, nc int) *AccuracyReport {
	return &AccuracyReport{
		ImageName:     imageName,
		Width:         w,
		Height:        h,
		Components:    nc,
		MaxDiff:       comparator.MaxDiff(),
		AvgDiff:       comparator.AvgDiff(),
		PctExact:      comparator.PctExact(),
		DiffCount:     comparator.DiffCount(),
		TotalSamples:  comparator.TotalSamples(),
		DiffPositions: comparator.DiffPositions(),
		Histogram:     comparator.DiffHistogram(),
	}
}

// String returns a human-readable summary of the accuracy report.
func (r *AccuracyReport) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Accuracy Report: %s\n", r.ImageName))
	sb.WriteString(fmt.Sprintf("  Dimensions: %dx%d, %d components\n", r.Width, r.Height, r.Components))
	sb.WriteString(fmt.Sprintf("  Total samples: %d\n", r.TotalSamples))
	sb.WriteString(fmt.Sprintf("  Exact matches: %.2f%% (%d/%d)\n",
		r.PctExact, r.TotalSamples-r.DiffCount, r.TotalSamples))
	sb.WriteString(fmt.Sprintf("  Max diff: %d\n", r.MaxDiff))
	sb.WriteString(fmt.Sprintf("  Avg diff: %.6f\n", r.AvgDiff))
	sb.WriteString(fmt.Sprintf("  Diff count: %d\n", r.DiffCount))

	if len(r.Histogram) > 0 {
		sb.WriteString("  Diff histogram:\n")
		for d := 0; d <= r.MaxDiff; d++ {
			if count, ok := r.Histogram[d]; ok {
				sb.WriteString(fmt.Sprintf("    diff=%d: %d samples\n", d, count))
			}
		}
	}

	if len(r.DiffPositions) > 0 && len(r.DiffPositions) <= 20 {
		sb.WriteString("  Diff positions (sample level):\n")
		for _, dp := range r.DiffPositions {
			sb.WriteString(fmt.Sprintf("    (%d,%d) ch%d: ref=%d ours=%d delta=%d\n",
				dp.X, dp.Y, dp.C, dp.Ref, dp.Ours, dp.Delta))
		}
	} else if len(r.DiffPositions) > 20 {
		sb.WriteString(fmt.Sprintf("  First 20 of %d diff positions:\n", r.DiffCount))
		for i := 0; i < 20 && i < len(r.DiffPositions); i++ {
			dp := r.DiffPositions[i]
			sb.WriteString(fmt.Sprintf("    (%d,%d) ch%d: ref=%d ours=%d delta=%d\n",
				dp.X, dp.Y, dp.C, dp.Ref, dp.Ours, dp.Delta))
		}
	}

	return sb.String()
}

// Pass returns true if the accuracy meets the given tolerance criteria.
func (r *AccuracyReport) Pass(maxDiff int, minPctExact float64) bool {
	return r.MaxDiff <= maxDiff && r.PctExact >= minPctExact
}
