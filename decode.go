package djpeg

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"

	internaldecoder "github.com/dh-kam/djpeg-go/internal/decoder"
	"github.com/dh-kam/djpeg-go/internal/marker"
)

// Config describes decoded raster metadata.
type Config struct {
	Width           int
	Height          int
	Components      int
	Stride          int
	PixelFormat     PixelFormat
	ColorSpace      ColorSpace
	InputComponents int
	InputColorSpace ColorSpace
}

// ImageConfig converts Config to the standard image.Config type.
func (c Config) ImageConfig() image.Config {
	return image.Config{
		ColorModel: c.ColorModel(),
		Width:      c.Width,
		Height:     c.Height,
	}
}

// ColorModel returns the standard color model for the decoded pixels.
func (c Config) ColorModel() color.Model {
	switch c.PixelFormat {
	case PixelFormatGray8:
		return color.GrayModel
	default:
		return color.RGBAModel
	}
}

// Decode decodes a JPEG into an image.Image.
func Decode(r io.Reader, opts ...Option) (image.Image, error) {
	options := collectOptions(opts)
	return DecodeWithOptions(r, &options)
}

// DecodeWithOptions decodes a JPEG into an image.Image using an Options value.
func DecodeWithOptions(r io.Reader, opts *Options) (image.Image, error) {
	raster, err := DecodeRasterWithOptions(r, opts)
	if err != nil {
		return nil, err
	}
	return raster, nil
}

// DecodeConfig reads JPEG metadata and returns the standard image.Config.
func DecodeConfig(r io.Reader, opts ...Option) (image.Config, error) {
	options := collectOptions(opts)
	cfg, err := DecodeRasterConfigWithOptions(r, &options)
	if err != nil {
		return image.Config{}, err
	}
	return cfg.ImageConfig(), nil
}

// DecodeRaster decodes a JPEG into a raw Gray8 or RGB24 raster.
func DecodeRaster(r io.Reader, opts ...Option) (*Raster, error) {
	options := collectOptions(opts)
	return DecodeRasterWithOptions(r, &options)
}

// DecodeRasterWithOptions decodes a JPEG into a raw raster using an Options
// value.
func DecodeRasterWithOptions(r io.Reader, opts *Options) (*Raster, error) {
	dec := newDecoderWithOptions(r, optionsFromPointer(opts))
	if _, err := dec.ReadHeader(); err != nil {
		return nil, err
	}
	if err := dec.Start(); err != nil {
		return nil, err
	}

	cfg := dec.OutputConfig()
	raster := NewRaster(cfg.Width, cfg.Height, cfg.PixelFormat)
	if raster.Format == PixelFormatUnknown {
		return nil, fmt.Errorf("%w: unsupported output pixel format %s", ErrUnsupported, cfg.PixelFormat)
	}

	for y := 0; y < cfg.Height; y++ {
		row := raster.Pix[y*raster.Stride : y*raster.Stride+raster.Stride]
		n, err := dec.ReadScanlines([][]byte{row})
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
	}
	if err := dec.Finish(); err != nil {
		return nil, err
	}
	return raster, nil
}

// DecodeRasterConfig reads JPEG metadata and returns raster metadata.
func DecodeRasterConfig(r io.Reader, opts ...Option) (Config, error) {
	options := collectOptions(opts)
	return DecodeRasterConfigWithOptions(r, &options)
}

// DecodeRasterConfigWithOptions reads JPEG metadata using an Options value.
func DecodeRasterConfigWithOptions(r io.Reader, opts *Options) (Config, error) {
	dec := newDecoderWithOptions(r, optionsFromPointer(opts))
	return dec.ReadHeader()
}

// Decoder exposes a low-level scanline-oriented public API without leaking
// internal decoder types.
type Decoder struct {
	dec    *internaldecoder.Decoder
	opts   Options
	header Config
	output Config
}

// NewDecoder creates a decoder for r.
func NewDecoder(r io.Reader, opts ...Option) *Decoder {
	return newDecoderWithOptions(r, collectOptions(opts))
}

func newDecoderWithOptions(r io.Reader, opts Options) *Decoder {
	return &Decoder{
		dec:  internaldecoder.New(r),
		opts: opts,
	}
}

// ReadHeader reads JPEG metadata.
func (d *Decoder) ReadHeader() (Config, error) {
	if err := d.applyOptions(); err != nil {
		return Config{}, err
	}
	width, height, inputComponents, inputCS, err := d.dec.ReadHeader()
	if err != nil {
		return Config{}, wrapDecodeError("read header", err)
	}
	cfg := configFromHeader(width, height, inputComponents, inputCS)
	d.header = cfg
	return cfg, nil
}

// Start starts decompression and prepares output scanlines.
func (d *Decoder) Start() error {
	if d.header.Width == 0 || d.header.Height == 0 {
		if _, err := d.ReadHeader(); err != nil {
			return err
		}
	}
	if err := d.applyOptions(); err != nil {
		return err
	}
	if err := d.dec.StartDecompress(); err != nil {
		return wrapDecodeError("start decompress", err)
	}
	d.output = configFromOutput(d.dec)
	d.output.InputComponents = d.header.InputComponents
	if d.output.InputColorSpace == ColorSpaceUnknown {
		d.output.InputColorSpace = d.header.InputColorSpace
	}
	return nil
}

