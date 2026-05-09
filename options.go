package djpeg

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Public sentinel errors returned by the package.
var (
	ErrInvalidJPEG   = errors.New("djpeg: invalid jpeg")
	ErrUnsupported   = errors.New("djpeg: unsupported jpeg feature")
	ErrInvalidOption = errors.New("djpeg: invalid option")
	ErrMemoryLimit   = errors.New("djpeg: memory limit exceeded")
)

// Option configures a decoder.
type Option func(*Options)

// Options controls JPEG decompression.
//
// The zero value is valid and matches the CLI default: IJG-compatible integer
// IDCT with fancy upsampling enabled.
type Options struct {
	IDCT              IDCTMethod
	Upsampling        UpsamplingMode
	Compatibility     CompatibilityMode
	InputColorSpace   InputColorSpace
	OutputColorSpace  ColorSpace
	ColorTransform    ColorTransform
	ChromaIDCTScaling ChromaIDCTScalingMode
	MaxMemoryBytes    int64
	ScaleNumerator    int
	ScaleDenominator  int
	SavedMarkers      []SavedMarkerOption
}

// SavedMarkerOption configures marker data retention during ReadHeader.
type SavedMarkerOption struct {
	Code        int
	LengthLimit uint
}

// IDCTMethod selects the inverse DCT implementation.
type IDCTMethod int

const (
	IDCTDefault IDCTMethod = iota
	IDCTInt
	IDCTFast
	IDCTFloat
)

// UpsamplingMode selects chroma upsampling behavior.
type UpsamplingMode int

const (
	UpsamplingDefault UpsamplingMode = iota
	UpsamplingFancy
	UpsamplingNearest
)

// CompatibilityMode selects decoder compatibility behavior.
type CompatibilityMode int

const (
	CompatibilityDefault CompatibilityMode = iota
	// CompatibilityIJG9 preserves the current IJG 9f exact-parity path.
	CompatibilityIJG9
	// CompatibilityTurboFancy is kept for callers that used the old
	// implementation-detail name.
	//
	// Deprecated: use CompatibilityPopplerPDF.
	CompatibilityTurboFancy
	// CompatibilityPopplerPDF selects the PDF DCT stream profile used to match
	// Poppler/ImageMagick style output.
	CompatibilityPopplerPDF
)

// ChromaIDCTScalingMode controls libjpeg's chroma IDCT scaling behavior.
type ChromaIDCTScalingMode int

const (
	ChromaIDCTScalingDefault ChromaIDCTScalingMode = iota
	ChromaIDCTScalingEnabled
	ChromaIDCTScalingDisabled
)

// ColorTransform selects an inverse color transform for RGB-style JPEG data.
type ColorTransform int

const (
	ColorTransformDefault ColorTransform = iota
	ColorTransformNone
	ColorTransformSubtractGreen
)

// InputColorSpace overrides the JPEG sample color space inferred from markers.
type InputColorSpace int

const (
	InputAuto InputColorSpace = iota
	InputGray
	InputRGB
	InputYCbCr
	InputCMYK
	InputYCCK
)

// WithIDCT selects the inverse DCT method.
func WithIDCT(method IDCTMethod) Option {
	return func(opts *Options) {
		opts.IDCT = method
	}
}

// WithUpsampling selects the chroma upsampling mode.
func WithUpsampling(mode UpsamplingMode) Option {
	return func(opts *Options) {
		opts.Upsampling = mode
	}
}

// WithNoSmooth disables fancy upsampling, matching the CLI --nosmooth flag.
func WithNoSmooth() Option {
	return WithUpsampling(UpsamplingNearest)
}

// WithFast enables the currently implemented subset of libjpeg's -fast mode:
// fast integer IDCT plus nearest-neighbor chroma upsampling.
func WithFast() Option {
	return func(opts *Options) {
		opts.IDCT = IDCTFast
		opts.Upsampling = UpsamplingNearest
	}
}

// WithCompatibility selects a decoder compatibility mode.
func WithCompatibility(mode CompatibilityMode) Option {
	return func(opts *Options) {
		opts.Compatibility = mode
	}
}

