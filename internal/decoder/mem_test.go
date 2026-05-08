package decoder

import (
	"bytes"
	"os"
	"runtime"
	"testing"
)

// loadTestJPEG loads a test JPEG fixture from testdata.
func loadTestJPEG(name string) []byte {
	data, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		return nil
	}
	return data
}

func TestMemoryUsageGray(t *testing.T) {
	jpegData := loadTestJPEG("test_gray.jpg")
	if jpegData == nil {
		t.Skip("test_gray.jpg not available")
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	pixels, w, h, components, err := DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}
	_ = pixels
	_ = w
	_ = h
	_ = components

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocBytes := m2.TotalAlloc - m1.TotalAlloc
	allocKB := float64(allocBytes) / 1024
	t.Logf("Grayscale JPEG memory allocated: %.2f KB", allocKB)
	t.Logf("Image dimensions: %dx%d, components: %d", w, h, components)
}

func TestMemoryUsageColor(t *testing.T) {
	jpegData := loadTestJPEG("test_color.jpg")
	if jpegData == nil {
		t.Skip("test_color.jpg not available")
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	pixels, w, h, components, err := DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}
	_ = pixels
	_ = w
	_ = h
	_ = components

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocBytes := m2.TotalAlloc - m1.TotalAlloc
	allocKB := float64(allocBytes) / 1024
	t.Logf("Color JPEG memory allocated: %.2f KB", allocKB)
	t.Logf("Image dimensions: %dx%d, components: %d", w, h, components)
}

func TestMemoryUsage420(t *testing.T) {
	jpegData := loadTestJPEG("test_420.jpg")
	if jpegData == nil {
		t.Skip("test_420.jpg not available")
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	pixels, w, h, components, err := DecodeToRGB(bytes.NewReader(jpegData))
	if err != nil {
		t.Fatalf("DecodeToRGB failed: %v", err)
	}
	_ = pixels
	_ = w
	_ = h
	_ = components

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocBytes := m2.TotalAlloc - m1.TotalAlloc
	allocKB := float64(allocBytes) / 1024
	t.Logf("4:2:0 JPEG memory allocated: %.2f KB", allocKB)
	t.Logf("Image dimensions: %dx%d, components: %d", w, h, components)
}

func TestAllocationCount(t *testing.T) {
	jpegData := loadTestJPEG("test_420.jpg")
	if jpegData == nil {
		t.Skip("test_420.jpg not available")
	}

	// Run once to warm up
	DecodeToRGB(bytes.NewReader(jpegData))

	// Measure allocations for a single decode
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	_, _, _, _, _ = DecodeToRGB(bytes.NewReader(jpegData))

	runtime.ReadMemStats(&m2)

	mallocs := m2.Mallocs - m1.Mallocs
	frees := m2.Frees - m1.Frees
	allocBytes := m2.TotalAlloc - m1.TotalAlloc
	t.Logf("4:2:0 single decode allocations: %d mallocs, %d frees", mallocs, frees)
	t.Logf("Total allocated: %d bytes (%.2f KB)", allocBytes, float64(allocBytes)/1024)
}

// BenchmarkMemoryAllocation tracks allocations per decode operation.
func BenchmarkMemoryAllocation(b *testing.B) {
	jpegData := loadTestJPEG("test_420.jpg")
	if jpegData == nil {
		b.Skip("test_420.jpg not available")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = DecodeToRGB(bytes.NewReader(jpegData))
	}
}

func BenchmarkMemoryAllocationGray(b *testing.B) {
	jpegData := loadTestJPEG("test_gray.jpg")
	if jpegData == nil {
		b.Skip("test_gray.jpg not available")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = DecodeToRGB(bytes.NewReader(jpegData))
	}
}

func BenchmarkMemoryAllocationColor(b *testing.B) {
	jpegData := loadTestJPEG("test_color.jpg")
	if jpegData == nil {
		b.Skip("test_color.jpg not available")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = DecodeToRGB(bytes.NewReader(jpegData))
	}
}
