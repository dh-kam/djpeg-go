package output

import (
	"encoding/binary"
	"fmt"
	"io"
)

// bmpWriter writes Windows BMP (bitmap) format.
//
// The implementation stores all scanlines in memory so it can write them
// in bottom-up order as required by the BMP specification. Rows are padded
// to 4-byte boundaries. Grayscale output uses an 8-bit palette.
type bmpWriter struct {
	w        io.Writer
	info     *ImageInfo
	os2      bool
	rows     [][]byte // buffered pixel rows (top-down order)
	rowIdx   int      // next row to fill
	rowWidth int      // bytes per row including padding
}

// Start initializes the writer. The actual BMP header is deferred until
// Finish because we need the complete image data to compute file size.
func (b *bmpWriter) Start(w io.Writer, info *ImageInfo) error {
	b.w = w
	b.info = info

	switch info.ColorSpace {
	case ColorSpaceGrayscale:
		b.rowWidth = (info.Width + 3) &^ 3 // 1 byte per pixel, pad to 4
	case ColorSpaceRGB:
		b.rowWidth = (info.Width*3 + 3) &^ 3 // 3 bytes per pixel, pad to 4
	default:
		return fmt.Errorf("bmp: unsupported color space %d", info.ColorSpace)
	}

	b.rows = make([][]byte, info.Height)
	for i := range b.rows {
		b.rows[i] = make([]byte, b.rowWidth) // zeroed => padding bytes are 0
	}
	b.rowIdx = 0
	return nil
}

// WriteScanline buffers one row of pixel data. For RGB input, the pixels
// are converted from RGB to BGR order as required by BMP.
func (b *bmpWriter) WriteScanline(line []byte) error {
	if b.rowIdx >= b.info.Height {
		return fmt.Errorf("bmp: too many scanlines")
	}

	row := b.rows[b.rowIdx]
	b.rowIdx++

	if b.info.ColorSpace == ColorSpaceGrayscale {
		// For grayscale, check if we're dealing with quantized data that needs demapping.
		if b.info.QuantizeColors && b.info.Colormap != nil {
			cm := b.info.Colormap
			map0 := cm.Maps[0]
			for i, idx := range line {
				row[i] = map0[idx]
			}
		} else {
			copy(row, line)
		}
	} else {
		// RGB -> BGR conversion, or colormapped (palette index) passthrough.
		if b.info.QuantizeColors && b.info.Colormap != nil {
			// Quantized color: indices map to RGB palette entries.
			cm := b.info.Colormap
			map0 := cm.Maps[0]
			map1 := cm.Maps[1]
			map2 := cm.Maps[2]
			for i, idx := range line {
				row[i*3+0] = map2[idx] // B
				row[i*3+1] = map1[idx] // G
				row[i*3+2] = map0[idx] // R
			}
		} else {
			// Full-color RGB: swap R and B channels for BGR.
			for i := 0; i < b.info.Width; i++ {
				row[i*3+0] = line[i*3+2] // B
				row[i*3+1] = line[i*3+1] // G
				row[i*3+2] = line[i*3+0] // R
			}
		}
	}
	return nil
}

