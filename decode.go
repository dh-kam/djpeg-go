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
	Width            int
	Height           int
	Components       int
	Stride           int
	PixelFormat      PixelFormat
	ColorSpace       ColorSpace
	InputComponents  int
	InputColorSpace  ColorSpace
	Baseline         bool
	Progressive      bool
	Arithmetic       bool
	HasMultipleScans bool
	InputComplete    bool
	SawJFIFMarker    bool
	JFIFMajorVersion uint8
	JFIFMinorVersion uint8
	DensityUnit      uint8
	XDensity         uint16
	YDensity         uint16
	SawAdobeMarker   bool
	AdobeTransform   uint8
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
	case PixelFormatCMYK32:
		return color.CMYKModel
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
	dec           *internaldecoder.Decoder
	opts          Options
	header        Config
	output        Config
	scaled        *Raster
	scaledNextRow int
	internalDone  bool
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
	if err := d.applyOutputColorSpaceToConfig(&cfg); err != nil {
		return Config{}, err
	}
	if err := d.applyScaleToConfig(&cfg); err != nil {
		return Config{}, err
	}
	d.populateHeaderMetadata(&cfg)
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
	baseOutput := configFromOutput(d.dec)
	baseOutput.InputComponents = d.header.InputComponents
	if baseOutput.InputColorSpace == ColorSpaceUnknown {
		baseOutput.InputColorSpace = d.header.InputColorSpace
	}
	d.populateHeaderMetadata(&baseOutput)
	d.output = baseOutput
	if err := d.applyScaleToConfig(&d.output); err != nil {
		return err
	}
	if d.output.Width != baseOutput.Width || d.output.Height != baseOutput.Height {
		scaled, err := d.readAndScaleStartedDecoder(baseOutput, d.output.Width, d.output.Height)
		if err != nil {
			return err
		}
		d.scaled = scaled
		d.scaledNextRow = 0
	}
	return nil
}

// StartDecompress starts decompression using libjpeg-style naming.
func (d *Decoder) StartDecompress() error {
	return d.Start()
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
	if d.scaled != nil {
		rowsRead := 0
		for rowsRead < len(rows) && d.scaledNextRow < d.scaled.Rect.Dy() {
			src := d.scaled.Pix[d.scaledNextRow*d.scaled.Stride : d.scaledNextRow*d.scaled.Stride+d.scaled.Stride]
			copy(rows[rowsRead], src)
			rowsRead++
			d.scaledNextRow++
		}
		return rowsRead, nil
	}
	n, err := d.dec.ReadScanlines(rows)
	if err != nil {
		return n, wrapDecodeError("read scanlines", err)
	}
	return n, nil
}

// SkipScanlines skips output scanlines using libjpeg-style naming.
func (d *Decoder) SkipScanlines(numLines int) (int, error) {
	if numLines < 0 {
		return 0, fmt.Errorf("%w: scanline count must be non-negative", ErrInvalidOption)
	}
	if numLines == 0 {
		return 0, nil
	}
	if d.scaled != nil {
		remaining := d.scaled.Rect.Dy() - d.scaledNextRow
		if numLines > remaining {
			numLines = remaining
		}
		d.scaledNextRow += numLines
		return numLines, nil
	}
	n, err := d.dec.SkipScanlines(numLines)
	if err != nil {
		return n, wrapDecodeError("skip scanlines", err)
	}
	return n, nil
}

// Finish completes decompression.
func (d *Decoder) Finish() error {
	if d.internalDone {
		return nil
	}
	if err := d.dec.FinishDecompress(); err != nil {
		return wrapDecodeError("finish decompress", err)
	}
	d.internalDone = true
	return nil
}

// FinishDecompress completes decompression using libjpeg-style naming.
func (d *Decoder) FinishDecompress() error {
	return d.Finish()
}

// Abort stops the current decompression operation and clears decoder-owned
// output state. It mirrors libjpeg's jpeg_abort_decompress behavior at the
// public facade level; the caller must provide a new reader to decode again.
func (d *Decoder) Abort() {
	d.dec.Abort()
	d.header = Config{}
	d.output = Config{}
	d.scaled = nil
	d.scaledNextRow = 0
	d.internalDone = false
}

