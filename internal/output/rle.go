package output

import (
	"encoding/binary"
	"fmt"
	"io"
)

// rleWriter writes Utah Raster Toolkit RLE format.
//
// RLE stores scanlines bottom-to-top, so we buffer all rows and write them
// in reverse order during Finish. The format uses a simple run-length encoding.
//
// RLE file structure:
//
//	Header (magic "RLE\n", then binary fields)
//	Color map (optional)
//	Scanline data (bottom-up, run-length encoded)
//	EOF opcode
type rleWriter struct {
	w       io.Writer
	info    *ImageInfo
	rows    [][]byte // buffered rows (top-down)
	rowIdx  int
	indexed bool
}

const (
	rleMagic   = "rle\x95" // RLE magic number (0x95 = 149 decimal version)
	rleOpSkip  = 1         // skip lines
	rleOpColor = 2         // set color channel
	rleOpDump  = 3         // dump pixels
	rleOpRun   = 4         // run of same pixel
	rleOpEOF   = 5         // end of file
	rleNop     = 6         // no-op
	rleComment = 7         // comment
	rleBgColor = 8         // background color (optional)
)

// Start initializes the RLE writer and validates the image can be stored.
func (r *rleWriter) Start(w io.Writer, info *ImageInfo) error {
	r.w = w
	r.info = info

	if info.Width > 32767 || info.Height > 32767 {
		return fmt.Errorf("rle: image too large for RLE format (%dx%d)", info.Width, info.Height)
	}
	if info.ColorSpace != ColorSpaceGrayscale && info.ColorSpace != ColorSpaceRGB {
		return fmt.Errorf("rle: unsupported color space")
	}
	if info.NumComponents != 1 && info.NumComponents != 3 {
		return fmt.Errorf("rle: unsupported number of components (%d)", info.NumComponents)
	}

	rowBytes := info.Width * info.NumComponents
	r.indexed = info.QuantizeColors && info.Colormap != nil
	if r.indexed {
		rowBytes = info.Width
	}
	r.rows = make([][]byte, info.Height)
	for i := range r.rows {
		r.rows[i] = make([]byte, rowBytes)
	}
	r.rowIdx = 0
	return nil
}

// WriteScanline buffers one row of pixel data.
func (r *rleWriter) WriteScanline(line []byte) error {
	if r.rowIdx >= r.info.Height {
		return fmt.Errorf("rle: too many scanlines")
	}
	copy(r.rows[r.rowIdx], line)
	r.rowIdx++
	return nil
}

// Finish writes the complete RLE file in bottom-up order with run-length encoding.
func (r *rleWriter) Finish() error {
	ncolors := r.info.NumComponents
	ncmap := 0
	var cmapData []uint16

	// If we have a colormap from quantization, encode it
	if r.info.QuantizeColors && r.info.Colormap != nil {
		ncolors = 1
		cm := r.info.Colormap
		ncmap = len(cm.Maps)
		cmapLen := 256
		cmapData = make([]uint16, ncmap*cmapLen)
		for ci := 0; ci < ncmap; ci++ {
			for i := 0; i < cmapLen; i++ {
				var v byte
				if i < cm.NumColors {
					v = cm.Maps[ci][i]
				}
				cmapData[ci*cmapLen+i] = uint16(v) << 8 // RLE stores 16-bit color values
			}
		}
	}

	// Write RLE header
	// Magic: "rle" + version byte (0x95)
	if _, err := r.w.Write([]byte(rleMagic)); err != nil {
		return fmt.Errorf("rle: writing magic: %w", err)
	}

	// Background color (4 bytes: nchannels, then B,G,R or just V for grayscale)
	// We use 0 (no background color)
	writeLE16(r.w, 0) // ncolors of background (0 = no background)

	// Color map descriptor
	cmaplen := 0
	if ncmap > 0 {
		cmaplen = 8 // log2(256) = 8
	}
	writeLE16(r.w, uint16(ncmap))
	r.w.Write([]byte{byte(cmaplen)})

	// Write colormap if present
	if ncmap > 0 && cmapData != nil {
		for _, v := range cmapData {
			writeBE16(r.w, v)
		}
	}

	// Image dimensions
	writeLE16(r.w, uint16(0))               // xmin
	writeLE16(r.w, uint16(r.info.Width-1))  // xmax
	writeLE16(r.w, uint16(0))               // ymin
	writeLE16(r.w, uint16(r.info.Height-1)) // ymax

	// Number of color channels and flags
	r.w.Write([]byte{byte(ncolors)}) // ncolors

	// Pixel data: write rows bottom-up, run-length encoded
	for row := r.info.Height - 1; row >= 0; row-- {
		data := r.rows[row]

		// Set color channel 0
		writeRLEOpcode(r.w, rleOpColor, 0)

		// Encode the row using run-length encoding
		if r.indexed || ncolors == 1 {
			rleEncodeRow(r.w, data, r.info.Width)
		} else {
			// For multi-channel, interleave channels into RLE format.
			// RLE format writes each channel separately for each scanline.
			for ch := 0; ch < ncolors; ch++ {
				writeRLEOpcode(r.w, rleOpColor, ch)
				chData := make([]byte, r.info.Width)
				for i := 0; i < r.info.Width; i++ {
					chData[i] = data[i*ncolors+ch]
				}
				rleEncodeRow(r.w, chData, r.info.Width)
			}
		}

		// Skip to next line (advance to next scanline)
		writeRLEOpcode(r.w, rleOpSkip, 1)
	}

	// EOF opcode
	writeRLEOpcode(r.w, rleOpEOF, 0)

	if flusher, ok := r.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// rleEncodeRow writes one row of single-channel pixel data using RLE encoding.
func rleEncodeRow(w io.Writer, data []byte, width int) {
	col := 0
	for col < width {
		// Look for a run of identical bytes
		runLen := 1
		for col+runLen < width && data[col+runLen] == data[col] && runLen < 255 {
			runLen++
		}

		if runLen >= 4 {
			// Worth encoding as a run
			writeRLEOpcode(w, rleOpRun, runLen)
			w.Write([]byte{data[col]})
			col += runLen
		} else {
			// Encode as a dump of unique pixels
			dumpLen := runLen
			// Extend dump while run lengths are short
			for col+dumpLen < width && dumpLen < 255 {
				nextRun := 1
				for col+dumpLen+nextRun < width && data[col+dumpLen+nextRun] == data[col+dumpLen] && nextRun < 4 {
					nextRun++
				}
				if nextRun >= 4 {
					break
				}
				dumpLen++
			}
			writeRLEOpcode(w, rleOpDump, dumpLen)
			w.Write(data[col : col+dumpLen])
			col += dumpLen
		}
	}
}

// writeRLEOpcode writes a 2-byte RLE opcode + count.
func writeRLEOpcode(w io.Writer, opcode, count int) {
	var buf [2]byte
	buf[0] = byte(opcode)
	buf[1] = byte(count)
	w.Write(buf[:])
}

// writeBE16 writes a 16-bit value in big-endian order.
func writeBE16(w io.Writer, v uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	w.Write(buf[:])
}
