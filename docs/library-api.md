# Public Library API

This guide describes the public facade exposed by the root module:

```go
import libjpeg "github.com/dh-kam/djpeg-go"
```

The public API is intentionally small. It hides `internal/*` packages and
provides stable entry points for external Go modules that need libjpeg-style
JPEG decompressor behavior. The `cmd/djpeg` command is a frontend and parity
tool, not the primary library contract.

## Choosing an API

Use `Decode` when you want an `image.Image` and standard Go image
interoperability:

```go
img, err := libjpeg.Decode(r)
```

Use `DecodeRaster` when exact byte layout matters. This is the best fit for
renderers, pixel comparisons, and applications that need direct Gray8 or RGB24
data:

```go
raster, err := libjpeg.DecodeRaster(r)
```

Use `NewDecoder` when you want to pull scanlines yourself. This API is useful
for format writers and transcoders, but it does not promise bounded-memory
streaming. The current decoder reads the compressed scan payload internally
before serving scanlines.

```go
dec := libjpeg.NewDecoder(r)
```

## Decode to image.Image

`Decode` returns an `image.Image`. The concrete image currently returned is a
`*Raster`, which implements `image.Image`.

```go
package main

import (
	"image/png"
	"os"

	libjpeg "github.com/dh-kam/djpeg-go"
)

func main() {
	in, err := os.Open("input.jpg")
	if err != nil {
		panic(err)
	}
	defer in.Close()

	img, err := libjpeg.Decode(in)
	if err != nil {
		panic(err)
	}

	out, err := os.Create("output.png")
	if err != nil {
		panic(err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		panic(err)
	}
}
```

If you need a standard `*image.RGBA`, decode as a raster and call `RGBA`:

```go
raster, err := libjpeg.DecodeRaster(r)
if err != nil {
	return err
}
rgba := raster.RGBA()
```

## Decode to Raw Pixels

`DecodeRaster` returns a top-down raster:

```go
type Raster struct {
	Pix    []byte
	Stride int
	Rect   image.Rectangle
	Format PixelFormat
}
```

`Pix` layout depends on `Format`:

- `PixelFormatGray8`: one byte per pixel, row stride is usually `width`
- `PixelFormatRGB24`: R, G, B bytes per pixel, row stride is usually `width*3`

Always use `Stride` to walk rows instead of assuming tightly packed rows.

```go
raster, err := libjpeg.DecodeRaster(r, libjpeg.WithIDCT(libjpeg.IDCTInt))
if err != nil {
	return err
}

width := raster.Rect.Dx()
height := raster.Rect.Dy()

for y := 0; y < height; y++ {
	row := raster.Pix[y*raster.Stride:]
	switch raster.Format {
	case libjpeg.PixelFormatGray8:
		useGrayRow(row[:width])
	case libjpeg.PixelFormatRGB24:
		useRGBRow(row[:width*3])
	}
}
```

## Read Metadata

Use `DecodeConfig` for standard Go image metadata:

```go
cfg, err := libjpeg.DecodeConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.ColorModel)
```

Use `DecodeRasterConfig` when you also need raster details:

```go
cfg, err := libjpeg.DecodeRasterConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.PixelFormat, cfg.Stride)
```

`Config` exposes:

```go
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
```

## Options

Most callers should use functional options. The zero value matches the CLI
default: integer IDCT, fancy upsampling enabled, and IJG-compatible chroma
IDCT scaling.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithIDCT(libjpeg.IDCTInt),
	libjpeg.WithNoSmooth(),
)
```

Available IDCT methods:

- `IDCTDefault`
- `IDCTInt`
- `IDCTFast`
- `IDCTFloat`

Available upsampling modes:

- `UpsamplingDefault`
- `UpsamplingFancy`
- `UpsamplingNearest`

Convenience option:

```go
libjpeg.WithNoSmooth()
libjpeg.WithFast()
libjpeg.WithGrayscaleOutput()
libjpeg.WithRGBOutput()
libjpeg.WithMaxMemory(20_000_000)
libjpeg.WithScale(1, 2)
```

`WithNoSmooth` maps to CLI `--nosmooth`. `WithFast` maps to the currently
implemented subset of CLI `--fast`: fast integer IDCT plus nearest-neighbor
chroma upsampling.

Force an output color space when callers need libjpeg-style `--grayscale` or
`--rgb` behavior:

```go
raster, err := libjpeg.DecodeRaster(r, libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceGray))
// Equivalent:
raster, err = libjpeg.DecodeRaster(r, libjpeg.WithGrayscaleOutput())
```

`WithMaxMemory` applies an approximate upper bound to decoder-owned compressed
scan data and component buffers. CLI-style values can be parsed with
`libjpeg.ParseMemoryLimit("20m")`; bare numbers are kilobytes, matching libjpeg.

`WithScale` applies libjpeg-style output scaling. For 8x8 DCT JPEGs, ratios map
to the closest supported scale size from `1/8` through `16/8`. The current
implementation decodes first and then resamples the output raster.

## Compatibility Modes

The default compatibility mode preserves the current IJG 9f exact-parity path.

```go
raster, err := libjpeg.DecodeRaster(r)
```

For PDF DCT streams that should match Poppler or ImageMagick style output, use
the Poppler compatibility preset:

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
)
```