// WithTurboFancy uses 8x8 chroma IDCT plus fancy upsampling for 4:2:0 data.
//
// Deprecated: use WithCompatibility(CompatibilityPopplerPDF). This alias is
// kept for the CLI --turbo-fancy flag and older callers.
func WithTurboFancy() Option {
	return WithCompatibility(CompatibilityPopplerPDF)
}

// WithChromaIDCTScaling controls whether chroma components use libjpeg's
// scaled-IDCT path when fancy upsampling is enabled.
func WithChromaIDCTScaling(enabled bool) Option {
	return func(opts *Options) {
		if enabled {
			opts.ChromaIDCTScaling = ChromaIDCTScalingEnabled
		} else {
			opts.ChromaIDCTScaling = ChromaIDCTScalingDisabled
		}
	}
}

// WithInputColorSpace overrides the JPEG sample color space.
func WithInputColorSpace(space InputColorSpace) Option {
	return func(opts *Options) {
		opts.InputColorSpace = space
	}
}

// WithColorTransform overrides the inverse color transform used for RGB-style
// JPEG data. Use ColorTransformDefault to follow the JPEG stream metadata.
func WithColorTransform(transform ColorTransform) Option {
	return func(opts *Options) {
		opts.ColorTransform = transform
	}
}

// WithOutputColorSpace forces decoded output pixels into a supported output
// color space. Use ColorSpaceUnknown for the decoder default.
func WithOutputColorSpace(space ColorSpace) Option {
	return func(opts *Options) {
		opts.OutputColorSpace = space
	}
}

// WithGrayscaleOutput forces grayscale output, matching the CLI --grayscale
// flag.
func WithGrayscaleOutput() Option {
	return WithOutputColorSpace(ColorSpaceGray)
}

// WithRGBOutput forces RGB output, matching the CLI --rgb flag.
func WithRGBOutput() Option {
	return WithOutputColorSpace(ColorSpaceRGB)
}

// WithMaxMemory sets an approximate upper bound, in bytes, for memory held by
// the decoder's compressed scan buffer and decoded component buffers. A value
// of zero leaves the decoder unlimited.
func WithMaxMemory(bytes int64) Option {
	return func(opts *Options) {
		opts.MaxMemoryBytes = bytes
	}
}

// WithScale scales the decoded output to the closest libjpeg-supported DCT
// scale ratio. For 8x8 DCT JPEGs, ratios map to 1/8 through 16/8.
func WithScale(numerator, denominator int) Option {
	return func(opts *Options) {
		opts.ScaleNumerator = numerator
		opts.ScaleDenominator = denominator
	}
}

// WithSavedMarkers mirrors libjpeg's jpeg_save_markers API for COM and APPn
// markers. It must be set before ReadHeader. lengthLimit caps saved marker data
// bytes; zero disables saving for that marker while preserving APP0/APP14
// internal processing.
func WithSavedMarkers(markerCode int, lengthLimit uint) Option {
	return func(opts *Options) {
		opts.SavedMarkers = append(opts.SavedMarkers, SavedMarkerOption{
			Code:        markerCode,
			LengthLimit: lengthLimit,
		})
	}
}

// ParseIDCTMethod converts a CLI-style IDCT name into an IDCTMethod.
func ParseIDCTMethod(s string) (IDCTMethod, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "default":
		return IDCTDefault, nil
	case "int", "islow", "slow":
		return IDCTInt, nil
	case "fast", "ifast":
		return IDCTFast, nil
	case "float", "flt":
		return IDCTFloat, nil
	default:
		return IDCTDefault, fmt.Errorf("%w: unknown IDCT method %q", ErrInvalidOption, s)
	}
}

// ParseInputColorSpace converts a CLI-style color space name.
func ParseInputColorSpace(s string) (InputColorSpace, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return InputAuto, nil
	case "gray", "grey", "grayscale", "greyscale":
		return InputGray, nil
	case "rgb":
		return InputRGB, nil
	case "ycbcr", "ycc":
		return InputYCbCr, nil
	case "cmyk":
		return InputCMYK, nil
	case "ycck":
		return InputYCCK, nil
	default:
		return InputAuto, fmt.Errorf("%w: unknown input color space %q", ErrInvalidOption, s)
	}
}

