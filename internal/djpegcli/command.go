// Package djpegcli implements the djpeg command-line interface.
package djpegcli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"

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
	OutFile         string `flag:"outfile" usage:"Write output to NAME"`
	Verbose         bool   `flag:"verbose" usage:"Verbose output"`
	Grayscale       bool   `flag:"grayscale" usage:"Force grayscale output"`
	ForceRGB        bool   `flag:"rgb" usage:"Force RGB output"`
	NumColors       int    `flag:"colors" usage:"Reduce image to no more than N colors"`
	Fast            bool   `flag:"fast" usage:"Fast, low-quality processing"`
	NoSmooth        bool   `flag:"nosmooth" usage:"Don't use high-quality upsampling"`
	OnePass         bool   `flag:"onepass" usage:"Use 1-pass quantization"`
	DctMethod       string `flag:"dct" usage:"IDCT method: int, fast, float"`
	DitherMode      string `flag:"dither" usage:"Dithering: fs, none, ordered"`
	MapFile         string `flag:"map" usage:"Map to colors from a GIF/PPM file"`
	MaxMemory       string `flag:"maxmemory" usage:"Maximum memory (KB or MB with m)"`
	Scale           string `flag:"scale" usage:"Scale output image by fraction M/N"`
	InputColorSpace string `flag:"input-colorspace" usage:"Interpret JPEG samples as auto, rgb, ycbcr, or grayscale"`
	TurboFancy      bool   `flag:"turbo-fancy" usage:"Use 8x8 IDCT plus fancy upsampling instead of IJG 9f chroma IDCT scaling"`
	CPUProfile      string `flag:"cpuprofile" usage:"Write CPU profile to FILE"`

	FmtPPM   bool `flag:"ppm" usage:"Output PPM/PGM format"`
	FmtBMP   bool `flag:"bmp" usage:"Output BMP format"`
	FmtGIF   bool `flag:"gif" usage:"Output GIF format"`
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
		String("input-colorspace", "auto", "Interpret JPEG samples as auto, rgb, ycbcr, or grayscale").
		Bool("turbo-fancy", false, "Use 8x8 IDCT plus fancy upsampling instead of IJG 9f chroma IDCT scaling").
		String("cpuprofile", "", "Write CPU profile to FILE").
		Bool("ppm", false, "Output PPM/PGM format").
		Bool("bmp", false, "Output BMP format").
		Bool("gif", false, "Output GIF format").
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

			// Determine output format
			opts.Format = output.FormatPPM // default
			if opts.FmtBMP {
				opts.Format = output.FormatBMP
			}
			if opts.FmtGIF {
				opts.Format = output.FormatGIF
			}
			if opts.FmtTarga {
				opts.Format = output.FormatTarga
			}
			if opts.FmtRLE {
				opts.Format = output.FormatRLE
			}
			if opts.FmtPPM {
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

func parseMaxMemory(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.ToLower(s)
	var mult int64 = 1024 // Default is KB in standard libjpeg unless 'm' suffix
	if strings.HasSuffix(s, "m") {
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	} else if strings.HasSuffix(s, "k") {
		mult = 1024
		s = strings.TrimSuffix(s, "k")
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val * mult
}

// Decompress reads a JPEG from input and writes the decoded image to out
// in the requested format.
func Decompress(input io.Reader, out io.Writer, opts *Options) error {
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

	// Apply user-specified overrides (must be after ReadHeader)
	if opts.MaxMemory != "" {
		limit := parseMaxMemory(opts.MaxMemory)
		if limit > 0 {
			// dec.SetMaxMemory(limit)
		}
	}

	if opts.Grayscale {
		// Force grayscale output by setting out color space
	}
	if opts.ForceRGB {
		// Force RGB output
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
	colorSpace := output.ColorSpaceRGB
	if outputConfig.PixelFormat == djpeg.PixelFormatGray8 {
		colorSpace = output.ColorSpaceGrayscale
	}

	info := &output.ImageInfo{
		Width:         outputWidth,
		Height:        outputHeight,
		NumComponents: outputComponents,
		ColorSpace:    colorSpace,
		DataPrecision: 8,
	}

	if opts.NumColors > 0 {
		info.QuantizeColors = true
		info.DesiredColors = opts.NumColors
	}
	if colormap != nil {
		info.QuantizeColors = true
		info.Colormap = colormap
		info.DesiredColors = colormap.NumColors
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

	// For GIF with RGB, force quantization check
	if opts.Format == output.FormatGIF && info.ColorSpace == output.ColorSpaceRGB && !info.QuantizeColors {
		return fmt.Errorf("GIF format requires color quantization for RGB images; use -colors N")
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

func decoderOptions(opts *Options) ([]djpeg.Option, error) {
	if opts == nil {
		return nil, nil
	}
	decodeOptions := make([]djpeg.Option, 0, 4)
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
	return decodeOptions, nil
}
