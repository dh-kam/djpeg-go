package output

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadColorMap reads a color map from an external file (GIF or PPM format).
// This is used by the "-map" option to specify a target palette for
// color quantization. It returns a Colormap with the unique colors found.
//
// The file format is auto-detected from the first byte:
//   - 'G': GIF file (global colormap is extracted)
//   - 'P': PPM file (unique pixels are collected)
func ReadColorMap(r io.Reader) (*Colormap, error) {
	br := bufio.NewReader(r)

	// Peek at the first byte to determine format
	firstByte, err := br.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading color map: %w", err)
	}

	switch firstByte {
	case 'G':
		return readGIFColormap(br)
	case 'P':
		return readPPMColormap(br)
	default:
		return nil, fmt.Errorf("unrecognized color map file format (expected GIF or PPM)")
	}
}

// readGIFColormap extracts the global colormap from a GIF file.
func readGIFColormap(br *bufio.Reader) (*Colormap, error) {
	// Read rest of GIF header: "IF87a" or "IF89a"
	header := make([]byte, 5)
	if _, err := io.ReadFull(br, header); err != nil {
		return nil, fmt.Errorf("reading GIF header: %w", err)
	}
	if header[0] != 'I' || header[1] != 'F' {
		return nil, fmt.Errorf("invalid GIF header")
	}

	// Read Logical Screen Descriptor (7 bytes)
	lsd := make([]byte, 7)
	if _, err := io.ReadFull(br, lsd); err != nil {
		return nil, fmt.Errorf("reading GIF screen descriptor: %w", err)
	}

	// Check for global color table
	hasGCT := lsd[4]&0x80 != 0
	if !hasGCT {
		return nil, fmt.Errorf("GIF has no global color table")
	}

	gctSize := 2 << (lsd[4] & 0x07)
	mapR := make([]uint8, gctSize)
	mapG := make([]uint8, gctSize)
	mapB := make([]uint8, gctSize)

	for i := 0; i < gctSize; i++ {
		rgb := make([]byte, 3)
		if _, err := io.ReadFull(br, rgb); err != nil {
			return nil, fmt.Errorf("reading GIF colormap entry %d: %w", i, err)
		}
		mapR[i] = rgb[0]
		mapG[i] = rgb[1]
		mapB[i] = rgb[2]
	}

	return &Colormap{
		Maps:      [][]uint8{mapR, mapG, mapB},
		NumColors: gctSize,
	}, nil
}

// readPPMColormap reads a PPM file and collects all unique pixel colors.
// Note: reading a large PPM file will be slow. Typically the map file
// should contain just one pixel of each desired color.
func readPPMColormap(br *bufio.Reader) (*Colormap, error) {
	// Read the format character after 'P'
	fmtChar, err := br.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading PPM format: %w", err)
	}

	w, err := readPBMInteger(br)
	if err != nil {
		return nil, err
	}
	h, err := readPBMInteger(br)
	if err != nil {
		return nil, err
	}
	maxval, err := readPBMInteger(br)
	if err != nil {
		return nil, err
	}

	if w <= 0 || h <= 0 || maxval <= 0 {
		return nil, fmt.Errorf("invalid PPM dimensions or maxval")
	}
	if maxval != 255 {
		return nil, fmt.Errorf("PPM maxval %d not supported (only 255)", maxval)
	}

	if err := skipPBMWhitespaceAndComments(br); err != nil {
		return nil, err
	}

	// Collect unique colors
	colorSet := make(map[[3]uint8]bool)
	var mapR, mapG, mapB []uint8

	switch fmtChar {
	case '3': // text PPM
		scanner := bufio.NewScanner(br)
		scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
			return bufio.ScanWords(data, atEOF)
		})
		for i := 0; i < w*h; i++ {
			r := scanPPMValue(scanner)
			g := scanPPMValue(scanner)
			b := scanPPMValue(scanner)
			if r < 0 || g < 0 || b < 0 {
				return nil, fmt.Errorf("invalid PPM text data")
			}
			key := [3]uint8{uint8(r), uint8(g), uint8(b)}
			if !colorSet[key] {
				colorSet[key] = true
				mapR = append(mapR, key[0])
				mapG = append(mapG, key[1])
				mapB = append(mapB, key[2])
			}
		}

	case '6': // raw PPM
		for i := 0; i < w*h; i++ {
			rgb := make([]byte, 3)
			if _, err := io.ReadFull(br, rgb); err != nil {
				return nil, fmt.Errorf("reading PPM pixel data: %w", err)
			}
			key := [3]uint8{rgb[0], rgb[1], rgb[2]}
			if !colorSet[key] {
				colorSet[key] = true
				mapR = append(mapR, key[0])
				mapG = append(mapG, key[1])
				mapB = append(mapB, key[2])
			}
		}

	default:
		return nil, fmt.Errorf("unsupported PPM format P%c", fmtChar)
	}

	return &Colormap{
		Maps:      [][]uint8{mapR, mapG, mapB},
		NumColors: len(mapR),
	}, nil
}

// readPBMInteger reads an unsigned decimal integer from a PBM/PPM stream,
// skipping comments and whitespace.
func readPBMInteger(br *bufio.Reader) (int, error) {
	// Skip whitespace and comments
	for {
		b, err := br.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == '#' {
			// Skip comment line
			line, _ := br.ReadString('\n')
			_ = line
			continue
		}
		if b >= '0' && b <= '9' {
			br.UnreadByte()
			break
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return 0, fmt.Errorf("unexpected character %q in PPM header", b)
	}

	// Read digits
	var digits strings.Builder
	for {
		b, err := br.ReadByte()
		if err != nil {
			break
		}
		if b >= '0' && b <= '9' {
			digits.WriteByte(b)
		} else {
			br.UnreadByte()
			break
		}
	}

	if digits.Len() == 0 {
		return 0, fmt.Errorf("expected integer in PPM header")
	}
	return strconv.Atoi(digits.String())
}

func skipPBMWhitespaceAndComments(br *bufio.Reader) error {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return err
		}
		if b == '#' {
			if _, err := br.ReadString('\n'); err != nil && err != io.EOF {
				return err
			}
			continue
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return br.UnreadByte()
	}
}

// scanPPMValue reads one integer value from a text PPM scanner.
func scanPPMValue(scanner *bufio.Scanner) int {
	if scanner.Scan() {
		v, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil {
			return -1
		}
		return v
	}
	return -1
}
