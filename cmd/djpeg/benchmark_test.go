package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/output"
)

func loadRandom100JPEGs(b *testing.B) [][]byte {
	b.Helper()

	matches, err := filepath.Glob("../../testdata/random100/*.jpg")
	if err != nil {
		b.Fatal(err)
	}
	if len(matches) == 0 {
		b.Skip("testdata/random100/*.jpg not available")
	}
	sort.Strings(matches)

	images := make([][]byte, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatalf("reading %s: %v", path, err)
		}
		images = append(images, data)
	}
	return images
}

func benchmarkDecompressRandom100(b *testing.B, noSmooth bool) {
	images := loadRandom100JPEGs(b)
	opts := &config{
		Format:    output.FormatPPM,
		DctMethod: "int",
		NoSmooth:  noSmooth,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, data := range images {
			if err := decompress(bytes.NewReader(data), io.Discard, opts); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkDecompressRandom100Default(b *testing.B) {
	benchmarkDecompressRandom100(b, false)
}

func BenchmarkDecompressRandom100NoSmooth(b *testing.B) {
	benchmarkDecompressRandom100(b, true)
}
