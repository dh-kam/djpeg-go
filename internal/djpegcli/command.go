// Package djpegcli implements the djpeg command-line interface.
package djpegcli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime/pprof"

	djpeg "github.com/dh-kam/djpeg-go"
	"github.com/dh-kam/djpeg-go/internal/output"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

// Build metadata is set by release builds through -ldflags.
var (
	Version         = "dev"
	Commit          = "unknown"
	Date            = "unknown"
	UpstreamVersion = "9f"
)

// Options holds the parsed command-line options.
type Options struct {
	OutFile          string `flag:"outfile" usage:"Write output to NAME"`
	Verbose          bool   `flag:"verbose" usage:"Verbose output"`
	Debug            bool   `flag:"debug" usage:"Emit debug output"`
	Grayscale        bool   `flag:"grayscale" usage:"Force grayscale output"`
	ForceRGB         bool   `flag:"rgb" usage:"Force RGB output"`
	NumColors        int    `flag:"colors" usage:"Reduce image to no more than N colors"`
	Fast             bool   `flag:"fast" usage:"Fast, low-quality processing"`
	NoSmooth         bool   `flag:"nosmooth" usage:"Don't use high-quality upsampling"`
	OnePass          bool   `flag:"onepass" usage:"Use 1-pass quantization"`
	DctMethod        string `flag:"dct" usage:"IDCT method: int, fast, float"`
	DitherMode       string `flag:"dither" usage:"Dithering: fs, none, ordered"`
	MapFile          string `flag:"map" usage:"Map to colors from a GIF/PPM file"`
	MaxMemory        string `flag:"maxmemory" usage:"Maximum memory (KB or MB with m)"`
	Scale            string `flag:"scale" usage:"Scale output image by fraction M/N"`
	InputColorSpace  string `flag:"input-colorspace" usage:"Interpret JPEG samples as auto, grayscale, rgb, ycbcr, cmyk, ycck, big-gamut-rgb, or big-gamut-ycbcr"`
	OutputColorSpace string `flag:"output-colorspace" usage:"Force output samples as auto, grayscale, rgb, ycbcr, cmyk, ycck, big-gamut-rgb, or big-gamut-ycbcr"`
	ColorTransform   string `flag:"color-transform" usage:"Inverse color transform: auto, none, subtract-green"`
	Compatibility    string `flag:"compatibility" usage:"Compatibility profile: default, ijg9, poppler-pdf"`
	TurboFancy       bool   `flag:"turbo-fancy" usage:"Deprecated alias for --compatibility poppler-pdf"`
	CPUProfile       string `flag:"cpuprofile" usage:"Write CPU profile to FILE"`

	FmtPPM   bool `flag:"ppm" usage:"Output PPM/PGM format"`
	FmtPNM   bool `flag:"pnm" usage:"Output PPM/PGM format"`
	FmtBMP   bool `flag:"bmp" usage:"Output BMP format"`
	FmtOS2   bool `flag:"os2" usage:"Output OS/2 BMP format"`
	FmtGIF   bool `flag:"gif" usage:"Output GIF format"`
	FmtGIF0  bool `flag:"gif0" usage:"Output uncompressed GIF format"`
	FmtTarga bool `flag:"targa" usage:"Output Targa format"`
	FmtRLE   bool `flag:"rle" usage:"Output RLE format"`

	Format    output.Format
	InputFile string
}

// VersionString returns the CLI version shown by --version.
func VersionString() string {
	return fmt.Sprintf("djpeg-go %s (commit %s, built %s, based on IJG libjpeg %s)", Version, Commit, Date, UpstreamVersion)
}