// Markers returns APPn and COM markers retained while reading the JPEG header.
// Configure retained marker types with WithSavedMarkers before ReadHeader.
func (d *Decoder) Markers() []Marker {
	internal := d.dec.SavedMarkers()
	out := make([]Marker, len(internal))
	for i, m := range internal {
		data := make([]byte, len(m.Data))
		copy(data, m.Data)
		out[i] = Marker{
			Code:           m.Code,
			OriginalLength: m.OriginalLength,
			Data:           data,
		}
	}
	return out
}

// OutputConfig returns output metadata after Start. Before Start, it returns
// the header-derived configuration.
func (d *Decoder) OutputConfig() Config {
	if d.output.Width != 0 || d.output.Height != 0 {
		return d.output
	}
	return d.header
}

// Header returns the header-derived configuration from the most recent
// ReadHeader call.
func (d *Decoder) Header() Config {
	return d.header
}

// OutputScanline returns the next output scanline index.
func (d *Decoder) OutputScanline() int {
	if d.scaled != nil {
		return d.scaledNextRow
	}
	return d.dec.OutputScanline()
}

// InputComplete reports whether the JPEG input has been fully consumed.
func (d *Decoder) InputComplete() bool {
	return d.dec.InputComplete()
}

// HasMultipleScans reports whether the JPEG header indicates multiple scans.
func (d *Decoder) HasMultipleScans() bool {
	return d.dec.HasMultipleScans()
}

// IsBaseline reports whether the JPEG uses baseline DCT coding.
func (d *Decoder) IsBaseline() bool {
	return d.dec.IsBaseline()
}

// IsProgressive reports whether the JPEG uses progressive coding.
func (d *Decoder) IsProgressive() bool {
	return d.dec.IsProgressive()
}

// IsArithmetic reports whether the JPEG uses arithmetic entropy coding.
func (d *Decoder) IsArithmetic() bool {
	return d.dec.IsArithmetic()
}

func (d *Decoder) applyOptions() error {
	if d.opts.MaxMemoryBytes < 0 {
		return fmt.Errorf("%w: max memory must be non-negative", ErrInvalidOption)
	}
	for _, saved := range d.opts.SavedMarkers {
		if !validSavedMarkerCode(saved.Code) {
			return fmt.Errorf("%w: marker code 0x%02x cannot be saved", ErrInvalidOption, saved.Code)
		}
		if err := d.dec.SaveMarkers(saved.Code, saved.LengthLimit); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidOption, err)
		}
	}
	idct, err := d.opts.IDCT.decoderName()
	if err != nil {
		return err
	}
	fancy, err := d.opts.Upsampling.fancy()
	if err != nil {
		return err
	}
	chromaIDCTScaling, err := d.opts.effectiveChromaIDCTScaling()
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
	colorTransform, err := d.opts.ColorTransform.decoderName()
	if err != nil {
		return err
	}
	if err := d.dec.SetColorTransform(colorTransform); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOption, err)
	}
	outputColorSpace, err := d.opts.OutputColorSpace.outputDecoderName()
	if err != nil {
		return err
	}
	if err := d.dec.SetOutputColorSpace(outputColorSpace); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOption, err)
	}
	if err := d.dec.SetMaxMemory(d.opts.MaxMemoryBytes); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOption, err)
	}
	return nil
}

func (d *Decoder) readAndScaleStartedDecoder(base Config, width, height int) (*Raster, error) {
	raster := NewRaster(base.Width, base.Height, base.PixelFormat)
	if raster.Format == PixelFormatUnknown {
		return nil, fmt.Errorf("%w: unsupported output pixel format %s", ErrUnsupported, base.PixelFormat)
	}
	for y := 0; y < base.Height; y++ {
		row := raster.Pix[y*raster.Stride : y*raster.Stride+raster.Stride]
		n, err := d.dec.ReadScanlines([][]byte{row})
		if err != nil {
			return nil, wrapDecodeError("read scanlines", err)
		}
		if n == 0 {
			break
		}
	}
	if err := d.dec.FinishDecompress(); err != nil {
		return nil, wrapDecodeError("finish decompress", err)
	}
	d.internalDone = true
	scaled, err := scaleRasterNearest(raster, width, height)
	if err != nil {
		return nil, err
	}
	return scaled, nil
}

