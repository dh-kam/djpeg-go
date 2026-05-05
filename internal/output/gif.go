package output

import (
	"fmt"
	"io"
)

// gifWriter writes GIF89a format with LZW compression.
//
// GIF requires color quantization to at most 256 colors. If the image
// has more than 256 colors and no quantization is applied, the writer
// returns an error. For grayscale images, a 256-level grayscale palette
// is generated automatically.
//
// The LZW compression implementation is a direct port of the algorithm
// in IJG's wrgif.c, using open-addressing double hashing.
const (
	maxLZWBits   = 12
	lzwTableSize = 1 << maxLZWBits // 4096
	hashSize     = 5003
)

type codeInt = int32 // must hold -1 .. 4096

type gifWriter struct {
	w    io.Writer
	info *ImageInfo

	// LZW compression state
	nBits    int
	maxcode  codeInt
	initBits int
	curAccum uint32
	curBits  int

	waitingCode codeInt
	firstByte   bool

	clearCode codeInt
	eofCode   codeInt
	freeCode  codeInt

	// Hash table for LZW
	hashCode  []codeInt
	hashValue []int32 // (prefix << 8) | suffix

	// GIF data packet buffer
	bytesInPkt int
	packetBuf  [256]byte

	// Buffered rows (GIF writes all at once in finish)
	rows   [][]byte
	rowIdx int
}

// Start validates that the image can be represented as GIF and initializes state.
func (g *gifWriter) Start(w io.Writer, info *ImageInfo) error {
	g.w = w
	g.info = info

	// GIF requires a color palette (indexed color). Grayscale is OK (1 channel).
	if info.ColorSpace != ColorSpaceGrayscale && info.ColorSpace != ColorSpaceRGB {
		return fmt.Errorf("gif: unsupported color space")
	}

	// If RGB without quantization, GIF cannot represent it directly.
	// For now we require quantization for RGB images.
	if info.ColorSpace == ColorSpaceRGB && !info.QuantizeColors {
		return fmt.Errorf("gif: RGB images require color quantization (use -colors N)")
	}

	// Allocate row buffer
	g.rows = make([][]byte, info.Height)
	for i := range g.rows {
		g.rows[i] = make([]byte, info.Width)
	}
	g.rowIdx = 0

	// Allocate LZW hash table
	g.hashCode = make([]codeInt, hashSize)
	g.hashValue = make([]int32, hashSize)

	return nil
}

// WriteScanline buffers one row of pixel data (palette indices).
func (g *gifWriter) WriteScanline(line []byte) error {
	if g.rowIdx >= g.info.Height {
		return fmt.Errorf("gif: too many scanlines")
	}
	copy(g.rows[g.rowIdx], line)
	g.rowIdx++
	return nil
}

