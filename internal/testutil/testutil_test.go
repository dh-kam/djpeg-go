package testutil

import (
	"testing"
)

func TestPixelComparatorExactMatch(t *testing.T) {
	t.Parallel()

	ref := make([]byte, 100)
	ours := make([]byte, 100)
	for i := range ref {
		ref[i] = byte(i % 256)
		ours[i] = byte(i % 256)
	}

	c := NewPixelComparator(ref, ours, 10, 10, 1)

	if !c.IsExact() {
		t.Error("expected exact match")
	}
	if c.MaxDiff() != 0 {
		t.Errorf("MaxDiff = %d, want 0", c.MaxDiff())
	}
	if c.AvgDiff() != 0 {
		t.Errorf("AvgDiff = %f, want 0", c.AvgDiff())
	}
	if c.DiffCount() != 0 {
		t.Errorf("DiffCount = %d, want 0", c.DiffCount())
	}
	if c.PctExact() != 100.0 {
		t.Errorf("PctExact = %f, want 100", c.PctExact())
	}
}

func TestPixelComparatorAllDifferent(t *testing.T) {
	t.Parallel()

	ref := make([]byte, 10)
	ours := make([]byte, 10)
	for i := range ref {
		ref[i] = 100
		ours[i] = 110
	}

	c := NewPixelComparator(ref, ours, 10, 1, 1)

	if c.IsExact() {
		t.Error("expected not exact match")
	}
	if c.MaxDiff() != 10 {
		t.Errorf("MaxDiff = %d, want 10", c.MaxDiff())
	}
	if c.DiffCount() != 10 {
		t.Errorf("DiffCount = %d, want 10", c.DiffCount())
	}
	if c.PctExact() != 0.0 {
		t.Errorf("PctExact = %f, want 0", c.PctExact())
	}
	hist := c.DiffHistogram()
	if hist[10] != 10 {
		t.Errorf("histogram[10] = %d, want 10", hist[10])
	}
}

func TestPixelComparatorRGB(t *testing.T) {
	t.Parallel()

	// 2x2 RGB image
	ref := []byte{
		100, 150, 200, // pixel (0,0)
		50, 60, 70,    // pixel (1,0)
		10, 20, 30,    // pixel (0,1)
		255, 0, 128,   // pixel (1,1)
	}
	ours := []byte{
		100, 150, 205, // pixel (0,0): blue differs by 5
		50, 60, 70,    // pixel (1,0): exact
		12, 20, 30,    // pixel (0,1): red differs by 2
		255, 0, 128,   // pixel (1,1): exact
	}

	c := NewPixelComparator(ref, ours, 2, 2, 3)

	if c.IsExact() {
		t.Error("expected not exact match")
	}
	if c.MaxDiff() != 5 {
		t.Errorf("MaxDiff = %d, want 5", c.MaxDiff())
	}
	if c.DiffCount() != 2 {
		t.Errorf("DiffCount = %d, want 2", c.DiffCount())
	}
	if c.TotalSamples() != 12 {
		t.Errorf("TotalSamples = %d, want 12", c.TotalSamples())
	}

	positions := c.DiffPositions()
	if len(positions) != 2 {
		t.Fatalf("len(DiffPositions) = %d, want 2", len(positions))
	}
	// First diff should be pixel (0,0) component 2 (blue), delta 5
	p0 := positions[0]
	if p0.X != 0 || p0.Y != 0 || p0.C != 2 || p0.Delta != 5 {
		t.Errorf("positions[0] = {%d,%d,%d} delta=%d, want {0,0,2} delta=5",
			p0.X, p0.Y, p0.C, p0.Delta)
	}
}

func TestPixelComparatorSinglePixelDiff(t *testing.T) {
	t.Parallel()

	ref := []byte{128, 128, 128}
	ours := []byte{128, 127, 128}

	c := NewPixelComparator(ref, ours, 1, 1, 3)

	if c.IsExact() {
		t.Error("expected not exact")
	}
	if c.MaxDiff() != 1 {
		t.Errorf("MaxDiff = %d, want 1", c.MaxDiff())
	}
	if c.AvgDiff() < 0.0 || c.AvgDiff() > 1.0 {
		t.Errorf("AvgDiff = %f, want between 0 and 1", c.AvgDiff())
	}
}

func TestPixelComparatorEmptyBuffers(t *testing.T) {
	t.Parallel()

	ref := []byte{}
	ours := []byte{}
	c := NewPixelComparator(ref, ours, 0, 0, 1)

	if !c.IsExact() {
		t.Error("empty buffers should be exact match")
	}
	if c.MaxDiff() != 0 {
		t.Errorf("MaxDiff = %d, want 0", c.MaxDiff())
	}
	if c.PctExact() != 100.0 {
		t.Errorf("PctExact = %f, want 100", c.PctExact())
	}
}

func TestPixelComparatorCaching(t *testing.T) {
	t.Parallel()

	ref := []byte{10, 20, 30}
	ours := []byte{10, 20, 30}
	c := NewPixelComparator(ref, ours, 3, 1, 1)

	// Call multiple methods -- should only compute once
	_ = c.MaxDiff()
	_ = c.AvgDiff()
	_ = c.DiffCount()
	_ = c.IsExact()
	_ = c.PctExact()
	_ = c.DiffHistogram()

	if !c.computed {
		t.Error("expected computed to be true after first call")
	}
}

func TestNewMinimalJPEGBuilderDefaults(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder()
	if b.width != 8 || b.height != 8 {
		t.Errorf("dimensions = %dx%d, want 8x8", b.width, b.height)
	}
	if b.numComp != 1 {
		t.Errorf("numComp = %d, want 1", b.numComp)
	}
}