// ParseCompatibilityMode converts a CLI-style compatibility profile name.
func ParseCompatibilityMode(s string) (CompatibilityMode, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	name = strings.ReplaceAll(name, "_", "-")
	switch name {
	case "", "default":
		return CompatibilityDefault, nil
	case "ijg", "ijg9", "ijg-9", "ijg-9f", "libjpeg", "libjpeg9", "libjpeg-9", "libjpeg-9f":
		return CompatibilityIJG9, nil
	case "poppler", "poppler-pdf", "pdf", "imagemagick", "magick", "libjpeg-turbo", "turbo", "turbo-fancy":
		return CompatibilityPopplerPDF, nil
	default:
		return CompatibilityDefault, fmt.Errorf("%w: unknown compatibility mode %q", ErrInvalidOption, s)
	}
}

// ParseColorTransform converts a CLI-style color transform name.
func ParseColorTransform(s string) (ColorTransform, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	name = strings.ReplaceAll(name, "_", "-")
	switch name {
	case "", "auto", "default":
		return ColorTransformDefault, nil
	case "none", "0":
		return ColorTransformNone, nil
	case "subtract-green", "subtractgreen", "rgb1", "1":
		return ColorTransformSubtractGreen, nil
	default:
		return ColorTransformDefault, fmt.Errorf("%w: unknown color transform %q", ErrInvalidOption, s)
	}
}

// ParseScale converts a libjpeg-style scale value. M/N is preferred; a bare M
// follows libjpeg's parser and uses the default denominator 8.
func ParseScale(s string) (int, int, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, 0, fmt.Errorf("%w: empty scale", ErrInvalidOption)
	}
	parts := strings.Split(raw, "/")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("%w: invalid scale %q", ErrInvalidOption, s)
	}
	numerator, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || numerator <= 0 {
		return 0, 0, fmt.Errorf("%w: invalid scale numerator %q", ErrInvalidOption, s)
	}
	denominator := 8
	if len(parts) == 2 {
		denominator, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || denominator <= 0 {
			return 0, 0, fmt.Errorf("%w: invalid scale denominator %q", ErrInvalidOption, s)
		}
	}
	return numerator, denominator, nil
}

// ParseMemoryLimit converts a libjpeg-style memory value to bytes. A bare
// number is interpreted as kilobytes; m/M means megabytes.
func ParseMemoryLimit(s string) (int64, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, fmt.Errorf("%w: empty memory limit", ErrInvalidOption)
	}

	i := 0
	for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("%w: invalid memory limit %q", ErrInvalidOption, s)
	}
	value, err := strconv.ParseInt(raw[:i], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid memory limit %q", ErrInvalidOption, s)
	}

	suffix := strings.ToLower(strings.TrimSpace(raw[i:]))
	var multiplier int64
	switch suffix {
	case "", "k", "kb":
		multiplier = 1000
	case "m", "mb":
		multiplier = 1000 * 1000
	default:
		return 0, fmt.Errorf("%w: invalid memory limit suffix %q", ErrInvalidOption, raw[i:])
	}
	if value > math.MaxInt64/multiplier {
		return 0, fmt.Errorf("%w: memory limit %q overflows int64", ErrInvalidOption, s)
	}
	return value * multiplier, nil
}

// ParseOutputColorSpace converts a CLI-style output color space name.
func ParseOutputColorSpace(s string) (ColorSpace, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto", "default":
		return ColorSpaceUnknown, nil
	case "gray", "grey", "grayscale", "greyscale":
		return ColorSpaceGray, nil
	case "rgb":
		return ColorSpaceRGB, nil
	case "cmyk":
		return ColorSpaceCMYK, nil
	case "ycck":
		return ColorSpaceYCCK, nil
	default:
		return ColorSpaceUnknown, fmt.Errorf("%w: unknown output color space %q", ErrInvalidOption, s)
	}
}

