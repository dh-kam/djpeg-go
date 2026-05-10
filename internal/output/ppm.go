package output

import (
	"fmt"
	"io"
)

// ppmWriter writes PPM (color, P6) or PGM (grayscale, P5) binary format.
type ppmWriter struct {
	w    io.Writer
	info *ImageInfo
}

// Start writes the PPM/PGM header.
func (p *ppmWriter) Start(w io.Writer, info *ImageInfo) error {
	p.w = w
	p.info = info

	switch info.ColorSpace {
	case ColorSpaceGrayscale:
		// P5 binary PGM: "P5\nwidth height\nmaxval\n"
		header := fmt.Sprintf("P5\n%d %d\n255\n", info.Width, info.Height)
		if _, err := io.WriteString(w, header); err != nil {
			return fmt.Errorf("ppm: writing header: %w", err)
		}
	case ColorSpaceRGB, ColorSpaceYCbCr, ColorSpaceBigGamutYCbCr, ColorSpaceCMYK, ColorSpaceYCCK:
		// P6 binary PPM: "P6\nwidth height\nmaxval\n"
		header := fmt.Sprintf("P6\n%d %d\n255\n", info.Width, info.Height)
		if _, err := io.WriteString(w, header); err != nil {
			return fmt.Errorf("ppm: writing header: %w", err)
		}
	default:
		return fmt.Errorf("ppm: unsupported color space %d", info.ColorSpace)
	}
	return nil
}

// WriteScanline writes one row of raw pixel data.
// For grayscale, line has Width bytes.
// For RGB, line has Width*3 bytes in R,G,B order.
// If quantization is active with a colormap, the line contains colormap
// indices; we demap them to actual pixel values before writing.
func (p *ppmWriter) WriteScanline(line []byte) error {
	var out []byte

	if p.info.QuantizeColors && p.info.Colormap != nil {
		out = p.demap(line)
	} else {
		out = rgbScanline(line, p.info.Width, p.info.ColorSpace)
	}

	if _, err := p.w.Write(out); err != nil {
		return fmt.Errorf("ppm: writing scanline: %w", err)
	}
	return nil
}

// demap converts colormap index values to actual pixel values.
func (p *ppmWriter) demap(line []byte) []byte {
	cm := p.info.Colormap
	switch p.info.ColorSpace {
	case ColorSpaceGrayscale:
		out := make([]byte, len(line))
		map0 := cm.Maps[0]
		for i, idx := range line {
			out[i] = map0[idx]
		}
		return out
	case ColorSpaceRGB:
		out := make([]byte, len(line)*3)
		map0 := cm.Maps[0]
		map1 := cm.Maps[1]
		map2 := cm.Maps[2]
		for i, idx := range line {
			out[i*3+0] = map0[idx]
			out[i*3+1] = map1[idx]
			out[i*3+2] = map2[idx]
		}
		return out
	case ColorSpaceYCbCr, ColorSpaceBigGamutYCbCr, ColorSpaceCMYK, ColorSpaceYCCK:
		out := make([]byte, len(line)*3)
		for i, idx := range line {
			out[i*3+0], out[i*3+1], out[i*3+2] = colormapEntryToRGB(cm, idx, p.info.ColorSpace)
		}
		return out
	default:
		return line
	}
}

// Finish flushes the output. For PPM/PGM there is no trailer.
func (p *ppmWriter) Finish() error {
	if flusher, ok := p.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}