This uses the libjpeg-style integer IDCT default, 8x8 chroma IDCT, and fancy
upsampling for the PDF fixture class that matches Poppler/ImageMagick output.
Keep it opt-in because changing the default would break IJG 9f exact parity.

The CLI/debug equivalent is:

```bash
djpeg --compatibility poppler-pdf --ppm input.jpg > output.ppm
```

`WithTurboFancy()` and `--turbo-fancy` remain as deprecated aliases:

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithIDCT(libjpeg.IDCTInt),
	libjpeg.WithTurboFancy(),
)
```

Advanced callers can override just the chroma IDCT scaling part of the profile:

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
	libjpeg.WithChromaIDCTScaling(true),
)
```

RGB-style streams can also override the inverse color transform:

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithInputColorSpace(libjpeg.InputRGB),
	libjpeg.WithColorTransform(libjpeg.ColorTransformSubtractGreen),
)
```

## Input Color Space Overrides

JPEG streams embedded in containers such as PDF can have color space metadata
outside the JPEG byte stream. Use `WithInputColorSpace` when the container
knows the intended sample interpretation.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithInputColorSpace(libjpeg.InputRGB),
	libjpeg.WithTurboFancy(),
)
```

Available input color spaces:

- `InputAuto`
- `InputGray`
- `InputRGB`
- `InputYCbCr`
- `InputCMYK`
- `InputYCCK`

The default is `InputAuto`, which uses the JPEG markers and component IDs.

Four-component PDF streams can request raw CMYK output:

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithInputColorSpace(libjpeg.InputYCCK),
	libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceCMYK),
)
// raster.Format is PixelFormatCMYK32.
```

For IJG compatibility, component IDs take precedence over Adobe APP14
transform hints. If the component IDs are ambiguous, Adobe transform `0`
infers CMYK for four-component streams, while transform `2` and unknown
four-component transform values infer YCCK. Explicit `WithInputColorSpace`
always overrides that marker inference.

## Header State

`Config` includes decompressor header state that maps to libjpeg fields and
helper APIs:

- `Baseline`, `Progressive`, and `Arithmetic` describe the SOF coding mode.
- `HasMultipleScans` mirrors `jpeg_has_multiple_scans()`.
- `InputComplete` mirrors `jpeg_input_complete()` at the time the config was
  produced.
- `SawJFIFMarker`, `JFIFMajorVersion`, `JFIFMinorVersion`, `DensityUnit`,
  `XDensity`, and `YDensity` expose parsed JFIF APP0 metadata.
- `SawAdobeMarker` and `AdobeTransform` expose parsed Adobe APP14 metadata.

The scanline decoder exposes the same state directly:

```go
dec := libjpeg.NewDecoder(r)
cfg, err := dec.ReadHeader()
if err != nil {
	return err
}

_ = cfg.SawAdobeMarker
_ = dec.HasMultipleScans()
_ = dec.InputComplete()
_ = dec.IsBaseline()
_ = dec.IsProgressive()
_ = dec.IsArithmetic()
```

`Abort` stops the current decompression operation and clears the public decoder
state. To decode another stream, create a new decoder with a new reader.

```go
dec.Abort()
```

## Saved Markers

The scanline decoder can retain APPn or COM marker payloads while reading the
header. This mirrors libjpeg's `jpeg_save_markers()` lifecycle: configure marker
retention before `ReadHeader`, then inspect `Markers()`.

```go
dec := libjpeg.NewDecoder(
	r,
	libjpeg.WithSavedMarkers(libjpeg.MarkerAPP14, 65533),
	libjpeg.WithSavedMarkers(libjpeg.MarkerCOM, 65533),
)

if _, err := dec.ReadHeader(); err != nil {
	return err
}

for _, marker := range dec.Markers() {
	_ = marker.Code
	_ = marker.OriginalLength
	_ = marker.Data
}
```

Only APP0 through APP15 and COM markers are accepted. APP0 and APP14 are still
parsed internally for JFIF/Adobe behavior when saving is disabled.

## Options Struct

For code that stores settings, use `Options` and `DecodeWithOptions` or
`DecodeRasterWithOptions`:

```go
opts := &libjpeg.Options{
	IDCT:              libjpeg.IDCTInt,
	Upsampling:        libjpeg.UpsamplingFancy,
	Compatibility:     libjpeg.CompatibilityIJG9,
	InputColorSpace:   libjpeg.InputAuto,
	OutputColorSpace:  libjpeg.ColorSpaceUnknown,
	ColorTransform:    libjpeg.ColorTransformDefault,
	ChromaIDCTScaling: libjpeg.ChromaIDCTScalingDefault,
}

