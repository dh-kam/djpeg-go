package output

import (
	"fmt"
	"io"
)

// targaWriter writes Targa/TGA image format.
//
// Supports uncompressed grayscale (8-bit) and uncompressed true-color
// (24-bit BGR) images. Top-down, non-interlaced. When color quantization
// is active with a palette, the writer emits a colormapped TGA image.
type targaWriter struct {
	w    io.Writer
	info *ImageInfo
	buf  []byte // reusable row buffer for BGR conversion
}

// Start writes the TGA file header.
func (t *targaWriter) Start(w io.Writer, info *ImageInfo) error {
	t.w = w
	t.info = info

	// Determine image type and bits per pixel
	var imageType byte
	var bitsPerPixel byte
	var numColors int
	var useColormap bool

	switch info.ColorSpace {
	case ColorSpaceGrayscale:
		// Targa doesn't have a mapped grayscale format; demap if quantized.
		imageType = 3 // uncompressed grayscale
		bitsPerPixel = 8
		useColormap = false

	case ColorSpaceRGB, ColorSpaceYCbCr, ColorSpaceBigGamutYCbCr, ColorSpaceCMYK, ColorSpaceYCCK:
		if info.QuantizeColors && info.Colormap != nil {
			numColors = info.Colormap.NumColors
			if numColors > 256 {
				return fmt.Errorf("targa: too many colors (%d)", numColors)
			}
			imageType = 1 // colormapped
			bitsPerPixel = 8
			useColormap = true
		} else {
			imageType = 2 // uncompressed true-color
			bitsPerPixel = 24
			useColormap = false
		}

	default:
		return fmt.Errorf("targa: unsupported color space")
	}

	// Build the 18-byte TGA header
	var header [18]byte

	if useColormap {
		header[1] = 1 // color map type 1 (has colormap)
		header[5] = byte(numColors & 0xFF)
		header[6] = byte(numColors >> 8)
		header[7] = 24 // bits per color map entry
	}

	header[2] = imageType
	header[12] = byte(info.Width & 0xFF)
	header[13] = byte(info.Width >> 8)
	header[14] = byte(info.Height & 0xFF)
	header[15] = byte(info.Height >> 8)
	header[16] = bitsPerPixel
	header[17] = 0x20 // top-down, non-interlaced

	if _, err := w.Write(header[:]); err != nil {
		return fmt.Errorf("targa: writing header: %w", err)
	}

	// Write the colormap if present (BGR order)
	if useColormap && info.Colormap != nil {
		cm := rgbColormap(info.Colormap, info.ColorSpace)
		for i := 0; i < numColors; i++ {
			// TGA colormap is BGR
			b := cm.Maps[2][i]
			gv := cm.Maps[1][i]
			r := cm.Maps[0][i]
			if _, err := w.Write([]byte{b, gv, r}); err != nil {
				return fmt.Errorf("targa: writing colormap: %w", err)
			}
		}
	}

	// Allocate row buffer if needed for RGB -> BGR conversion
	if colorSpaceCanWriteRGB(info.ColorSpace) && !useColormap {
		t.buf = make([]byte, info.Width*3)
	}

	return nil
}

// WriteScanline writes one row of pixel data.
// For full-color RGB, pixels are converted from RGB to BGR order.
// For colormapped or grayscale, data is written as-is (after optional demapping).
func (t *targaWriter) WriteScanline(line []byte) error {
	var out []byte

	switch t.info.ColorSpace {
	case ColorSpaceGrayscale:
		if t.info.QuantizeColors && t.info.Colormap != nil {
			// Demap grayscale quantized output
			out = make([]byte, len(line))
			map0 := t.info.Colormap.Maps[0]
			for i, idx := range line {
				out[i] = map0[idx]
			}
		} else {
			out = line
		}

	case ColorSpaceRGB, ColorSpaceYCbCr, ColorSpaceBigGamutYCbCr, ColorSpaceCMYK, ColorSpaceYCCK:
		if t.info.QuantizeColors {
			// Colormapped: indices are written directly (already handled by
			// the colormap in the header)
			out = line
		} else {
			// Convert RGB to BGR
			line = rgbScanline(line, t.info.Width, t.info.ColorSpace)
			for i := 0; i < t.info.Width; i++ {
				t.buf[i*3+0] = line[i*3+2] // B
				t.buf[i*3+1] = line[i*3+1] // G
				t.buf[i*3+2] = line[i*3+0] // R
			}
			out = t.buf
		}
	}

	if _, err := t.w.Write(out); err != nil {
		return fmt.Errorf("targa: writing scanline: %w", err)
	}
	return nil
}

// Finish flushes the output. TGA has no trailer.
func (t *targaWriter) Finish() error {
	if flusher, ok := t.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}
