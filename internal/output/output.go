// Package output implements image format writers for the djpeg decompressor.
//
// It provides a Writer interface and factory functions for producing output in
// PPM/PGM, BMP, GIF, Targa, and RLE formats. Each writer implements the
// Start/WriteScanline/Finish lifecycle pattern.
package output

import (
	"errors"
	"fmt"
	"io"
)

// ColorSpace represents the color space of the output image.
type ColorSpace int

const (
	ColorSpaceGrayscale ColorSpace = iota
	ColorSpaceRGB
)

// Colormap holds an optional palette for indexed-color output.
// Each entry in Maps is a slice of N uint8 values, one per channel.
// Maps has length 1 for grayscale or 3 for RGB.
type Colormap struct {
	Maps     [][]uint8
	NumColors int
}

// ImageInfo carries the metadata needed by output writers, mirroring
// the fields that the C code reads from j_decompress_ptr.
type ImageInfo struct {
	Width           int
	Height          int
	NumComponents   int        // 1 for grayscale, 3 for RGB
	ColorSpace      ColorSpace
	QuantizeColors  bool       // whether color quantization is active
	DesiredColors   int        // desired number of colors (0 = unlimited)
	Colormap        *Colormap  // colormap if QuantizeColors is true
	XDensity        int        // pixels per unit (used by BMP)
	YDensity        int
	DensityUnit     int        // 1=dpi, 2=dpcm
	DataPrecision   int        // bits per sample (8 or 12)
}

// Writer is the interface that all output format writers must implement.
// Callers invoke Start once, WriteScanline for each row (top to bottom),
// and Finish when all rows have been written.
type Writer interface {
	// Start writes any file header and prepares the writer to receive rows.
	// info describes the image dimensions and color space.
	Start(w io.Writer, info *ImageInfo) error

	// WriteScanline writes one row of pixel data.
	// For RGB images, line contains width*3 bytes in R,G,B order.
	// For grayscale images, line contains width bytes.
	// The caller owns the slice; the writer must not retain it.
	WriteScanline(line []byte) error

	// Finish writes any trailer, flushes, and completes the output.
	Finish() error
}

// Format identifies an output image format.
type Format string

const (
	FormatPPM   Format = "ppm"
	FormatBMP   Format = "bmp"
	FormatGIF   Format = "gif"
	FormatTarga Format = "targa"
	FormatRLE   Format = "rle"
)

// NewWriter creates a Writer for the requested output format.
func NewWriter(f Format) (Writer, error) {
	switch f {
	case FormatPPM:
		return &ppmWriter{}, nil
	case FormatBMP:
		return &bmpWriter{}, nil
	case FormatGIF:
		return &gifWriter{}, nil
	case FormatTarga:
		return &targaWriter{}, nil
	case FormatRLE:
		return &rleWriter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", f)
	}
}

// errUnsupported is returned when a feature is not yet implemented.
var errUnsupported = errors.New("unsupported feature")

// errWrite is a sentinel for I/O failures during output.
var errWrite = errors.New("write error")