func collectOptions(opts []Option) Options {
	var out Options
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

func optionsFromPointer(opts *Options) Options {
	if opts == nil {
		return Options{}
	}
	return *opts
}

func (c ColorSpace) outputDecoderName() (string, error) {
	switch c {
	case ColorSpaceUnknown:
		return "auto", nil
	case ColorSpaceGray:
		return "gray", nil
	case ColorSpaceRGB:
		return "rgb", nil
	case ColorSpaceCMYK:
		return "cmyk", nil
	case ColorSpaceYCCK:
		return "ycck", nil
	default:
		return "", fmt.Errorf("%w: output color space %s is not supported", ErrUnsupported, c)
	}
}

func (t ColorTransform) decoderName() (string, error) {
	switch t {
	case ColorTransformDefault:
		return "auto", nil
	case ColorTransformNone:
		return "none", nil
	case ColorTransformSubtractGreen:
		return "subtract-green", nil
	default:
		return "", fmt.Errorf("%w: unknown color transform %d", ErrInvalidOption, t)
	}
}

func (o Options) scaleSize() (int, bool, error) {
	numerator := o.ScaleNumerator
	denominator := o.ScaleDenominator
	if numerator == 0 && denominator == 0 {
		return 8, false, nil
	}
	if numerator <= 0 || denominator <= 0 {
		return 0, false, fmt.Errorf("%w: scale must be positive", ErrInvalidOption)
	}
	for scale := 1; scale <= 16; scale++ {
		if int64(numerator)*8 <= int64(denominator)*int64(scale) {
			return scale, scale != 8, nil
		}
	}
	return 16, true, nil
}

func scaledDimensions(width, height, scale int) (int, int, error) {
	if scale <= 0 {
		return 0, 0, fmt.Errorf("%w: scale must be positive", ErrInvalidOption)
	}
	if width < 0 || height < 0 {
		return 0, 0, fmt.Errorf("%w: negative image dimensions", ErrInvalidJPEG)
	}
	if width == 0 || height == 0 {
		return 0, 0, nil
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/scale || height > maxInt/scale {
		return 0, 0, fmt.Errorf("%w: scaled dimensions overflow", ErrInvalidOption)
	}
	return divRoundUp(width*scale, 8), divRoundUp(height*scale, 8), nil
}

func divRoundUp(n, d int) int {
	return (n + d - 1) / d
}

func (m IDCTMethod) decoderName() (string, error) {
	switch m {
	case IDCTDefault, IDCTInt:
		return "int", nil
	case IDCTFast:
		return "fast", nil
	case IDCTFloat:
		return "float", nil
	default:
		return "", fmt.Errorf("%w: unknown IDCT method %d", ErrInvalidOption, m)
	}
}

func (m UpsamplingMode) fancy() (bool, error) {
	switch m {
	case UpsamplingDefault, UpsamplingFancy:
		return true, nil
	case UpsamplingNearest:
		return false, nil
	default:
		return false, fmt.Errorf("%w: unknown upsampling mode %d", ErrInvalidOption, m)
	}
}

func (m CompatibilityMode) String() string {
	switch m {
	case CompatibilityDefault:
		return "default"
	case CompatibilityIJG9:
		return "ijg9"
	case CompatibilityTurboFancy:
		return "turbo-fancy"
	case CompatibilityPopplerPDF:
		return "poppler-pdf"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

func (o Options) effectiveChromaIDCTScaling() (bool, error) {
	switch o.ChromaIDCTScaling {
	case ChromaIDCTScalingDefault:
		return o.Compatibility.chromaIDCTScaling()
	case ChromaIDCTScalingEnabled:
		return true, nil
	case ChromaIDCTScalingDisabled:
		return false, nil
	default:
		return false, fmt.Errorf("%w: unknown chroma IDCT scaling mode %d", ErrInvalidOption, o.ChromaIDCTScaling)
	}
}

func (m CompatibilityMode) chromaIDCTScaling() (bool, error) {
	switch m {
	case CompatibilityDefault, CompatibilityIJG9:
		return true, nil
	case CompatibilityTurboFancy, CompatibilityPopplerPDF:
		return false, nil
	default:
		return false, fmt.Errorf("%w: unknown compatibility mode %d", ErrInvalidOption, m)
	}
}

func (s InputColorSpace) decoderName() (string, error) {
	switch s {
	case InputAuto:
		return "auto", nil
	case InputGray:
		return "gray", nil
	case InputRGB:
		return "rgb", nil
	case InputYCbCr:
		return "ycbcr", nil
	case InputCMYK:
		return "cmyk", nil
	case InputYCCK:
		return "ycck", nil
	default:
		return "", fmt.Errorf("%w: unknown input color space %d", ErrInvalidOption, s)
	}
}
