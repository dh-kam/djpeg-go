package decoder

import (
	"bytes"
	"os"
	"testing"
)

// FuzzFullDecode tests that full JPEG decoding does not panic on arbitrary input.
func FuzzFullDecode(f *testing.F) {
	// Seed with real JPEG files
	seeds := []string{
		testFixture("test_gray.jpg"),
		testFixture("test_color.jpg"),
		testFixture("test_420.jpg"),
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err == nil {
			f.Add(data)
		}
	}

	// Also add minimal/invalid seeds
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xD9})             // SOI + EOI
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xC0, 0x00, 0x02}) // truncated SOF0
	f.Add([]byte{})                                   // empty
	f.Add([]byte{0x89, 0x50, 0x4E, 0x47})             // PNG header
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x02}) // truncated APP0
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02}) // truncated SOS

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := New(bytes.NewReader(data))

		// ReadHeader should not panic
		_, _, _, _, _ = dec.ReadHeader()

		// StartDecompress should not panic
		_ = dec.StartDecompress()

		// Try to read scanlines
		scanline := make([]byte, 4096)
		for i := 0; i < 100; i++ {
			n, _ := dec.ReadScanlines([][]byte{scanline})
			if n == 0 {
				break
			}
		}

		// Finish should not panic
		_ = dec.FinishDecompress()
	})
}

// FuzzDecodeToRGB tests the convenience DecodeToRGB function.
func FuzzDecodeToRGB(f *testing.F) {
	data, err := os.ReadFile(testFixture("test_gray.jpg"))
	if err == nil {
		f.Add(data)
	}
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xD9})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic
		_, _, _, _, _ = DecodeToRGB(bytes.NewReader(data))
	})
}

// TestDecodeToRGBColorFile tests DecodeToRGB with a color JPEG.
func TestDecodeToRGBColorFile(t *testing.T) {
	f, err := os.Open(testFixture("test_color.jpg"))
	if err != nil {
		t.Skip("test_color.jpg not available:", err)
	}
	defer f.Close()

	pixels, w, h, components, err := DecodeToRGB(f)
	if err != nil {
		t.Fatalf("DecodeToRGB color: %v", err)
	}
	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Errorf("pixel data length = %d, want %d", len(pixels), expectedLen)
	}
	if components != 3 {
		t.Errorf("components = %d, want 3", components)
	}
}

// TestDecodeToRGB420File tests DecodeToRGB with a 4:2:0 JPEG.
func TestDecodeToRGB420File(t *testing.T) {
	f, err := os.Open(testFixture("test_420.jpg"))
	if err != nil {
		t.Skip("test_420.jpg not available:", err)
	}
	defer f.Close()

	pixels, w, h, components, err := DecodeToRGB(f)
	if err != nil {
		t.Fatalf("DecodeToRGB 420: %v", err)
	}
	expectedLen := w * h * components
	if len(pixels) != expectedLen {
		t.Errorf("pixel data length = %d, want %d", len(pixels), expectedLen)
	}
}

// TestDecoderOutColorSpace tests the OutColorSpace method.
func TestDecoderOutColorSpace(t *testing.T) {
	f, err := os.Open(testFixture("test_gray.jpg"))
	if err != nil {
		t.Skip("test_gray.jpg not available:", err)
	}
	defer f.Close()

	dec := New(f)
	_, _, _, _, err = dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}

	cs := dec.OutColorSpace()
	_ = cs // just verify it doesn't panic

	jcs := dec.JPEGColorSpace()
	_ = jcs
}

// TestDecoderReadScanlinesBadState tests ReadScanlines with wrong state.
func TestDecoderReadScanlinesBadState(t *testing.T) {
	dec := New(bytes.NewReader([]byte{0xFF, 0xD8, 0xFF, 0xD9}))
	scanline := make([]byte, 100)
	n, err := dec.ReadScanlines([][]byte{scanline})
	if err == nil {
		t.Error("expected error for ReadScanlines without StartDecompress")
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}

// TestDecoderFinishWithoutStart tests FinishDecompress without starting.
func TestDecoderFinishWithoutStart(t *testing.T) {
	dec := New(bytes.NewReader([]byte{0xFF, 0xD8, 0xFF, 0xD9}))
	err := dec.FinishDecompress()
	// Should not panic
	_ = err
}

// TestDecoderProgressiveReject tests that progressive JPEG is rejected.
func TestDecoderProgressiveReject(t *testing.T) {
	// Build a minimal progressive JPEG (SOF2)
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xC2}) // SOF2 = progressive
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xC4})
	buf.Write([]byte{0x00, 0x14})
	buf.WriteByte(0x10)
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write([]byte{0x00})
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x3F)
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0xFF, 0xD9})

	dec := New(bytes.NewReader(buf.Bytes()))
	_, _, _, _, err := dec.ReadHeader()
	if err == nil {
		t.Error("expected error for progressive JPEG")
	}
}

