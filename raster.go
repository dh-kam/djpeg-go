package djpeg

import (
	"image"
	"image/color"
)

// PixelFormat describes the byte layout of Raster.Pix.
type PixelFormat int

const (
	PixelFormatUnknown PixelFormat = iota
	PixelFormatGray8
	PixelFormatRGB24
	PixelFormatCMYK32
	PixelFormatYCCK32
)

// Channels returns the number of bytes per pixel.
func (f PixelFormat) Channels() int {
	switch f {
	case PixelFormatGray8:
		return 1
	case PixelFormatRGB24:
		return 3
	case PixelFormatCMYK32, PixelFormatYCCK32:
		return 4
	default:
		return 0
	}
}

func (f PixelFormat) String() string {
	switch f {
	case PixelFormatGray8:
		return "gray8"
	case PixelFormatRGB24:
		return "rgb24"
	case PixelFormatCMYK32:
		return "cmyk32"
	case PixelFormatYCCK32:
		return "ycck32"
	default:
		return "unknown"
	}
}

// ColorSpace describes an input or output color space without exposing
// internal decoder enums.
type ColorSpace int

const (
	ColorSpaceUnknown ColorSpace = iota
	ColorSpaceGray
	ColorSpaceRGB
	ColorSpaceYCbCr
	ColorSpaceCMYK
	ColorSpaceYCCK
	ColorSpaceBigGamutRGB
	ColorSpaceBigGamutYCbCr
)

func (c ColorSpace) String() string {
	switch c {
	case ColorSpaceGray:
		return "gray"
	case ColorSpaceRGB:
		return "rgb"
	case ColorSpaceYCbCr:
		return "ycbcr"
	case ColorSpaceCMYK:
		return "cmyk"
	case ColorSpaceYCCK:
		return "ycck"
	case ColorSpaceBigGamutRGB:
		return "big-gamut-rgb"
	case ColorSpaceBigGamutYCbCr:
		return "big-gamut-ycbcr"
	default:
		return "unknown"
	}
}

// Raster is a decoded top-down pixel buffer.
//
// RGB pixels are laid out as R, G, B bytes. Grayscale pixels are one Y byte.
// Raster implements image.Image; At is intended for interoperability, while
// Pix is the efficient access path.
type Raster struct {
	Pix    []byte
	Stride int
	Rect   image.Rectangle
	Format PixelFormat
}

// NewRaster allocates a Raster for the requested dimensions and pixel format.
func NewRaster(width, height int, format PixelFormat) *Raster {
	channels := format.Channels()
	if width < 0 || height < 0 || channels == 0 {
		return &Raster{Format: PixelFormatUnknown}
	}
	stride := width * channels
	return &Raster{
		Pix:    make([]byte, height*stride),
		Stride: stride,
		Rect:   image.Rect(0, 0, width, height),
		Format: format,
	}
}

// Bounds implements image.Image.
func (r *Raster) Bounds() image.Rectangle {
	if r == nil {
		return image.Rectangle{}
	}
	return r.Rect
}

// ColorModel implements image.Image.
func (r *Raster) ColorModel() color.Model {
	if r == nil {
		return color.RGBAModel
	}
	switch r.Format {
	case PixelFormatGray8:
		return color.GrayModel
	case PixelFormatCMYK32:
		return color.CMYKModel
	default:
		return color.RGBAModel
	}
}

// At implements image.Image.
func (r *Raster) At(x, y int) color.Color {
	if r == nil || !image.Pt(x, y).In(r.Rect) {
		return color.RGBA{}
	}
	i := (y-r.Rect.Min.Y)*r.Stride + (x-r.Rect.Min.X)*r.Format.Channels()
	switch r.Format {
	case PixelFormatGray8:
		return color.Gray{Y: r.Pix[i]}
	case PixelFormatRGB24:
		return color.RGBA{R: r.Pix[i], G: r.Pix[i+1], B: r.Pix[i+2], A: 0xff}
	case PixelFormatCMYK32:
		return color.CMYK{C: r.Pix[i], M: r.Pix[i+1], Y: r.Pix[i+2], K: r.Pix[i+3]}
	case PixelFormatYCCK32:
		c, m, yy := yccToCMY(r.Pix[i], r.Pix[i+1], r.Pix[i+2])
		return color.CMYK{C: c, M: m, Y: yy, K: r.Pix[i+3]}
	default:
		return color.RGBA{}
	}
}