func TestBuildGrayscaleJPEG(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetGrayscale().
		SetPattern("solid")
	data := b.Build()

	// Verify SOI marker
	if data[0] != 0xFF || data[1] != 0xD8 {
		t.Error("missing SOI marker")
	}

	// Verify EOI marker
	l := len(data)
	if data[l-2] != 0xFF || data[l-1] != 0xD9 {
		t.Error("missing EOI marker")
	}

	// Verify SOF0 marker exists
	found := false
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] == 0xC0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing SOF0 marker")
	}

	// Should be decodable
	t.Logf("Built grayscale JPEG: %d bytes", len(data))
}

func TestBuildColor444JPEG(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetColor().
		SetPattern("solid")
	data := b.Build()

	if len(data) == 0 {
		t.Error("Build returned empty data")
	}

	// Verify it has the right markers
	hasDQT, hasSOF0, hasDHT, hasSOS := false, false, false, false
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0xFF {
			switch data[i+1] {
			case 0xDB:
				hasDQT = true
			case 0xC0:
				hasSOF0 = true
			case 0xC4:
				hasDHT = true
			case 0xDA:
				hasSOS = true
			}
		}
	}
	if !hasDQT {
		t.Error("missing DQT marker")
	}
	if !hasSOF0 {
		t.Error("missing SOF0 marker")
	}
	if !hasDHT {
		t.Error("missing DHT marker")
	}
	if !hasSOS {
		t.Error("missing SOS marker")
	}

	t.Logf("Built color 4:4:4 JPEG: %d bytes", len(data))
}

func TestBuildColor420JPEG(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder().
		SetDimensions(16, 16).
		SetColor().
		SetSubsampling(0, 2, 2).
		SetSubsampling(1, 1, 1).
		SetSubsampling(2, 1, 1).
		SetPattern("solid")
	data := b.Build()

	if len(data) == 0 {
		t.Error("Build returned empty data")
	}
	t.Logf("Built color 4:2:0 JPEG: %d bytes", len(data))
}

func TestBuildGradientPattern(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder().
		SetDimensions(16, 8).
		SetGrayscale().
		SetPattern("gradient")
	data := b.Build()

	if len(data) == 0 {
		t.Error("Build returned empty data")
	}
	t.Logf("Built gradient JPEG: %d bytes", len(data))
}

func TestExpectedPixelsGrayscale(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetGrayscale().
		SetPattern("solid")
	pixels := b.ExpectedPixels()

	if len(pixels) != 64 {
		t.Fatalf("expected 64 pixels, got %d", len(pixels))
	}
	for i, p := range pixels {
		if p != 128 {
			t.Errorf("pixel[%d] = %d, want 128", i, p)
		}
	}
}

func TestExpectedPixelsGradient(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder().
		SetDimensions(16, 8).
		SetGrayscale().
		SetPattern("gradient")
	pixels := b.ExpectedPixels()

	if len(pixels) != 128 {
		t.Fatalf("expected 128 pixels, got %d", len(pixels))
	}
	// The gradient pattern uses block-level pixel positions (each 8x8 block is uniform).
	// For 16x8 grayscale: MCU col 0 block at pixelX=0 => gradient value = (0*255)/15 = 0
	// MCU col 1 block at pixelX=8 => gradient value = (8*255)/15 = 136
	if pixels[0] != 0 {
		t.Errorf("first pixel = %d, want 0", pixels[0])
	}
	// Block at column 1 (pixels 8..15) should be 136
	if pixels[8] != 136 {
		t.Errorf("pixel at col 8 = %d, want 136", pixels[8])
	}
}

func TestExpectedPixelsColor(t *testing.T) {
	t.Parallel()

	b := NewMinimalJPEGBuilder().
		SetDimensions(8, 8).
		SetColor().
		SetPattern("solid")
	pixels := b.ExpectedPixels()

	if len(pixels) != 192 { // 8*8*3
		t.Fatalf("expected 192 bytes, got %d", len(pixels))
	}
	// All pixels should be (128, 128, 128) since Y=128, Cb=128, Cr=128 => RGB=(128,128,128)
	for i := 0; i < 64; i++ {
		r := pixels[i*3]
		g := pixels[i*3+1]
		bl := pixels[i*3+2]
		if r != 128 || g != 128 || bl != 128 {
			t.Errorf("pixel[%d] = (%d,%d,%d), want (128,128,128)", i, r, g, bl)
		}
	}
}

func TestGoldenJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := GoldenTestData{
		Width:        8,
		Height:       8,
		Components:   1,
		PixelsBase64: encodeBase64([]byte{128, 128, 128}),
		Description:  "test",
	}

	// Verify the base64 round-trips correctly
	decoded, err := DecodeBase64(original.PixelsBase64)
	if err != nil {
		t.Fatalf("DecodeBase64 error: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("decoded length = %d, want 3", len(decoded))
	}
	for i, v := range decoded {
		if v != 128 {
			t.Errorf("decoded[%d] = %d, want 128", i, v)
		}
	}
}

func TestAccuracyReportString(t *testing.T) {
	t.Parallel()

	ref := []byte{100, 150, 200, 50, 60, 70}
	ours := []byte{100, 150, 205, 50, 60, 70}
	c := NewPixelComparator(ref, ours, 2, 1, 3)
	report := GenerateAccuracyReport(c, "test_image", 2, 1, 3)

	s := report.String()
	if len(s) == 0 {
		t.Error("report String() is empty")
	}
	if report.MaxDiff != 5 {
		t.Errorf("MaxDiff = %d, want 5", report.MaxDiff)
	}
	if !report.Pass(5, 0.0) {
		t.Error("report should pass with maxDiff=5 tolerance")
	}
	if report.Pass(4, 0.0) {
		t.Error("report should fail with maxDiff=4 tolerance")
	}
}