// NewRootCommand creates the djpeg root command.
func NewRootCommand() *cobra.Command {
	opts := &Options{}
	binder := flagsbinder.NewViperCobraFlagsBinder().
		String("outfile", "", "Write output to NAME").
		Bool("verbose", false, "Verbose output").
		Bool("debug", false, "Emit debug output").
		Bool("grayscale", false, "Force grayscale output").
		Bool("rgb", false, "Force RGB output").
		Int("colors", 0, "Reduce image to no more than N colors").
		Bool("fast", false, "Fast, low-quality processing").
		Bool("nosmooth", false, "Don't use high-quality upsampling").
		Bool("onepass", false, "Use 1-pass quantization").
		String("dct", "", "IDCT method: int, fast, float").
		String("dither", "", "Dithering: fs, none, ordered").
		String("map", "", "Map to colors from a GIF/PPM file").
		String("maxmemory", "", "Maximum memory (KB or MB with m)").
		String("scale", "", "Scale output image by fraction M/N").
		String("input-colorspace", "auto", "Interpret JPEG samples as auto, grayscale, rgb, ycbcr, cmyk, ycck, big-gamut-rgb, or big-gamut-ycbcr").
		String("output-colorspace", "auto", "Force output samples as auto, grayscale, rgb, ycbcr, cmyk, ycck, big-gamut-rgb, or big-gamut-ycbcr").
		String("color-transform", "auto", "Inverse color transform: auto, none, subtract-green").
		String("compatibility", "default", "Compatibility profile: default, ijg9, poppler-pdf").
		Bool("turbo-fancy", false, "Deprecated alias for --compatibility poppler-pdf").
		String("cpuprofile", "", "Write CPU profile to FILE").
		Bool("ppm", false, "Output PPM/PGM format").
		Bool("pnm", false, "Output PPM/PGM format").
		Bool("bmp", false, "Output BMP format").
		Bool("os2", false, "Output OS/2 BMP format").
		Bool("gif", false, "Output GIF format").
		Bool("gif0", false, "Output uncompressed GIF format").
		Bool("targa", false, "Output Targa format").
		Bool("rle", false, "Output RLE format")

	cmd := &cobra.Command{
		Use:           "djpeg [options] [inputfile]",
		Short:         "djpeg decodes JPEG images to various output formats.",
		Version:       VersionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, opts, args...); err != nil {
				_ = cmd.Usage()
				return fmt.Errorf("binding flags: %w", err)
			}
			if opts.Debug {
				opts.Verbose = true
			}

			// Determine output format
			opts.Format = output.FormatPPM // default
			if opts.FmtBMP {
				opts.Format = output.FormatBMP
			}
			if opts.FmtOS2 {
				opts.Format = output.FormatBMPOS2
			}
			if opts.FmtGIF {
				opts.Format = output.FormatGIF
			}
			if opts.FmtGIF0 {
				opts.Format = output.FormatGIF0
			}
			if opts.FmtTarga {
				opts.Format = output.FormatTarga
			}
			if opts.FmtRLE {
				opts.Format = output.FormatRLE
			}
			if opts.FmtPPM || opts.FmtPNM {
				opts.Format = output.FormatPPM
			}

			if len(args) > 1 {
				_ = cmd.Usage()
				return fmt.Errorf("only one input file allowed")
			}
			if len(args) == 1 {
				opts.InputFile = args[0]
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			stopProfile, err := startCPUProfile(opts.CPUProfile)
			if err != nil {
				return err
			}
			defer stopProfile()

			// Open input file or stdin
			var input io.Reader
			var inputCloser io.Closer
			if opts.InputFile != "" {
				f, err := os.Open(opts.InputFile)
				if err != nil {
					return fmt.Errorf("can't open %s: %w", opts.InputFile, err)
				}
				input = f
				inputCloser = f
			} else {
				input = cmd.InOrStdin()
				inputCloser = nil
			}
			defer func() {
				if inputCloser != nil {
					inputCloser.Close()
				}
			}()

			// Open output file or stdout
			var outWriter io.Writer
			var outCloser io.Closer
			if opts.OutFile != "" {
				f, err := os.Create(opts.OutFile)
				if err != nil {
					return fmt.Errorf("can't open %s: %w", opts.OutFile, err)
				}
				outWriter = f
				outCloser = f
			} else {
				outWriter = cmd.OutOrStdout()
				outCloser = nil
			}
			defer func() {
				if outCloser != nil {
					outCloser.Close()
				}
			}()

			// Buffer output for performance
			bufOut := bufio.NewWriter(outWriter)
			defer bufOut.Flush()

			if err := Decompress(input, bufOut, opts); err != nil {
				return err
			}

			return nil
		},
	}

	binder.SetTo(cmd.Flags())
	_ = cmd.Flags().MarkHidden("cpuprofile")
	return cmd
}