func (d *Decoder) applyOutputColorSpaceToConfig(cfg *Config) error {
	switch d.opts.OutputColorSpace {
	case ColorSpaceUnknown:
		return nil
	case ColorSpaceGray:
		cfg.PixelFormat = PixelFormatGray8
		cfg.ColorSpace = ColorSpaceGray
	case ColorSpaceRGB:
		cfg.PixelFormat = PixelFormatRGB24
		cfg.ColorSpace = ColorSpaceRGB
	case ColorSpaceCMYK:
		cfg.PixelFormat = PixelFormatCMYK32
		cfg.ColorSpace = ColorSpaceCMYK
	case ColorSpaceYCCK:
		cfg.PixelFormat = PixelFormatYCCK32
		cfg.ColorSpace = ColorSpaceYCCK
	default:
		return fmt.Errorf("%w: output color space %s is not supported", ErrUnsupported, d.opts.OutputColorSpace)
	}
	cfg.Components = cfg.PixelFormat.Channels()
	cfg.Stride = cfg.Width * cfg.Components
	return nil
}

func (d *Decoder) applyScaleToConfig(cfg *Config) error {
	scale, enabled, err := d.opts.scaleSize()
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	width, height, err := scaledDimensions(cfg.Width, cfg.Height, scale)
	if err != nil {
		return err
	}
	cfg.Width = width
	cfg.Height = height
	cfg.Stride = width * cfg.Components
	return nil
}

func (d *Decoder) populateHeaderMetadata(cfg *Config) {
	cfg.Baseline = d.dec.IsBaseline()
	cfg.Progressive = d.dec.IsProgressive()
	cfg.Arithmetic = d.dec.IsArithmetic()
	cfg.HasMultipleScans = d.dec.HasMultipleScans()
	cfg.InputComplete = d.dec.InputComplete()
	sawJFIF, major, minor, densityUnit, xDensity, yDensity := d.dec.JFIFInfo()
	cfg.SawJFIFMarker = sawJFIF
	cfg.JFIFMajorVersion = major
	cfg.JFIFMinorVersion = minor
	cfg.DensityUnit = densityUnit
	cfg.XDensity = xDensity
	cfg.YDensity = yDensity
	sawAdobe, transform := d.dec.AdobeInfo()
	cfg.SawAdobeMarker = sawAdobe
	cfg.AdobeTransform = transform
}

func configFromHeader(width, height, inputComponents int, inputCS marker.ColorSpace) Config {
	inputColorSpace := colorSpaceFromMarker(inputCS)
	pixelFormat := PixelFormatRGB24
	outputColorSpace := ColorSpaceRGB
	if inputComponents == 1 || inputCS == marker.CSGrayScale {
		pixelFormat = PixelFormatGray8
		outputColorSpace = ColorSpaceGray
	} else if inputComponents == 4 || inputCS == marker.CSCMYK || inputCS == marker.CSYCCK {
		pixelFormat = PixelFormatCMYK32
		outputColorSpace = ColorSpaceCMYK
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
	} else if dec.OutputComponents() == 4 || dec.OutColorSpace() == marker.CSCMYK || dec.OutColorSpace() == marker.CSYCCK {
		if dec.OutColorSpace() == marker.CSYCCK {
			pixelFormat = PixelFormatYCCK32
			colorSpace = ColorSpaceYCCK
		} else {
			pixelFormat = PixelFormatCMYK32
			colorSpace = ColorSpaceCMYK
		}
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
	if errors.Is(err, internaldecoder.ErrMemoryLimitExceeded) {
		return fmt.Errorf("%s: %w: %v", stage, ErrMemoryLimit, err)
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