// ReadScanlines reads decoded scanlines into caller-provided row buffers.
func (d *Decoder) ReadScanlines(rows [][]byte) (int, error) {
	cfg := d.OutputConfig()
	if cfg.Stride > 0 {
		for i, row := range rows {
			if len(row) < cfg.Stride {
				return 0, fmt.Errorf("%w: scanline %d has %d bytes, want at least %d", ErrInvalidOption, i, len(row), cfg.Stride)
			}
		}
	}
	n, err := d.dec.ReadScanlines(rows)
	if err != nil {
		return n, wrapDecodeError("read scanlines", err)
	}
	return n, nil
}

// Finish completes decompression.
func (d *Decoder) Finish() error {
	if err := d.dec.FinishDecompress(); err != nil {
		return wrapDecodeError("finish decompress", err)
	}
	return nil
}

// OutputConfig returns output metadata after Start. Before Start, it returns
// the header-derived configuration.
func (d *Decoder) OutputConfig() Config {
	if d.output.Width != 0 || d.output.Height != 0 {
		return d.output
	}
	return d.header
}

func (d *Decoder) applyOptions() error {
	idct, err := d.opts.IDCT.decoderName()
	if err != nil {
		return err
	}
	fancy, err := d.opts.Upsampling.fancy()
	if err != nil {
		return err
	}
	chromaIDCTScaling, err := d.opts.Compatibility.chromaIDCTScaling()
	if err != nil {
		return err
	}
	inputColorSpace, err := d.opts.InputColorSpace.decoderName()
	if err != nil {
		return err
	}

	d.dec.SetIDCTMethod(idct)
	d.dec.SetFancyUpsampling(fancy)
	d.dec.SetChromaIDCTScaling(chromaIDCTScaling)
	if err := d.dec.SetInputColorSpace(inputColorSpace); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOption, err)
	}
	return nil
}

func configFromHeader(width, height, inputComponents int, inputCS marker.ColorSpace) Config {
	inputColorSpace := colorSpaceFromMarker(inputCS)
	pixelFormat := PixelFormatRGB24
	outputColorSpace := ColorSpaceRGB
	if inputComponents == 1 || inputCS == marker.CSGrayScale {
		pixelFormat = PixelFormatGray8
		outputColorSpace = ColorSpaceGray
	}
	components := pixelFormat.Channels()
	return Config{
		Width:           width,
		Height:          height,
		Components:      components,
		Stride:          width * components,
		PixelFormat:     pixelFormat,
		ColorSpace:      outputColorSpace,
		InputComponents: inputComponents,
		InputColorSpace: inputColorSpace,
	}
}

func configFromOutput(dec *internaldecoder.Decoder) Config {
	pixelFormat := PixelFormatRGB24
	colorSpace := colorSpaceFromMarker(dec.OutColorSpace())
	if dec.OutputComponents() == 1 || dec.OutColorSpace() == marker.CSGrayScale {
		pixelFormat = PixelFormatGray8
		colorSpace = ColorSpaceGray
	}
	components := pixelFormat.Channels()
	return Config{
		Width:           dec.OutputWidth(),
		Height:          dec.OutputHeight(),
		Components:      components,
		Stride:          dec.OutputWidth() * components,
		PixelFormat:     pixelFormat,
		ColorSpace:      colorSpace,
		InputColorSpace: colorSpaceFromMarker(dec.JPEGColorSpace()),
	}
}

func colorSpaceFromMarker(cs marker.ColorSpace) ColorSpace {
	switch cs {
	case marker.CSGrayScale:
		return ColorSpaceGray
	case marker.CSRGB:
		return ColorSpaceRGB
	case marker.CSYCbCr:
		return ColorSpaceYCbCr
	case marker.CSCMYK:
		return ColorSpaceCMYK
	case marker.CSYCCK:
		return ColorSpaceYCCK
	case marker.CSBGRGB:
		return ColorSpaceBigGamutRGB
	case marker.CSBGYCC:
		return ColorSpaceBigGamutYCbCr
	default:
		return ColorSpaceUnknown
	}
}

func wrapDecodeError(stage string, err error) error {
	if err == nil {
		return nil
	}
	if isUnsupportedError(err) {
		return fmt.Errorf("%s: %w: %v", stage, ErrUnsupported, err)
	}
	if isInvalidJPEGError(err) {
		return fmt.Errorf("%s: %w: %v", stage, ErrInvalidJPEG, err)
	}
	return fmt.Errorf("%s: %w", stage, err)
}

func isUnsupportedError(err error) bool {
	if errors.Is(err, internaldecoder.ErrUnsupportedJPEG) ||
		errors.Is(err, marker.ErrSOFUnsupported) ||
		errors.Is(err, marker.ErrBadPrecision) ||
		errors.Is(err, marker.ErrConversionNotImpl) ||
		errors.Is(err, marker.ErrNotImpl) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unsupported") ||
		strings.Contains(msg, "progressive") ||
		strings.Contains(msg, "arithmetic") ||
		strings.Contains(msg, "not yet supported")
}

func isInvalidJPEGError(err error) bool {
	return errors.Is(err, marker.ErrNoSOI) ||
		errors.Is(err, marker.ErrNoImage) ||
		errors.Is(err, marker.ErrBadLength) ||
		errors.Is(err, marker.ErrSOFNoSOS) ||
		errors.Is(err, marker.ErrEmptyImage)
}