// Finish writes the complete GIF file: header, color table, image data, trailer.
func (g *gifWriter) Finish() error {
	// Determine color count and palette
	numColors := 256
	if g.info.QuantizeColors && g.info.Colormap != nil && g.info.Colormap.NumColors > 0 {
		numColors = g.info.Colormap.NumColors
	}
	if numColors > 256 {
		numColors = 256
	}

	// Compute bits per pixel
	bitsPerPixel := 1
	for (1 << bitsPerPixel) < numColors {
		bitsPerPixel++
	}
	colorMapSize := 1 << bitsPerPixel
	initCodeSize := bitsPerPixel
	if initCodeSize < 2 {
		initCodeSize = 2
	}

	// --- GIF Header ---
	g.w.Write([]byte("GIF87a"))

	// Logical Screen Descriptor
	writeLE16(g.w, uint16(g.info.Width))
	writeLE16(g.w, uint16(g.info.Height))
	flagByte := byte(0x80) // global color table present
	flagByte |= byte((bitsPerPixel - 1) << 4) // color resolution
	flagByte |= byte(bitsPerPixel - 1)         // size of global color table
	g.w.Write([]byte{flagByte, 0, 0})          // flag, bg color, aspect ratio

	// Global Color Table
	for i := 0; i < colorMapSize; i++ {
		var r, gv, b byte
		if i < numColors {
			if g.info.QuantizeColors && g.info.Colormap != nil {
				cm := g.info.Colormap
				if g.info.ColorSpace == ColorSpaceRGB && len(cm.Maps) >= 3 {
					r = cm.Maps[0][i]
					gv = cm.Maps[1][i]
					b = cm.Maps[2][i]
				} else if len(cm.Maps) >= 1 {
					v := cm.Maps[0][i]
					r, gv, b = v, v, v
				}
			} else {
				// Grayscale palette
				v := byte((i * 255) / (numColors - 1))
				r, gv, b = v, v, v
			}
		} else {
			// Fill remaining entries with gray
			r, gv, b = 128, 128, 128
		}
		g.w.Write([]byte{r, gv, b})
	}

	// Image Descriptor
	g.w.Write([]byte{','})       // separator
	writeLE16(g.w, 0)            // left
	writeLE16(g.w, 0)            // top
	writeLE16(g.w, uint16(g.info.Width))
	writeLE16(g.w, uint16(g.info.Height))
	g.w.Write([]byte{0x00})      // not interlaced, no local color map

	// Initial code size byte
	g.w.Write([]byte{byte(initCodeSize)})

	// Initialize LZW compression and write image data
	g.compressInit(initCodeSize + 1)
	for row := 0; row < g.info.Height; row++ {
		for col := 0; col < g.info.Width; col++ {
			g.compressByte(g.rows[row][col])
		}
	}
	g.compressTerm()

	// Block terminator
	g.w.Write([]byte{0})

	// GIF trailer
	g.w.Write([]byte{';'})

	if flusher, ok := g.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

// compressInit sets up LZW state.
func (g *gifWriter) compressInit(iBits int) {
	g.nBits = iBits
	g.initBits = iBits
	g.maxcode = (1 << iBits) - 1
	g.clearCode = codeInt(1 << (iBits - 1))
	g.eofCode = g.clearCode + 1
	g.freeCode = g.clearCode + 2
	g.firstByte = true
	g.bytesInPkt = 0
	g.curAccum = 0
	g.curBits = 0

	// Clear hash table
	for i := range g.hashCode {
		g.hashCode[i] = 0
	}

	// Initial clear code
	g.output(g.clearCode)
}

// compressByte processes one pixel through the LZW compressor.
func (g *gifWriter) compressByte(c byte) {
	ci := codeInt(c)

	if g.firstByte {
		g.waitingCode = ci
		g.firstByte = false
		return
	}

	// Hash table lookup
	probeValue := int32(g.waitingCode)<<8 | int32(c)
	i := int(c)<<(maxLZWBits-8) + int(g.waitingCode)
	if i >= hashSize {
		i -= hashSize
	}

	if g.hashCode[i] == 0 {
		// Empty slot: output waiting code and add new entry
		g.output(g.waitingCode)
		if g.freeCode < lzwTableSize {
			g.hashCode[i] = g.freeCode
			g.hashValue[i] = probeValue
			g.freeCode++
		} else {
			g.clearBlock()
		}
		g.waitingCode = ci
		return
	}
	if g.hashValue[i] == probeValue {
		g.waitingCode = g.hashCode[i]
		return
	}

	// Secondary hash (Knott's algorithm)
	var disp int
	if i == 0 {
		disp = 1
	} else {
		disp = hashSize - i
	}
	for {
		i -= disp
		if i < 0 {
			i += hashSize
		}
		if g.hashCode[i] == 0 {
			g.output(g.waitingCode)
			if g.freeCode < lzwTableSize {
				g.hashCode[i] = g.freeCode
				g.hashValue[i] = probeValue
				g.freeCode++
			} else {
				g.clearBlock()
			}
			g.waitingCode = ci
			return
		}
		if g.hashValue[i] == probeValue {
			g.waitingCode = g.hashCode[i]
			return
		}
	}
}

// compressTerm flushes remaining LZW state.
func (g *gifWriter) compressTerm() {
	if !g.firstByte {
		g.output(g.waitingCode)
	}
	g.output(g.eofCode)
	if g.curBits > 0 {
		g.charOut(byte(g.curAccum & 0xFF))
	}
	g.flushPacket()
}

// clearBlock resets the compressor and emits a clear code.
func (g *gifWriter) clearBlock() {
	for i := range g.hashCode {
		g.hashCode[i] = 0
	}
	g.freeCode = g.clearCode + 2
	g.output(g.clearCode)
	g.nBits = g.initBits
	g.maxcode = (1 << g.nBits) - 1
}

// output emits an LZW code of nBits width.
func (g *gifWriter) output(code codeInt) {
	g.curAccum |= uint32(code) << g.curBits
	g.curBits += g.nBits

	for g.curBits >= 8 {
		g.charOut(byte(g.curAccum & 0xFF))
		g.curAccum >>= 8
		g.curBits -= 8
	}

	// Adjust code size if needed
	if g.freeCode > g.maxcode {
		g.nBits++
		if g.nBits == maxLZWBits {
			g.maxcode = lzwTableSize
		} else {
			g.maxcode = (1 << g.nBits) - 1
		}
	}
}

// charOut adds one byte to the GIF data packet.
func (g *gifWriter) charOut(c byte) {
	g.packetBuf[g.bytesInPkt+1] = c
	g.bytesInPkt++
	if g.bytesInPkt >= 255 {
		g.flushPacket()
	}
}

// flushPacket writes the current data packet to the output.
func (g *gifWriter) flushPacket() {
	if g.bytesInPkt > 0 {
		g.packetBuf[0] = byte(g.bytesInPkt)
		g.w.Write(g.packetBuf[:g.bytesInPkt+1])
		g.bytesInPkt = 0
	}
}

// writeLE16 writes a 16-bit value in little-endian order.
func writeLE16(w io.Writer, v uint16) {
	var buf [2]byte
	buf[0] = byte(v & 0xFF)
	buf[1] = byte(v >> 8)
	w.Write(buf[:])
}