// Finish writes the BMP file header, optional color table, and pixel data
// in bottom-up row order.
func (b *bmpWriter) Finish() error {
	bitsPerPixel, cmapEntries := b.headerLayout()

	colorEntrySize := 4
	dibHeaderSize := 40
	if b.os2 {
		colorEntrySize = 3
		dibHeaderSize = 12
	}
	headerSize := uint32(14 + dibHeaderSize + cmapEntries*colorEntrySize)
	imageSize := uint32(b.rowWidth) * uint32(b.info.Height)
	fileSize := headerSize + imageSize

	if err := b.writeFileHeader(fileSize, headerSize); err != nil {
		return err
	}
	if b.os2 {
		if err := b.writeOS2CoreHeader(bitsPerPixel); err != nil {
			return err
		}
	} else if err := b.writeWindowsInfoHeader(bitsPerPixel, cmapEntries); err != nil {
		return err
	}

	// --- Color table (if needed) ---
	if cmapEntries > 0 {
		if err := b.writeColormap(cmapEntries, colorEntrySize); err != nil {
			return err
		}
	}

	// --- Pixel data (bottom-up) ---
	for row := b.info.Height - 1; row >= 0; row-- {
		if _, err := b.w.Write(b.rows[row]); err != nil {
			return fmt.Errorf("bmp: writing pixel data: %w", err)
		}
	}

	if flusher, ok := b.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func (b *bmpWriter) headerLayout() (bitsPerPixel, cmapEntries int) {
	switch b.info.ColorSpace {
	case ColorSpaceGrayscale:
		return 8, 256
	case ColorSpaceRGB:
		if b.info.QuantizeColors {
			return 8, 256
		}
		return 24, 0
	default:
		return 0, 0
	}
}

func (b *bmpWriter) writeFileHeader(fileSize, pixelOffset uint32) error {
	fileHeader := make([]byte, 14)
	fileHeader[0] = 'B'
	fileHeader[1] = 'M'
	binary.LittleEndian.PutUint32(fileHeader[2:6], fileSize)
	// bfReserved1, bfReserved2 = 0
	binary.LittleEndian.PutUint32(fileHeader[10:14], pixelOffset)
	if _, err := b.w.Write(fileHeader); err != nil {
		return fmt.Errorf("bmp: writing file header: %w", err)
	}
	return nil
}

func (b *bmpWriter) writeWindowsInfoHeader(bitsPerPixel, cmapEntries int) error {
	infoHeader := make([]byte, 40)
	binary.LittleEndian.PutUint32(infoHeader[0:4], 40) // biSize
	binary.LittleEndian.PutUint32(infoHeader[4:8], uint32(b.info.Width))
	binary.LittleEndian.PutUint32(infoHeader[8:12], uint32(b.info.Height))
	binary.LittleEndian.PutUint16(infoHeader[12:14], 1) // biPlanes
	binary.LittleEndian.PutUint16(infoHeader[14:16], uint16(bitsPerPixel))
	// biCompression = 0 (BI_RGB)
	// biSizeImage = 0 (correct for uncompressed)
	if b.info.DensityUnit == 2 { // dots/cm -> dots/meter
		binary.LittleEndian.PutUint32(infoHeader[24:28], uint32(b.info.XDensity)*100)
		binary.LittleEndian.PutUint32(infoHeader[28:32], uint32(b.info.YDensity)*100)
	}
	binary.LittleEndian.PutUint32(infoHeader[32:36], uint32(cmapEntries))
	if _, err := b.w.Write(infoHeader); err != nil {
		return fmt.Errorf("bmp: writing info header: %w", err)
	}
	return nil
}

func (b *bmpWriter) writeOS2CoreHeader(bitsPerPixel int) error {
	coreHeader := make([]byte, 12)
	binary.LittleEndian.PutUint32(coreHeader[0:4], 12) // bcSize
	binary.LittleEndian.PutUint16(coreHeader[4:6], uint16(b.info.Width))
	binary.LittleEndian.PutUint16(coreHeader[6:8], uint16(b.info.Height))
	binary.LittleEndian.PutUint16(coreHeader[8:10], 1) // bcPlanes
	binary.LittleEndian.PutUint16(coreHeader[10:12], uint16(bitsPerPixel))
	if _, err := b.w.Write(coreHeader); err != nil {
		return fmt.Errorf("bmp: writing OS/2 core header: %w", err)
	}
	return nil
}

// writeColormap writes the BMP color table. Windows entries are BGR0; OS/2
// entries are BGR.
func (b *bmpWriter) writeColormap(cmapEntries, entrySize int) error {
	entry := make([]byte, entrySize)

	if b.info.QuantizeColors && b.info.Colormap != nil {
		cm := b.info.Colormap
		switch b.info.ColorSpace {
		case ColorSpaceGrayscale:
			// Grayscale palette from colormap
			map0 := cm.Maps[0]
			for i := 0; i < cmapEntries; i++ {
				var v byte
				if i < cm.NumColors {
					v = map0[i]
				} else if i < 256 {
					v = byte(i) // linear gray fill
				}
				entry[0] = v
				entry[1] = v
				entry[2] = v
				if _, err := b.w.Write(entry); err != nil {
					return fmt.Errorf("bmp: writing colormap: %w", err)
				}
			}
		case ColorSpaceRGB:
			map0 := cm.Maps[0]
			map1 := cm.Maps[1]
			map2 := cm.Maps[2]
			for i := 0; i < cmapEntries; i++ {
				if i < cm.NumColors {
					entry[0] = map2[i] // B
					entry[1] = map1[i] // G
					entry[2] = map0[i] // R
				} else {
					// Fill remaining entries with gray (128 = CENTERJSAMPLE)
					entry[0] = 128
					entry[1] = 128
					entry[2] = 128
				}
				if _, err := b.w.Write(entry); err != nil {
					return fmt.Errorf("bmp: writing colormap: %w", err)
				}
			}
		}
	} else {
		// Linear grayscale palette
		for i := 0; i < cmapEntries; i++ {
			v := byte(i)
			entry[0] = v
			entry[1] = v
			entry[2] = v
			if _, err := b.w.Write(entry); err != nil {
				return fmt.Errorf("bmp: writing colormap: %w", err)
			}
		}
	}
	return nil
}
