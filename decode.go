package djpeg

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"strings"

	internaldecoder "github.com/dh-kam/djpeg-go/internal/decoder"
	"github.com/dh-kam/djpeg-go/internal/marker"
	internaloutput "github.com/dh-kam/djpeg-go/internal/output"
)

// Config describes decoded raster metadata.
type Config struct {
	Width             int
	Height            int
	Components        int
	Stride            int
	PixelFormat       PixelFormat
	ColorSpace        ColorSpace
	ImageWidth        int
	ImageHeight       int
	InputComponents   int
	InputColorSpace   ColorSpace
	DataPrecision     int
	MaxHSampFactor    int
	MaxVSampFactor    int
	MinDCTHScaledSize int
	MinDCTVScaledSize int
	BlockSize         int
	ScaleNum          uint
	ScaleDenom        uint
	Baseline          bool
	Progressive       bool
	Arithmetic        bool
	HasMultipleScans  bool
	InputComplete     bool
	SawJFIFMarker     bool
	JFIFMajorVersion  uint8
	JFIFMinorVersion  uint8
	DensityUnit       uint8
	XDensity          uint16
	YDensity          uint16
	SawAdobeMarker    bool
	AdobeTransform    uint8
	RecOutbufHeight   int
	Quantized         bool
	DesiredNumColors  int
	ActualNumColors   int
	RawDataOut        bool
	BufferedImage     bool
	OutputGamma       float64
	DoBlockSmoothing  bool
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
	options := optionsFromPointer(opts)
	if options.RawDataOut {
		return nil, fmt.Errorf("%w: raw data output requires DecodeRawComponents or Decoder.ReadRawData", ErrInvalidOption)
	}
	dec := newDecoderWithOptions(r, options)
	if _, err := dec.ReadHeader(); err != nil {
		return nil, err
	}
	if err := dec.Start(); err != nil {
		return nil, err
	}

	cfg := dec.OutputConfig()
	raster := NewRaster(cfg.Width, cfg.Height, cfg.PixelFormat)
	if cfg.PixelFormat == PixelFormatIndexed8 {
		raster.Palette = dec.Palette()
	}
	if raster.Format == PixelFormatUnknown {
		return nil, fmt.Errorf("%w: unsupported output pixel format %s", ErrUnsupported, cfg.PixelFormat)
	}

	if options.BufferedImage {
		if ok, err := dec.StartOutput(dec.InputScanNumber()); err != nil {
			return nil, err
		} else if !ok {
			return nil, fmt.Errorf("%w: buffered image output suspended", ErrInvalidOption)
		}
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
	if options.BufferedImage {
		if ok, err := dec.FinishOutput(); err != nil {
			return nil, err
		} else if !ok {
			return nil, fmt.Errorf("%w: buffered image output suspended", ErrInvalidOption)
		}
	}
	if err := dec.Finish(); err != nil {
		return nil, err
	}
	return raster, nil
}

// DecodeRawComponents decodes JPEG samples into downsampled component planes
// using libjpeg-style raw_data_out mode.
func DecodeRawComponents(r io.Reader, opts ...Option) ([]RawComponent, Config, error) {
	options := collectOptions(opts)
	options.RawDataOut = true
	return DecodeRawComponentsWithOptions(r, &options)
}

// DecodeRawComponentsWithOptions decodes JPEG samples into downsampled
// component planes using an Options value.
func DecodeRawComponentsWithOptions(r io.Reader, opts *Options) ([]RawComponent, Config, error) {
	options := optionsFromPointer(opts)
	options.RawDataOut = true
	dec := newDecoderWithOptions(r, options)
	if _, err := dec.ReadHeader(); err != nil {
		return nil, Config{}, err
	}
	if err := dec.StartDecompress(); err != nil {
		return nil, Config{}, err
	}
	components, err := dec.ReadRawData()
	if err != nil {
		return nil, Config{}, err
	}
	cfg := dec.OutputConfig()
	if err := dec.FinishDecompress(); err != nil {
		return nil, Config{}, err
	}
	return components, cfg, nil
}

// DecodeCoefficients decodes JPEG entropy data into quantized DCT coefficient
// blocks using a libjpeg-style jpeg_read_coefficients path.
func DecodeCoefficients(r io.Reader, opts ...Option) ([]CoefficientComponent, Config, error) {
	options := collectOptions(opts)
	return DecodeCoefficientsWithOptions(r, &options)
}

// DecodeCoefficientsWithOptions decodes JPEG entropy data into quantized DCT
// coefficient blocks using an Options value.
func DecodeCoefficientsWithOptions(r io.Reader, opts *Options) ([]CoefficientComponent, Config, error) {
	dec := newDecoderWithOptions(r, optionsFromPointer(opts))
	cfg, err := dec.ReadHeader()
	if err != nil {
		return nil, Config{}, err
	}
	components, err := dec.ReadCoefficients()
	if err != nil {
		return nil, Config{}, err
	}
	return components, cfg, nil
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
	quantSource   *Raster
	palette       color.Palette
	internalDone  bool
	started       bool
	outputPass    bool
	inputScan     int
	outputScan    int
	inputComplete bool
	cropActive    bool
	cropX         int
	cropWidth     int
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
	if err := d.applyDecompressionParameters(); err != nil {
		return Config{}, err
	}
	cfg := configFromHeader(width, height, inputComponents, inputCS)
	if err := d.applyOutputColorSpaceToConfig(&cfg); err != nil {
		return Config{}, err
	}
	if err := d.applyScaleToConfig(&cfg); err != nil {
		return Config{}, err
	}
	if err := d.applyQuantizationToConfig(&cfg); err != nil {
		return Config{}, err
	}
	d.populateHeaderMetadata(&cfg)
	d.header = cfg
	return cfg, nil
}

// ReadHeaderRequireImage reads JPEG metadata using libjpeg's require_image flag.
func (d *Decoder) ReadHeaderRequireImage(requireImage bool) (Config, HeaderStatus, error) {
	if err := d.applyOptions(); err != nil {
		return Config{}, HeaderSuspended, err
	}
	code, err := d.dec.ReadHeaderStatus(requireImage)
	status := HeaderStatus(code)
	if err != nil {
		return Config{}, status, wrapDecodeError("read header", err)
	}
	if status != HeaderOK {
		d.header = Config{}
		return Config{}, status, nil
	}
	if err := d.syncHeaderFromDecoder(); err != nil {
		return Config{}, status, err
	}
	return d.header, status, nil
}

// ConsumeInput advances JPEG input processing using libjpeg-style return codes.
func (d *Decoder) ConsumeInput() (InputStatus, error) {
	if err := d.applyOptions(); err != nil {
		return InputSuspended, err
	}
	code, err := d.dec.ConsumeInput()
	status := InputStatus(code)
	if err != nil {
		return status, wrapDecodeError("consume input", err)
	}
	if status == InputReachedSOS {
		if err := d.applyOptions(); err != nil {
			return status, err
		}
		if err := d.syncHeaderFromDecoder(); err != nil {
			return status, err
		}
	}
	return status, nil
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
	if d.dec.IsProgressive() {
		return wrapDecodeError("start decompress", fmt.Errorf("jpeg: progressive JPEG not yet supported"))
	}
	if d.dec.IsArithmetic() {
		return wrapDecodeError("start decompress", fmt.Errorf("jpeg: arithmetic coding not supported"))
	}
	if err := d.dec.StartDecompress(); err != nil {
		return wrapDecodeError("start decompress", err)
	}
	d.started = true
	d.outputPass = !d.opts.BufferedImage
	d.inputScan = d.dec.InputScanNumber()
	if d.inputScan <= 0 {
		d.inputScan = 1
	}
	d.outputScan = d.inputScan
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
	if d.quantizationRequested() {
		quantizedOutput := d.output
		if err := d.applyQuantizationToConfig(&quantizedOutput); err != nil {
			return err
		}
		src := d.scaled
		if src == nil {
			var err error
			src, err = d.readAndScaleStartedDecoder(baseOutput, d.output.Width, d.output.Height)
			if err != nil {
				return err
			}
		}
		d.quantSource = src
		quantized, err := d.quantizeRaster(src)
		if err != nil {
			return err
		}
		d.scaled = quantized
		d.scaledNextRow = 0
		d.palette = append(color.Palette(nil), quantized.Palette...)
		quantizedOutput.ActualNumColors = len(d.palette)
		d.output = quantizedOutput
	} else {
		d.quantSource = nil
		d.palette = nil
	}
	if d.opts.BufferedImage && d.scaled == nil {
		scaled, err := d.readAndScaleStartedDecoder(baseOutput, d.output.Width, d.output.Height)
		if err != nil {
			return err
		}
		d.scaled = scaled
		d.scaledNextRow = 0
	}
	if d.opts.BufferedImage {
		d.outputPass = false
		d.inputComplete = true
		d.output.InputComplete = true
	}
	return nil
}

// StartDecompress starts decompression using libjpeg-style naming.
func (d *Decoder) StartDecompress() error {
	return d.Start()
}

// CalcOutputDimensions computes output dimensions without starting output.
func (d *Decoder) CalcOutputDimensions() (Config, error) {
	if d.header.Width == 0 || d.header.Height == 0 {
		if _, err := d.ReadHeader(); err != nil {
			return Config{}, err
		}
	}
	if err := d.applyOptions(); err != nil {
		return Config{}, err
	}
	d.dec.CalcOutputDimensions()
	cfg := configFromOutput(d.dec)
	cfg.InputComponents = d.header.InputComponents
	if cfg.InputColorSpace == ColorSpaceUnknown {
		cfg.InputColorSpace = d.header.InputColorSpace
	}
	d.populateHeaderMetadata(&cfg)
	if err := d.applyScaleToConfig(&cfg); err != nil {
		return Config{}, err
	}
	if err := d.applyQuantizationToConfig(&cfg); err != nil {
		return Config{}, err
	}
	d.output = cfg
	return cfg, nil
}

// ReadScanlines reads decoded scanlines into caller-provided row buffers.
func (d *Decoder) ReadScanlines(rows [][]byte) (int, error) {
	if d.opts.RawDataOut {
		return 0, fmt.Errorf("%w: raw data output requires ReadRawData", ErrInvalidOption)
	}
	if d.opts.BufferedImage && !d.outputPass {
		return 0, fmt.Errorf("%w: buffered image output requires StartOutput", ErrInvalidOption)
	}
	cfg := d.OutputConfig()
	if cfg.Stride > 0 {
		for i, row := range rows {
			if len(row) < cfg.Stride {
				return 0, fmt.Errorf("%w: scanline %d has %d bytes, want at least %d", ErrInvalidOption, i, len(row), cfg.Stride)
			}
		}
	}
	d.reportProgress(d.OutputScanline(), cfg.Height)
	if d.scaled != nil {
		rowsRead := 0
		for rowsRead < len(rows) && d.scaledNextRow < d.scaled.Rect.Dy() {
			src := d.scaled.Pix[d.scaledNextRow*d.scaled.Stride : d.scaledNextRow*d.scaled.Stride+d.scaled.Stride]
			if d.cropActive {
				offset := d.cropX * cfg.Components
				src = src[offset : offset+cfg.Stride]
			}
			copy(rows[rowsRead], src)
			rowsRead++
			d.scaledNextRow++
		}
		return rowsRead, nil
	}
	if d.cropActive {
		fullStride := d.dec.OutputWidth() * cfg.Components
		if fullStride <= 0 {
			return 0, nil
		}
		buf := make([]byte, len(rows)*fullStride)
		fullRows := make([][]byte, len(rows))
		for i := range fullRows {
			fullRows[i] = buf[i*fullStride : (i+1)*fullStride]
		}
		n, err := d.dec.ReadScanlines(fullRows)
		if err != nil {
			return n, wrapDecodeError("read scanlines", err)
		}
		offset := d.cropX * cfg.Components
		for i := 0; i < n; i++ {
			copy(rows[i], fullRows[i][offset:offset+cfg.Stride])
		}
		return n, nil
	}
	n, err := d.dec.ReadScanlines(rows)
	if err != nil {
		return n, wrapDecodeError("read scanlines", err)
	}
	return n, nil
}

// ReadRawData returns decoded downsampled component planes. It requires
// WithRawDataOutput and StartDecompress, and mirrors libjpeg's raw_data_out /
// jpeg_read_raw_data path at the facade level.
func (d *Decoder) ReadRawData() ([]RawComponent, error) {
	if !d.opts.RawDataOut {
		return nil, fmt.Errorf("%w: raw data output was not enabled", ErrInvalidOption)
	}
	if !d.started {
		return nil, fmt.Errorf("%w: StartDecompress must be called before ReadRawData", ErrInvalidOption)
	}
	d.reportProgress(d.OutputScanline(), d.OutputConfig().Height)
	internal, err := d.dec.ReadRawData()
	if err != nil {
		return nil, wrapDecodeError("read raw data", err)
	}
	return convertRawComponents(internal), nil
}

// RawDataLinesPerIMCURow returns the maxLines value required for one
// ReadRawDataRows call, matching libjpeg's max_v_samp_factor *
// min_DCT_v_scaled_size rule.
func (d *Decoder) RawDataLinesPerIMCURow() int {
	if lines := d.dec.RawDataLinesPerIMCURow(); lines > 0 {
		return lines
	}
	return d.header.MaxVSampFactor * d.header.MinDCTVScaledSize
}

// ReadRawDataRows returns one iMCU row of decoded downsampled component
// planes. It requires WithRawDataOutput and StartDecompress, and mirrors one
// libjpeg jpeg_read_raw_data call at the facade level. The returned row count
// is the number of output scanlines consumed.
func (d *Decoder) ReadRawDataRows(maxLines int) ([]RawComponent, int, error) {
	if !d.opts.RawDataOut {
		return nil, 0, fmt.Errorf("%w: raw data output was not enabled", ErrInvalidOption)
	}
	if !d.started {
		return nil, 0, fmt.Errorf("%w: StartDecompress must be called before ReadRawDataRows", ErrInvalidOption)
	}
	linesPerIMCU := d.RawDataLinesPerIMCURow()
	if maxLines < linesPerIMCU {
		return nil, 0, fmt.Errorf("%w: raw data maxLines=%d, want at least %d", ErrInvalidOption, maxLines, linesPerIMCU)
	}
	d.reportProgress(d.OutputScanline(), d.OutputConfig().Height)
	internal, rows, err := d.dec.ReadRawDataRows(maxLines)
	if err != nil {
		return nil, rows, wrapDecodeError("read raw data", err)
	}
	return convertRawComponents(internal), rows, nil
}

func convertRawComponents(internal []internaldecoder.RawComponent) []RawComponent {
	if len(internal) == 0 {
		return nil
	}
	out := make([]RawComponent, len(internal))
	for i, comp := range internal {
		pix := make([]byte, len(comp.Pix))
		copy(pix, comp.Pix)
		out[i] = RawComponent{
			Component: convertComponent(comp.Component),
			Width:     comp.Width,
			Height:    comp.Height,
			Stride:    comp.Stride,
			Pix:       pix,
		}
	}
	return out
}

// ReadCoefficients returns quantized DCT coefficient blocks for each component.
// It mirrors libjpeg's jpeg_read_coefficients at the facade level for baseline
// sequential JPEGs.
func (d *Decoder) ReadCoefficients() ([]CoefficientComponent, error) {
	if d.started {
		return nil, fmt.Errorf("%w: ReadCoefficients must be called before StartDecompress", ErrInvalidOption)
	}
	if d.opts.RawDataOut {
		return nil, fmt.Errorf("%w: coefficient output cannot be combined with raw data output", ErrInvalidOption)
	}
	if d.quantizationRequested() {
		return nil, fmt.Errorf("%w: coefficient output cannot be combined with quantized output", ErrInvalidOption)
	}
	if _, enabled, err := d.opts.scaleSize(); err != nil {
		return nil, err
	} else if enabled {
		return nil, fmt.Errorf("%w: coefficient output ignores scaled scanline output", ErrInvalidOption)
	}
	if d.header.Width == 0 || d.header.Height == 0 {
		if _, err := d.ReadHeader(); err != nil {
			return nil, err
		}
	}
	if err := d.applyOptions(); err != nil {
		return nil, err
	}
	if d.dec.IsProgressive() {
		return nil, wrapDecodeError("read coefficients", fmt.Errorf("jpeg: progressive coefficient decoding not yet supported"))
	}
	if d.dec.IsArithmetic() {
		return nil, wrapDecodeError("read coefficients", fmt.Errorf("jpeg: arithmetic coefficient decoding not supported"))
	}
	d.reportProgress(0, d.header.Height)
	internal, err := d.dec.ReadCoefficients()
	if err != nil {
		return nil, wrapDecodeError("read coefficients", err)
	}
	out := make([]CoefficientComponent, len(internal))
	for i, comp := range internal {
		blocks := make([]CoefficientBlock, len(comp.Blocks))
		for j, block := range comp.Blocks {
			copy(blocks[j][:], block[:])
		}
		out[i] = CoefficientComponent{
			Component:      convertComponent(comp.Component),
			WidthInBlocks:  comp.WidthInBlocks,
			HeightInBlocks: comp.HeightInBlocks,
			Blocks:         blocks,
		}
	}
	d.internalDone = true
	d.inputComplete = true
	return out, nil
}

// CropScanline restricts subsequent scanline output to a horizontal region.
func (d *Decoder) CropScanline(xOffset, width int) (int, int, error) {
	if d.opts.RawDataOut {
		return 0, 0, fmt.Errorf("%w: raw data output does not support CropScanline", ErrInvalidOption)
	}
	if xOffset < 0 || width <= 0 {
		return 0, 0, fmt.Errorf("%w: crop x offset must be non-negative and width must be positive", ErrInvalidOption)
	}
	if !d.started || d.output.Width == 0 || d.output.Height == 0 {
		return 0, 0, fmt.Errorf("%w: crop requires StartDecompress", ErrInvalidOption)
	}
	if d.cropActive {
		return 0, 0, fmt.Errorf("%w: crop is already configured", ErrInvalidOption)
	}
	if d.OutputScanline() != 0 {
		return 0, 0, fmt.Errorf("%w: crop must be configured before reading scanlines", ErrInvalidOption)
	}
	if xOffset >= d.output.Width {
		return 0, 0, fmt.Errorf("%w: crop x offset is outside the output image", ErrInvalidOption)
	}
	if width > d.output.Width-xOffset {
		width = d.output.Width - xOffset
	}
	d.cropActive = true
	d.cropX = xOffset
	d.cropWidth = width
	d.output.Width = width
	d.output.Stride = width * d.output.Components
	return xOffset, width, nil
}

// SkipScanlines skips output scanlines using libjpeg-style naming.
func (d *Decoder) SkipScanlines(numLines int) (int, error) {
	if d.opts.RawDataOut {
		return 0, fmt.Errorf("%w: raw data output requires ReadRawData", ErrInvalidOption)
	}
	if d.opts.BufferedImage && !d.outputPass {
		return 0, fmt.Errorf("%w: buffered image output requires StartOutput", ErrInvalidOption)
	}
	if numLines < 0 {
		return 0, fmt.Errorf("%w: scanline count must be non-negative", ErrInvalidOption)
	}
	if numLines == 0 {
		return 0, nil
	}
	d.reportProgress(d.OutputScanline(), d.OutputConfig().Height)
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
	if d.opts.BufferedImage && d.outputPass {
		return fmt.Errorf("%w: FinishOutput must be called before FinishDecompress", ErrInvalidOption)
	}
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
	d.quantSource = nil
	d.palette = nil
	d.internalDone = false
	d.started = false
	d.outputPass = false
	d.inputScan = 0
	d.outputScan = 0
	d.inputComplete = false
	d.cropActive = false
	d.cropX = 0
	d.cropWidth = 0
}

// StartOutput starts one buffered-image output pass. It mirrors libjpeg's
// jpeg_start_output at the facade level. The current implementation supports
// baseline images by replaying a buffered raster; progressive image decoding is
// still reported as ErrUnsupported by StartDecompress.
func (d *Decoder) StartOutput(scanNumber int) (bool, error) {
	if !d.opts.BufferedImage {
		return false, fmt.Errorf("%w: StartOutput requires WithBufferedImage", ErrInvalidOption)
	}
	if !d.started || d.scaled == nil {
		return false, fmt.Errorf("%w: StartDecompress must be called before StartOutput", ErrInvalidOption)
	}
	if d.outputPass {
		return false, fmt.Errorf("%w: FinishOutput must be called before starting another output pass", ErrInvalidOption)
	}
	if scanNumber <= 0 {
		scanNumber = 1
	}
	inputScan := d.InputScanNumber()
	if d.InputComplete() && inputScan > 0 && scanNumber > inputScan {
		scanNumber = inputScan
	}
	d.outputScan = scanNumber
	d.scaledNextRow = 0
	d.outputPass = true
	return true, nil
}

// FinishOutput finishes the current buffered-image output pass. It mirrors
// libjpeg's jpeg_finish_output at the facade level.
func (d *Decoder) FinishOutput() (bool, error) {
	if !d.opts.BufferedImage {
		return false, fmt.Errorf("%w: FinishOutput requires WithBufferedImage", ErrInvalidOption)
	}
	if !d.outputPass {
		return false, fmt.Errorf("%w: StartOutput must be called before FinishOutput", ErrInvalidOption)
	}
	d.outputPass = false
	return true, nil
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

// Palette returns the current quantized output palette after Start.
func (d *Decoder) Palette() color.Palette {
	return append(color.Palette(nil), d.palette...)
}

// NewColormap switches quantized output to a new external colormap and resets
// output scanline state. It mirrors libjpeg's jpeg_new_colormap at the facade
// level for the buffered quantized raster path.
func (d *Decoder) NewColormap(palette color.Palette) error {
	if !d.started || d.quantSource == nil || d.output.PixelFormat != PixelFormatIndexed8 {
		return fmt.Errorf("%w: NewColormap requires started quantized output", ErrInvalidOption)
	}
	if d.opts.BufferedImage && d.outputPass {
		return fmt.Errorf("%w: NewColormap must be called between buffered output passes", ErrInvalidOption)
	}
	next := append(color.Palette(nil), palette...)
	if len(next) == 0 {
		return fmt.Errorf("%w: colormap must not be empty", ErrInvalidOption)
	}
	d.opts.Colormap = next
	d.opts.QuantizeColors = true
	if err := d.validateQuantizationOptions(); err != nil {
		return err
	}
	quantized, err := d.quantizeRaster(d.quantSource)
	if err != nil {
		return err
	}
	d.scaled = quantized
	d.scaledNextRow = 0
	d.palette = append(color.Palette(nil), quantized.Palette...)
	d.output.Quantized = true
	d.output.DesiredNumColors = len(d.palette)
	d.output.ActualNumColors = len(d.palette)
	return nil
}

// SetMarkerProcessor configures APPn or COM marker callback processing before
// ReadHeader. It mirrors libjpeg's jpeg_set_marker_processor API at the facade
// level.
func (d *Decoder) SetMarkerProcessor(markerCode int, processor MarkerProcessor) error {
	option := MarkerProcessorOption{Code: markerCode, Processor: processor}
	if err := d.applyMarkerProcessor(option); err != nil {
		return err
	}
	d.opts.MarkerProcessors = append(d.opts.MarkerProcessors, option)
	return nil
}

// QuantizationTable returns a parsed quantization table after ReadHeader.
func (d *Decoder) QuantizationTable(index int) (QuantizationTable, bool) {
	values, sent, ok := d.dec.QuantizationTable(index)
	if !ok {
		return QuantizationTable{}, false
	}
	return QuantizationTable{Values: values, SentTable: sent}, true
}

// HuffmanTable returns a parsed DC or AC Huffman table after ReadHeader.
func (d *Decoder) HuffmanTable(index int, class HuffmanTableClass) (HuffmanTable, bool) {
	var dc bool
	switch class {
	case HuffmanTableDC:
		dc = true
	case HuffmanTableAC:
		dc = false
	default:
		return HuffmanTable{}, false
	}
	bits, values, sent, ok := d.dec.HuffmanTable(index, dc)
	if !ok {
		return HuffmanTable{}, false
	}
	return HuffmanTable{Bits: bits, Values: values, SentTable: sent}, true
}

// Components returns parsed JPEG component metadata after ReadHeader.
func (d *Decoder) Components() []Component {
	internal := d.dec.Components()
	out := make([]Component, len(internal))
	for i, comp := range internal {
		out[i] = convertComponent(comp)
	}
	return out
}

func convertComponent(comp internaldecoder.Component) Component {
	return Component{
		ID:                     comp.ID,
		Index:                  comp.Index,
		HSampFactor:            comp.HSampFactor,
		VSampFactor:            comp.VSampFactor,
		QuantizationTableIndex: comp.QuantizationTableIndex,
		DCHuffmanTableIndex:    comp.DCHuffmanTableIndex,
		ACHuffmanTableIndex:    comp.ACHuffmanTableIndex,
		WidthInBlocks:          comp.WidthInBlocks,
		HeightInBlocks:         comp.HeightInBlocks,
		DownsampledWidth:       comp.DownsampledWidth,
		DownsampledHeight:      comp.DownsampledHeight,
		DCTHScaledSize:         comp.DCTHScaledSize,
		DCTVScaledSize:         comp.DCTVScaledSize,
		ComponentNeeded:        comp.ComponentNeeded,
	}
}

// RestartInterval returns the parsed DRI restart interval from the header.
func (d *Decoder) RestartInterval() uint {
	return d.dec.RestartInterval()
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

// InputScanNumber returns the number of SOS markers seen by the input side.
func (d *Decoder) InputScanNumber() int {
	if d.inputScan > 0 {
		return d.inputScan
	}
	return d.dec.InputScanNumber()
}

// OutputScanNumber returns the nominal scan number being displayed.
func (d *Decoder) OutputScanNumber() int {
	if d.outputScan > 0 {
		return d.outputScan
	}
	return d.dec.OutputScanNumber()
}

// InputComplete reports whether the JPEG input has been fully consumed.
func (d *Decoder) InputComplete() bool {
	if d.inputComplete {
		return true
	}
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

func (d *Decoder) reportProgress(counter, limit int) {
	if d.opts.ProgressMonitor == nil {
		return
	}
	if counter < 0 {
		counter = 0
	}
	if limit < 0 {
		limit = 0
	}
	if limit > 0 && counter > limit {
		counter = limit
	}
	d.opts.ProgressMonitor(Progress{
		PassCounter:     int64(counter),
		PassLimit:       int64(limit),
		CompletedPasses: 0,
		TotalPasses:     1,
	})
}

func (d *Decoder) applyOptions() error {
	if d.opts.MaxMemoryBytes < 0 {
		return fmt.Errorf("%w: max memory must be non-negative", ErrInvalidOption)
	}
	if err := d.validateQuantizationOptions(); err != nil {
		return err
	}
	if err := d.validateRawDataOptions(); err != nil {
		return err
	}
	for _, saved := range d.opts.SavedMarkers {
		if !validSavedMarkerCode(saved.Code) {
			return fmt.Errorf("%w: marker code 0x%02x cannot be saved", ErrInvalidOption, saved.Code)
		}
		if err := d.dec.SaveMarkers(saved.Code, saved.LengthLimit); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidOption, err)
		}
	}
	for _, processor := range d.opts.MarkerProcessors {
		if err := d.applyMarkerProcessor(processor); err != nil {
			return err
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
	d.dec.SetRawDataOut(d.opts.RawDataOut)
	if err := d.applyDecompressionParameters(); err != nil {
		return err
	}
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

func (d *Decoder) applyDecompressionParameters() error {
	if d.opts.OutputGamma != 0 {
		if d.opts.OutputGamma <= 0 || math.IsNaN(d.opts.OutputGamma) || math.IsInf(d.opts.OutputGamma, 0) {
			return fmt.Errorf("%w: output gamma must be positive and finite", ErrInvalidOption)
		}
		d.dec.SetOutputGamma(d.opts.OutputGamma)
	}
	blockSmoothing, explicit, err := d.opts.BlockSmoothing.value()
	if err != nil {
		return err
	}
	if explicit {
		d.dec.SetBlockSmoothing(blockSmoothing)
	}
	return nil
}

func (d *Decoder) applyMarkerProcessor(option MarkerProcessorOption) error {
	if !validSavedMarkerCode(option.Code) {
		return fmt.Errorf("%w: marker code 0x%02x cannot use a processor", ErrInvalidOption, option.Code)
	}
	if option.Processor == nil {
		return fmt.Errorf("%w: marker code 0x%02x has nil processor", ErrInvalidOption, option.Code)
	}
	err := d.dec.SetMarkerProcessor(option.Code, func(saved internaldecoder.SavedMarker) error {
		payload := make([]byte, len(saved.Data))
		copy(payload, saved.Data)
		return option.Processor(Marker{
			Code:           saved.Code,
			OriginalLength: saved.OriginalLength,
			Data:           payload,
		})
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOption, err)
	}
	return nil
}

func (d *Decoder) quantizeRaster(src *Raster) (*Raster, error) {
	if src == nil {
		return nil, fmt.Errorf("%w: missing decoded raster for quantization", ErrInvalidOption)
	}
	outputCS, err := outputColorSpaceForRaster(src.Format)
	if err != nil {
		return nil, err
	}
	dither, err := d.opts.DitherMode.outputDitherMode()
	if err != nil {
		return nil, err
	}
	colormap, err := outputColormapFromPalette(d.opts.Colormap, src.Format)
	if err != nil {
		return nil, err
	}

	width := src.Rect.Dx()
	height := src.Rect.Dy()
	rows := make([][]byte, height)
	for y := 0; y < height; y++ {
		rows[y] = src.Pix[y*src.Stride : y*src.Stride+width*src.Format.Channels()]
	}
	info := &internaloutput.ImageInfo{
		Width:          width,
		Height:         height,
		NumComponents:  src.Format.Channels(),
		ColorSpace:     outputCS,
		QuantizeColors: true,
		DesiredColors:  d.opts.DesiredNumColors,
		Colormap:       colormap,
		DataPrecision:  8,
	}
	indexRows, qmap, err := internaloutput.QuantizeRows(rows, info, internaloutput.QuantizeOptions{
		DesiredColors: d.opts.DesiredNumColors,
		Colormap:      colormap,
		Dither:        dither,
	})
	if err != nil {
		return nil, wrapQuantizeError(err)
	}

	out := NewRaster(width, height, PixelFormatIndexed8)
	for y, row := range indexRows {
		copy(out.Pix[y*out.Stride:y*out.Stride+width], row)
	}
	out.Palette = paletteFromOutputColormap(qmap)
	return out, nil
}

func outputColorSpaceForRaster(format PixelFormat) (internaloutput.ColorSpace, error) {
	switch format {
	case PixelFormatGray8:
		return internaloutput.ColorSpaceGrayscale, nil
	case PixelFormatRGB24:
		return internaloutput.ColorSpaceRGB, nil
	default:
		return internaloutput.ColorSpaceRGB,
			fmt.Errorf("%w: quantized output supports gray and rgb output, got %s", ErrUnsupported, format)
	}
}

func outputColormapFromPalette(palette color.Palette, format PixelFormat) (*internaloutput.Colormap, error) {
	if len(palette) == 0 {
		return nil, nil
	}
	if len(palette) < 2 {
		return nil, fmt.Errorf("%w: colormap needs at least 2 colors", ErrInvalidOption)
	}
	if len(palette) > 256 {
		return nil, fmt.Errorf("%w: colormap has %d colors, max is 256", ErrInvalidOption, len(palette))
	}
	switch format {
	case PixelFormatGray8:
		gray := make([]uint8, len(palette))
		for i, c := range palette {
			gray[i] = color.GrayModel.Convert(c).(color.Gray).Y
		}
		return &internaloutput.Colormap{Maps: [][]uint8{gray}, NumColors: len(palette)}, nil
	case PixelFormatRGB24:
		red := make([]uint8, len(palette))
		green := make([]uint8, len(palette))
		blue := make([]uint8, len(palette))
		for i, c := range palette {
			r, g, b, _ := c.RGBA()
			red[i] = byte(r >> 8)
			green[i] = byte(g >> 8)
			blue[i] = byte(b >> 8)
		}
		return &internaloutput.Colormap{Maps: [][]uint8{red, green, blue}, NumColors: len(palette)}, nil
	default:
		return nil, fmt.Errorf("%w: quantized output supports gray and rgb output, got %s", ErrUnsupported, format)
	}
}

func paletteFromOutputColormap(cmap *internaloutput.Colormap) color.Palette {
	if cmap == nil || cmap.NumColors <= 0 || len(cmap.Maps) == 0 {
		return nil
	}
	palette := make(color.Palette, cmap.NumColors)
	if len(cmap.Maps) == 1 {
		gray := cmap.Maps[0]
		for i := range palette {
			palette[i] = color.Gray{Y: gray[i]}
		}
		return palette
	}
	red := cmap.Maps[0]
	green := cmap.Maps[1]
	blue := cmap.Maps[2]
	for i := range palette {
		palette[i] = color.RGBA{R: red[i], G: green[i], B: blue[i], A: 0xff}
	}
	return palette
}

func wrapQuantizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unsupported") {
		return fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	return fmt.Errorf("%w: %v", ErrInvalidOption, err)
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

func (d *Decoder) quantizationRequested() bool {
	return d.opts.QuantizeColors || len(d.opts.Colormap) > 0
}

func (d *Decoder) validateQuantizationOptions() error {
	if !d.quantizationRequested() && d.opts.DitherMode == DitherDefault {
		return nil
	}
	if d.opts.DesiredNumColors < 0 {
		return fmt.Errorf("%w: desired colors must be non-negative", ErrInvalidOption)
	}
	if d.opts.DesiredNumColors == 1 {
		return fmt.Errorf("%w: quantized output needs at least 2 colors", ErrInvalidOption)
	}
	if d.opts.DesiredNumColors > 256 {
		return fmt.Errorf("%w: quantized output supports at most 256 colors", ErrInvalidOption)
	}
	if len(d.opts.Colormap) > 256 {
		return fmt.Errorf("%w: colormap has %d colors, max is 256", ErrInvalidOption, len(d.opts.Colormap))
	}
	if len(d.opts.Colormap) == 1 {
		return fmt.Errorf("%w: colormap needs at least 2 colors", ErrInvalidOption)
	}
	if _, err := d.opts.DitherMode.outputDitherMode(); err != nil {
		return err
	}
	return nil
}

func (d *Decoder) validateRawDataOptions() error {
	if !d.opts.RawDataOut {
		return nil
	}
	if d.quantizationRequested() {
		return fmt.Errorf("%w: raw data output cannot be combined with quantized output", ErrInvalidOption)
	}
	if d.opts.BufferedImage {
		return fmt.Errorf("%w: raw data output with buffered image mode is not supported", ErrUnsupported)
	}
	if _, enabled, err := d.opts.scaleSize(); err != nil {
		return err
	} else if enabled {
		return fmt.Errorf("%w: raw data output with facade scaling is not supported", ErrUnsupported)
	}
	return nil
}

func (d *Decoder) applyQuantizationToConfig(cfg *Config) error {
	if !d.quantizationRequested() {
		return nil
	}
	switch cfg.PixelFormat {
	case PixelFormatGray8, PixelFormatRGB24:
	default:
		return fmt.Errorf("%w: quantized output supports gray and rgb output, got %s", ErrUnsupported, cfg.PixelFormat)
	}
	cfg.Components = 1
	cfg.Stride = cfg.Width
	cfg.PixelFormat = PixelFormatIndexed8
	cfg.Quantized = true
	cfg.DesiredNumColors = d.desiredNumColors()
	if len(d.opts.Colormap) > 0 {
		cfg.ActualNumColors = len(d.opts.Colormap)
	}
	return nil
}

func (d *Decoder) desiredNumColors() int {
	if len(d.opts.Colormap) > 0 {
		return len(d.opts.Colormap)
	}
	if d.opts.DesiredNumColors > 0 {
		return d.opts.DesiredNumColors
	}
	return 256
}

func (m DitherMode) outputDitherMode() (internaloutput.DitherMode, error) {
	switch m {
	case DitherDefault:
		return internaloutput.DitherDefault, nil
	case DitherNone:
		return internaloutput.DitherNone, nil
	case DitherOrdered:
		return internaloutput.DitherOrdered, nil
	case DitherFloydSteinberg:
		return internaloutput.DitherFS, nil
	default:
		return internaloutput.DitherDefault, fmt.Errorf("%w: unknown dither mode %d", ErrInvalidOption, m)
	}
}

func (d *Decoder) syncHeaderFromDecoder() error {
	if err := d.applyDecompressionParameters(); err != nil {
		return err
	}
	cfg := configFromHeader(d.dec.ImageWidth(), d.dec.ImageHeight(), d.dec.NumComponents(), d.dec.JPEGColorSpace())
	if err := d.applyOutputColorSpaceToConfig(&cfg); err != nil {
		return err
	}
	if err := d.applyScaleToConfig(&cfg); err != nil {
		return err
	}
	if err := d.applyQuantizationToConfig(&cfg); err != nil {
		return err
	}
	d.populateHeaderMetadata(&cfg)
	d.header = cfg
	return nil
}

func (d *Decoder) populateHeaderMetadata(cfg *Config) {
	cfg.Baseline = d.dec.IsBaseline()
	cfg.Progressive = d.dec.IsProgressive()
	cfg.Arithmetic = d.dec.IsArithmetic()
	cfg.HasMultipleScans = d.dec.HasMultipleScans()
	cfg.InputComplete = d.dec.InputComplete()
	cfg.ImageWidth = d.dec.ImageWidth()
	cfg.ImageHeight = d.dec.ImageHeight()
	cfg.DataPrecision = d.dec.DataPrecision()
	cfg.MaxHSampFactor = d.dec.MaxHSampFactor()
	cfg.MaxVSampFactor = d.dec.MaxVSampFactor()
	cfg.MinDCTHScaledSize = d.dec.MinDCTHScaledSize()
	cfg.MinDCTVScaledSize = d.dec.MinDCTVScaledSize()
	cfg.BlockSize = d.dec.BlockSize()
	cfg.ScaleNum = d.dec.ScaleNum()
	cfg.ScaleDenom = d.dec.ScaleDenom()
	cfg.RawDataOut = d.opts.RawDataOut || d.dec.RawDataOut()
	cfg.BufferedImage = d.opts.BufferedImage
	cfg.OutputGamma = d.dec.OutputGamma()
	cfg.DoBlockSmoothing = d.dec.DoBlockSmoothing()
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
		RecOutbufHeight: 1,
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
		RecOutbufHeight: dec.RecommendedOutputBufferHeight(),
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