// RGBA converts the raster to a standard *image.RGBA.
func (r *Raster) RGBA() *image.RGBA {
	if r == nil {
		return image.NewRGBA(image.Rectangle{})
	}
	out := image.NewRGBA(r.Rect)
	width := r.Rect.Dx()
	height := r.Rect.Dy()
	switch r.Format {
	case PixelFormatGray8:
		for y := 0; y < height; y++ {
			src := r.Pix[y*r.Stride : y*r.Stride+width]
			dst := out.Pix[y*out.Stride : y*out.Stride+width*4]
			for x, v := range src {
				j := x * 4
				dst[j] = v
				dst[j+1] = v
				dst[j+2] = v
				dst[j+3] = 0xff
			}
		}
	case PixelFormatRGB24:
		for y := 0; y < height; y++ {
			src := r.Pix[y*r.Stride : y*r.Stride+width*3]
			dst := out.Pix[y*out.Stride : y*out.Stride+width*4]
			for x := 0; x < width; x++ {
				s := x * 3
				d := x * 4
				dst[d] = src[s]
				dst[d+1] = src[s+1]
				dst[d+2] = src[s+2]
				dst[d+3] = 0xff
			}
		}
	case PixelFormatCMYK32, PixelFormatYCCK32:
		for y := 0; y < height; y++ {
			src := r.Pix[y*r.Stride : y*r.Stride+width*4]
			dst := out.Pix[y*out.Stride : y*out.Stride+width*4]
			for x := 0; x < width; x++ {
				s := x * 4
				c := src[s]
				m := src[s+1]
				yy := src[s+2]
				if r.Format == PixelFormatYCCK32 {
					c, m, yy = yccToCMY(src[s], src[s+1], src[s+2])
				}
				red, green, blue, _ := color.CMYK{C: c, M: m, Y: yy, K: src[s+3]}.RGBA()
				d := x * 4
				dst[d] = byte(red >> 8)
				dst[d+1] = byte(green >> 8)
				dst[d+2] = byte(blue >> 8)
				dst[d+3] = 0xff
			}
		}
	}
	return out
}

func yccToCMY(y, cb, cr byte) (byte, byte, byte) {
	yy := int(y)
	cbb := int(cb)
	crr := int(cr)
	r := clampByte(yy + ((91881 * (crr - 128)) >> 16))
	g := clampByte(yy - ((22554*(cbb-128) + 46802*(crr-128)) >> 16))
	b := clampByte(yy + ((116130 * (cbb - 128)) >> 16))
	return 255 - r, 255 - g, 255 - b
}

func clampByte(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func scaleRasterNearest(src *Raster, width, height int) (*Raster, error) {
	if src == nil {
		return nil, nil
	}
	if width == src.Rect.Dx() && height == src.Rect.Dy() {
		return src, nil
	}
	dst := NewRaster(width, height, src.Format)
	if dst.Format == PixelFormatUnknown {
		return nil, ErrInvalidOption
	}
	srcWidth := src.Rect.Dx()
	srcHeight := src.Rect.Dy()
	channels := src.Format.Channels()
	for y := 0; y < height; y++ {
		srcY := y * srcHeight / height
		srcRow := src.Pix[srcY*src.Stride : srcY*src.Stride+srcWidth*channels]
		dstRow := dst.Pix[y*dst.Stride : y*dst.Stride+width*channels]
		for x := 0; x < width; x++ {
			srcX := x * srcWidth / width
			copy(dstRow[x*channels:(x+1)*channels], srcRow[srcX*channels:(srcX+1)*channels])
		}
	}
	return dst, nil
}
