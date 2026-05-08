package djpeg

import (
	"errors"
	"fmt"
	"strings"
)

// Public sentinel errors returned by the package.
var (
	ErrInvalidJPEG   = errors.New("djpeg: invalid jpeg")
	ErrUnsupported   = errors.New("djpeg: unsupported jpeg feature")
	ErrInvalidOption = errors.New("djpeg: invalid option")
)

// Option configures a decoder.
type Option func(*Options)

// Options controls JPEG decompression.
//
// The zero value is valid and matches the CLI default: IJG-compatible integer
// IDCT with fancy upsampling enabled.
type Options struct {
	IDCT            IDCTMethod
	Upsampling      UpsamplingMode
	Compatibility   CompatibilityMode
	InputColorSpace InputColorSpace
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
	CompatibilityIJG9
	CompatibilityTurboFancy
	CompatibilityPopplerPDF
)

// InputColorSpace overrides the JPEG sample color space inferred from markers.
type InputColorSpace int

const (
	InputAuto InputColorSpace = iota
	InputGray
	InputRGB
	InputYCbCr
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

// WithCompatibility selects a decoder compatibility mode.
func WithCompatibility(mode CompatibilityMode) Option {
	return func(opts *Options) {
		opts.Compatibility = mode
	}
}

// WithTurboFancy uses 8x8 chroma IDCT plus fancy upsampling for 4:2:0 data.
//
// This matches the CLI --turbo-fancy flag and is useful for Poppler or
// ImageMagick style PDF DCT stream output.
func WithTurboFancy() Option {
	return WithCompatibility(CompatibilityTurboFancy)
}

// WithInputColorSpace overrides the JPEG sample color space.
func WithInputColorSpace(space InputColorSpace) Option {
	return func(opts *Options) {
		opts.InputColorSpace = space
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
	default:
		return InputAuto, fmt.Errorf("%w: unknown input color space %q", ErrInvalidOption, s)
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
	default:
		return "", fmt.Errorf("%w: unknown input color space %d", ErrInvalidOption, s)
	}
}