// Execute runs the djpeg CLI.
func Execute() error {
	return NewRootCommand().Execute()
}

func startCPUProfile(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating CPU profile %s: %w", path, err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("starting CPU profile %s: %w", path, err)
	}
	return func() {
		pprof.StopCPUProfile()
		_ = f.Close()
	}, nil
}

// Decompress reads a JPEG from input and writes the decoded image to out
// in the requested format.
func Decompress(input io.Reader, out io.Writer, opts *Options) error {
	if err := validateUnsupportedOptions(opts); err != nil {
		return err
	}
	decodeOptions, err := decoderOptions(opts)
	if err != nil {
		return err
	}

	// Read color map if specified
	var colormap *output.Colormap
	if opts.MapFile != "" {
		f, err := os.Open(opts.MapFile)
		if err != nil {
			return fmt.Errorf("can't open color map file %s: %w", opts.MapFile, err)
		}
		defer f.Close()
		colormap, err = output.ReadColorMap(f)
		if err != nil {
			return fmt.Errorf("reading color map: %w", err)
		}
	}

	// Create the output format writer
	w, err := output.NewWriter(opts.Format)
	if err != nil {
		return fmt.Errorf("creating output writer: %w", err)
	}

	// Create decoder and read the JPEG header
	dec := djpeg.NewDecoder(input, decodeOptions...)

	header, err := dec.ReadHeader()
	if err != nil {
		return fmt.Errorf("reading JPEG header: %w", err)
	}

	// Start decompression (sets up pipeline)
	if err := dec.Start(); err != nil {
		return fmt.Errorf("starting decompression: %w", err)
	}

	outputConfig := dec.OutputConfig()
	outputWidth := outputConfig.Width
	outputHeight := outputConfig.Height
	outputComponents := outputConfig.Components

	// Build output image info
	var colorSpace output.ColorSpace
	switch outputConfig.PixelFormat {
	case djpeg.PixelFormatGray8:
		colorSpace = output.ColorSpaceGrayscale
	case djpeg.PixelFormatRGB24:
		colorSpace = output.ColorSpaceRGB
	case djpeg.PixelFormatYCbCr24:
		colorSpace = output.ColorSpaceYCbCr
	case djpeg.PixelFormatBigGamutYCbCr24:
		colorSpace = output.ColorSpaceBigGamutYCbCr
	case djpeg.PixelFormatCMYK32:
		colorSpace = output.ColorSpaceCMYK
	case djpeg.PixelFormatYCCK32:
		colorSpace = output.ColorSpaceYCCK
	default:
		return fmt.Errorf("%w: cmd/djpeg output writers do not support %s pixels", djpeg.ErrUnsupported, outputConfig.PixelFormat)
	}

	info := &output.ImageInfo{
		Width:         outputWidth,
		Height:        outputHeight,
		NumComponents: outputComponents,
		ColorSpace:    colorSpace,
		DataPrecision: 8,
	}

	quantizeOptions, err := prepareQuantization(opts, info, colormap)
	if err != nil {
		return err
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Input: %dx%d, %d components\n",
			header.Width, header.Height, header.InputComponents)
		fmt.Fprintf(os.Stderr, "Output: %dx%d, %d components\n",
			outputWidth, outputHeight, outputComponents)
		fmt.Fprintf(os.Stderr, "Output format: %s\n", opts.Format)
		if opts.Grayscale {
			fmt.Fprintf(os.Stderr, "Color space: grayscale\n")
		}
		if opts.NumColors > 0 {
			fmt.Fprintf(os.Stderr, "Colors: %d\n", opts.NumColors)
		}
	}

	if info.QuantizeColors {
		rows, err := readDecodedRows(dec, outputHeight, outputWidth*outputComponents)
		if err != nil {
			return err
		}
		if err := dec.Finish(); err != nil {
			return fmt.Errorf("finishing decompression: %w", err)
		}
		indexRows, qmap, err := output.QuantizeRows(rows, info, quantizeOptions)
		if err != nil {
			return fmt.Errorf("%w: quantizing output: %v", djpeg.ErrInvalidOption, err)
		}
		info.Colormap = qmap
		info.DesiredColors = qmap.NumColors

		if err := w.Start(out, info); err != nil {
			return fmt.Errorf("starting output: %w", err)
		}
		for y, row := range indexRows {
			if err := w.WriteScanline(row); err != nil {
				return fmt.Errorf("writing scanline %d: %w", y, err)
			}
		}
		if err := w.Finish(); err != nil {
			return fmt.Errorf("finishing output: %w", err)
		}
		return nil
	}

	// Start the output writer
	if err := w.Start(out, info); err != nil {
		return fmt.Errorf("starting output: %w", err)
	}

	// Read and write scanlines
	rowStride := outputWidth * outputComponents
	scanline := make([]byte, rowStride)
	for y := 0; y < outputHeight; y++ {
		n, err := dec.ReadScanlines([][]byte{scanline})
		if err != nil {
			return fmt.Errorf("reading scanline %d: %w", y, err)
		}
		if n == 0 {
			// End of image or error
			break
		}
		if err := w.WriteScanline(scanline); err != nil {
			return fmt.Errorf("writing scanline %d: %w", y, err)
		}
	}

	// Finish
	if err := dec.Finish(); err != nil {
		return fmt.Errorf("finishing decompression: %w", err)
	}
	if err := w.Finish(); err != nil {
		return fmt.Errorf("finishing output: %w", err)
	}

	return nil
}