raster, err := libjpeg.DecodeRasterWithOptions(r, opts)
```

CLI-style strings can be parsed with:

```go
idct, err := libjpeg.ParseIDCTMethod("int")
space, err := libjpeg.ParseInputColorSpace("rgb")
```

## Scanline API

`NewDecoder` exposes a scanline-oriented lifecycle:

```go
dec := libjpeg.NewDecoder(
	r,
	libjpeg.WithIDCT(libjpeg.IDCTInt),
	libjpeg.WithNoSmooth(),
)

header, err := dec.ReadHeader()
if err != nil {
	return err
}
fmt.Println(header.Width, header.Height)

if err := dec.Start(); err != nil {
	return err
}

out := dec.OutputConfig()
row := make([]byte, out.Stride)
if _, err := dec.SkipScanlines(10); err != nil {
	return err
}
for dec.OutputScanline() < out.Height {
	n, err := dec.ReadScanlines([][]byte{row})
	if err != nil {
		return err
	}
	if n == 0 {
		break
	}
	writeRow(row[:out.Stride])
}

if err := dec.Finish(); err != nil {
	return err
}
```

`SkipScanlines` mirrors libjpeg's `jpeg_skip_scanlines()`: it advances the
output scanline cursor, stops at the bottom of the image, and returns the
number of rows actually skipped.

Read all expected output rows before calling `Finish`. Stopping early can
surface a "too little data" decompressor error because the decoder expects the
full image to be consumed.

This API is scanline-oriented but not a bounded-memory streaming contract. It
exists so callers can avoid building a second full output image while writing
to another format. `WithScale` currently buffers and resamples the full output
raster before serving scaled scanlines.

## Error Handling

The public package exposes stable sentinel errors:

```go
var (
	ErrInvalidJPEG   = errors.New("djpeg: invalid jpeg")
	ErrUnsupported   = errors.New("djpeg: unsupported jpeg feature")
	ErrInvalidOption = errors.New("djpeg: invalid option")
	ErrMemoryLimit   = errors.New("djpeg: memory limit exceeded")
)
```

Use `errors.Is`:

```go
img, err := libjpeg.Decode(r)
if err != nil {
	switch {
	case errors.Is(err, libjpeg.ErrUnsupported):
		return fmt.Errorf("JPEG feature is not supported: %w", err)
	case errors.Is(err, libjpeg.ErrMemoryLimit):
		return fmt.Errorf("JPEG decode exceeded the memory limit: %w", err)
	case errors.Is(err, libjpeg.ErrInvalidJPEG):
		return fmt.Errorf("invalid JPEG data: %w", err)
	default:
		return err
	}
}
_ = img
```

Currently unsupported JPEG features include progressive JPEG and arithmetic
coding.

## Mapping CLI Flags to API Options

| CLI flag | Public API |
| --- | --- |
| `--dct int` | `libjpeg.WithIDCT(libjpeg.IDCTInt)` |
| `--dct fast` | `libjpeg.WithIDCT(libjpeg.IDCTFast)` |
| `--dct float` | `libjpeg.WithIDCT(libjpeg.IDCTFloat)` |
| `--fast` | `libjpeg.WithFast()` |
| `--nosmooth` | `libjpeg.WithNoSmooth()` |
| `--grayscale` | `libjpeg.WithGrayscaleOutput()` |
| `--rgb` | `libjpeg.WithRGBOutput()` |
| `--maxmemory 20m` | `limit, _ := libjpeg.ParseMemoryLimit("20m"); libjpeg.WithMaxMemory(limit)` |
| `--scale 1/2` | `libjpeg.WithScale(1, 2)` |
| `--compatibility poppler-pdf` | `libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF)` |
| `--turbo-fancy` | `libjpeg.WithTurboFancy()` deprecated alias |
| `--input-colorspace rgb` | `libjpeg.WithInputColorSpace(libjpeg.InputRGB)` |
| `--color-transform subtract-green` | `libjpeg.WithColorTransform(libjpeg.ColorTransformSubtractGreen)` |
| `--input-colorspace ycbcr` | `libjpeg.WithInputColorSpace(libjpeg.InputYCbCr)` |
| `--input-colorspace grayscale` | `libjpeg.WithInputColorSpace(libjpeg.InputGray)` |

CLI output flags such as `--ppm`, `--bmp`, `--targa`, `--rle`, and `--gif` are
not part of the public decode facade yet. Use `DecodeRaster` and write your
desired output format in application code.

## External Module Setup

In another module:

```bash
go get github.com/dh-kam/djpeg-go
```

Then import the root package:

```go
import libjpeg "github.com/dh-kam/djpeg-go"
```

Do not import `github.com/dh-kam/djpeg-go/internal/...`. Those packages are
implementation details and are intentionally blocked by Go's `internal`
package rules for external consumers.

## Compatibility Notes

- Baseline, non-progressive JPEG is the supported path.
- Progressive and arithmetic-coded JPEGs return `ErrUnsupported`.
- `Decode` returns a `*Raster` behind the `image.Image` interface. This keeps
  raw bytes available without forcing an RGBA allocation.
- The package does not call `image.RegisterFormat` automatically. Use
  `libjpeg.Decode` explicitly when you need this decoder's parity behavior.
- The default option set is chosen to preserve IJG 9f exact parity for the
  current random100 corpus.