// TestDecoderArithmeticReject tests that arithmetic coding JPEG is rejected.
func TestDecoderArithmeticReject(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xDB})
	buf.Write([]byte{0x00, 0x43})
	buf.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		buf.WriteByte(0x01)
	}
	buf.Write([]byte{0xFF, 0xC9}) // SOF9 = arithmetic
	buf.Write([]byte{0x00, 0x0B})
	buf.WriteByte(0x08)
	buf.Write([]byte{0x00, 0x08})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x11)
	buf.WriteByte(0x00)
	buf.Write([]byte{0xFF, 0xDA})
	buf.Write([]byte{0x00, 0x08})
	buf.WriteByte(0x01)
	buf.WriteByte(0x01)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x3F)
	buf.WriteByte(0x00)
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0xFF, 0xD9})

	dec := New(bytes.NewReader(buf.Bytes()))
	_, _, _, _, err := dec.ReadHeader()
	if err == nil {
		t.Error("expected error for arithmetic coding JPEG")
	}
}

// TestDecodeWithIDCTMethods tests decoding with different IDCT methods.
func TestDecodeWithIDCTMethods(t *testing.T) {
	methods := []string{"slow", "fast", "float", "unknown"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			f, err := os.Open(testFixture("test_gray.jpg"))
			if err != nil {
				t.Skip("test_gray.jpg not available:", err)
			}
			defer f.Close()

			dec := New(f)
			dec.SetIDCTMethod(method)

			w, h, _, _, err := dec.ReadHeader()
			if err != nil {
				t.Fatalf("ReadHeader: %v", err)
			}

			if err := dec.StartDecompress(); err != nil {
				t.Fatalf("StartDecompress: %v", err)
			}

			scanline := make([]byte, w*dec.OutputComponents())
			rowsRead := 0
			for rowsRead < h {
				n, err := dec.ReadScanlines([][]byte{scanline})
				if err != nil {
					t.Fatalf("ReadScanlines: %v", err)
				}
				if n == 0 {
					break
				}
				rowsRead += n
			}

			if rowsRead != h {
				t.Errorf("read %d rows, want %d", rowsRead, h)
			}

			dec.FinishDecompress()
		})
	}
}

// TestDecodeColorWithIDCTMethods tests color JPEG decoding with different IDCT methods.
func TestDecodeColorWithIDCTMethods(t *testing.T) {
	methods := []string{"slow", "fast", "float"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			f, err := os.Open(testFixture("test_color.jpg"))
			if err != nil {
				t.Skip("test_color.jpg not available:", err)
			}
			defer f.Close()

			dec := New(f)
			dec.SetIDCTMethod(method)

			w, h, _, _, err := dec.ReadHeader()
			if err != nil {
				t.Fatalf("ReadHeader: %v", err)
			}

			if err := dec.StartDecompress(); err != nil {
				t.Fatalf("StartDecompress: %v", err)
			}

			scanline := make([]byte, w*dec.OutputComponents())
			rowsRead := 0
			for rowsRead < h {
				n, err := dec.ReadScanlines([][]byte{scanline})
				if err != nil {
					t.Fatalf("ReadScanlines: %v", err)
				}
				if n == 0 {
					break
				}
				rowsRead += n
			}

			if rowsRead != h {
				t.Errorf("read %d rows, want %d", rowsRead, h)
			}

			dec.FinishDecompress()
		})
	}
}

// TestDecode420WithIDCTMethods tests 4:2:0 JPEG decoding with different IDCT methods.
func TestDecode420WithIDCTMethods(t *testing.T) {
	methods := []string{"slow", "fast", "float"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			f, err := os.Open(testFixture("test_420.jpg"))
			if err != nil {
				t.Skip("test_420.jpg not available:", err)
			}
			defer f.Close()

			dec := New(f)
			dec.SetIDCTMethod(method)

			w, h, _, _, err := dec.ReadHeader()
			if err != nil {
				t.Fatalf("ReadHeader: %v", err)
			}

			if err := dec.StartDecompress(); err != nil {
				t.Fatalf("StartDecompress: %v", err)
			}

			scanline := make([]byte, w*dec.OutputComponents())
			rowsRead := 0
			for rowsRead < h {
				n, err := dec.ReadScanlines([][]byte{scanline})
				if err != nil {
					t.Fatalf("ReadScanlines: %v", err)
				}
				if n == 0 {
					break
				}
				rowsRead += n
			}

			if rowsRead != h {
				t.Errorf("read %d rows, want %d", rowsRead, h)
			}

			dec.FinishDecompress()
		})
	}
}