func prepareQuantization(opts *Options, info *output.ImageInfo, colormap *output.Colormap) (output.QuantizeOptions, error) {
	dither, err := output.ParseDitherMode(opts.DitherMode)
	if err != nil {
		return output.QuantizeOptions{}, fmt.Errorf("%w: %v", djpeg.ErrInvalidOption, err)
	}

	desiredColors := opts.NumColors
	quantize := desiredColors > 0 || colormap != nil
	if (opts.Format == output.FormatGIF || opts.Format == output.FormatGIF0) && colorSpaceNeedsGIFPalette(info.ColorSpace) {
		quantize = true
		if desiredColors == 0 && colormap == nil {
			desiredColors = 256
			if opts.Fast {
				desiredColors = 216
			}
		}
	}
	if opts.Fast && quantize && desiredColors == 0 && colormap == nil {
		desiredColors = 216
	}
	if dither == output.DitherDefault && opts.Fast && quantize {
		dither = output.DitherOrdered
	}

	if quantize {
		info.QuantizeColors = true
		info.DesiredColors = desiredColors
		info.Colormap = colormap
		if colormap != nil {
			info.DesiredColors = colormap.NumColors
		}
	}

	return output.QuantizeOptions{
		DesiredColors: desiredColors,
		Colormap:      colormap,
		Dither:        dither,
		OnePass:       opts.OnePass || opts.Fast,
	}, nil
}

func colorSpaceNeedsGIFPalette(colorSpace output.ColorSpace) bool {
	switch colorSpace {
	case output.ColorSpaceRGB, output.ColorSpaceYCbCr, output.ColorSpaceBigGamutYCbCr, output.ColorSpaceCMYK, output.ColorSpaceYCCK:
		return true
	default:
		return false
	}
}

func readDecodedRows(dec *djpeg.Decoder, outputHeight, rowStride int) ([][]byte, error) {
	rows := make([][]byte, 0, outputHeight)
	scanline := make([]byte, rowStride)
	for y := 0; y < outputHeight; y++ {
		n, err := dec.ReadScanlines([][]byte{scanline})
		if err != nil {
			return nil, fmt.Errorf("reading scanline %d: %w", y, err)
		}
		if n == 0 {
			break
		}
		row := make([]byte, rowStride)
		copy(row, scanline)
		rows = append(rows, row)
	}
	if len(rows) != outputHeight {
		return nil, fmt.Errorf("decoded %d scanlines, want %d", len(rows), outputHeight)
	}
	return rows, nil
}

