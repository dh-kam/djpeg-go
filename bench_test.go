package djpeggo_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/decoder"
)

// loadJPEG loads a test JPEG file, skipping if not available.
func loadJPEG(b *testing.B, name string) []byte {
	b.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		b.Skip(name, "not available:", err)
	}
	return data
}

func BenchmarkDecodeGrayJPEG(b *testing.B) {
	data := loadJPEG(b, "test_gray.jpg")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = decoder.DecodeToRGB(bytes.NewReader(data))
	}
}

func BenchmarkDecodeColorJPEG(b *testing.B) {
	data := loadJPEG(b, "test_color.jpg")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = decoder.DecodeToRGB(bytes.NewReader(data))
	}
}

func BenchmarkDecode420JPEG(b *testing.B) {
	data := loadJPEG(b, "test_420.jpg")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = decoder.DecodeToRGB(bytes.NewReader(data))
	}
}