func validateUnsupportedOptions(opts *Options) error {
	if opts == nil {
		return nil
	}
	if opts.OutputColorSpace != "" {
		outputColorSpace, err := djpeg.ParseOutputColorSpace(opts.OutputColorSpace)
		if err != nil {
			return err
		}
		if opts.Grayscale && outputColorSpace != djpeg.ColorSpaceUnknown && outputColorSpace != djpeg.ColorSpaceGray {
			return fmt.Errorf("%w: --output-colorspace conflicts with --grayscale", djpeg.ErrInvalidOption)
		}
		if opts.ForceRGB && outputColorSpace != djpeg.ColorSpaceUnknown && outputColorSpace != djpeg.ColorSpaceRGB {
			return fmt.Errorf("%w: --output-colorspace conflicts with --rgb", djpeg.ErrInvalidOption)
		}
	}
	switch {
	case opts.Grayscale && opts.ForceRGB:
		return fmt.Errorf("%w: --grayscale and --rgb cannot be used together", djpeg.ErrInvalidOption)
	case opts.NumColors < 0:
		return fmt.Errorf("%w: --colors must be non-negative", djpeg.ErrInvalidOption)
	default:
		return nil
	}
}

func decoderOptions(opts *Options) ([]djpeg.Option, error) {
	if opts == nil {
		return nil, nil
	}
	decodeOptions := make([]djpeg.Option, 0, 4)
	if opts.Fast {
		decodeOptions = append(decodeOptions, djpeg.WithFast())
	}
	if opts.Compatibility != "" {
		mode, err := djpeg.ParseCompatibilityMode(opts.Compatibility)
		if err != nil {
			return nil, err
		}
		decodeOptions = append(decodeOptions, djpeg.WithCompatibility(mode))
	}
	if opts.Grayscale {
		decodeOptions = append(decodeOptions, djpeg.WithGrayscaleOutput())
	}
	if opts.ForceRGB {
		decodeOptions = append(decodeOptions, djpeg.WithRGBOutput())
	}
	if opts.MaxMemory != "" {
		limit, err := djpeg.ParseMemoryLimit(opts.MaxMemory)
		if err != nil {
			return nil, err
		}
		decodeOptions = append(decodeOptions, djpeg.WithMaxMemory(limit))
	}
	if opts.Scale != "" {
		numerator, denominator, err := djpeg.ParseScale(opts.Scale)
		if err != nil {
			return nil, err
		}
		decodeOptions = append(decodeOptions, djpeg.WithScale(numerator, denominator))
	}
	if opts.DctMethod != "" {
		method, err := djpeg.ParseIDCTMethod(opts.DctMethod)
		if err != nil {
			return nil, err
		}
		decodeOptions = append(decodeOptions, djpeg.WithIDCT(method))
	}
	if opts.NoSmooth {
		decodeOptions = append(decodeOptions, djpeg.WithNoSmooth())
	}
	if opts.TurboFancy {
		decodeOptions = append(decodeOptions, djpeg.WithTurboFancy())
	}
	if opts.InputColorSpace != "" {
		space, err := djpeg.ParseInputColorSpace(opts.InputColorSpace)
		if err != nil {
			return nil, err
		}
		decodeOptions = append(decodeOptions, djpeg.WithInputColorSpace(space))
	}
	if opts.OutputColorSpace != "" {
		space, err := djpeg.ParseOutputColorSpace(opts.OutputColorSpace)
		if err != nil {
			return nil, err
		}
		decodeOptions = append(decodeOptions, djpeg.WithOutputColorSpace(space))
	}
	if opts.ColorTransform != "" {
		transform, err := djpeg.ParseColorTransform(opts.ColorTransform)
		if err != nil {
			return nil, err
		}
		decodeOptions = append(decodeOptions, djpeg.WithColorTransform(transform))
	}
	return decodeOptions, nil
}
