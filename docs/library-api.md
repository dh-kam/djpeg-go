# Public Library API

This guide describes the public facade exposed by the root module:

```go
import djpeg "github.com/dh-kam/djpeg-go"
```

The public API is intentionally small. It hides `internal/*` packages and
provides stable entry points for external Go modules that need JPEG decode
behavior close to IJG `djpeg`.

## Choosing an API

Use `Decode` when you want an `image.Image` and standard Go image
interoperability:

```go
img, err := djpeg.Decode(r)
```

Use `DecodeRaster` when exact byte layout matters. This is the best fit for
renderers, pixel comparisons, and applications that need direct Gray8 or RGB24
data:

```go
raster, err := djpeg.DecodeRaster(r)
```

Use `NewDecoder` when you want to pull scanlines yourself. This API is useful
for format writers and transcoders, but it does not promise bounded-memory
streaming. The current decoder reads the compressed scan payload internally
before serving scanlines.

```go
dec := djpeg.NewDecoder(r)
```

## Decode to image.Image

`Decode` returns an `image.Image`. The concrete image currently returned is a
`*djpeg.Raster`, which implements `image.Image`.

```go
package main

import (
	"image/png"
	"os"

	djpeg "github.com/dh-kam/djpeg-go"
)

func main() {
	in, err := os.Open("input.jpg")
	if err != nil {
		panic(err)
	}
	defer in.Close()

	img, err := djpeg.Decode(in)
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
raster, err := djpeg.DecodeRaster(r)
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
raster, err := djpeg.DecodeRaster(r, djpeg.WithIDCT(djpeg.IDCTInt))
if err != nil {
	return err
}

width := raster.Rect.Dx()
height := raster.Rect.Dy()

for y := 0; y < height; y++ {
	row := raster.Pix[y*raster.Stride:]
	switch raster.Format {
	case djpeg.PixelFormatGray8:
		useGrayRow(row[:width])
	case djpeg.PixelFormatRGB24:
		useRGBRow(row[:width*3])
	}
}
```

## Read Metadata

Use `DecodeConfig` for standard Go image metadata:

```go
cfg, err := djpeg.DecodeConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.ColorModel)
```

Use `DecodeRasterConfig` when you also need raster details:

```go
cfg, err := djpeg.DecodeRasterConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.PixelFormat, cfg.Stride)
```

`Config` exposes:

```go
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
```

## Options

Most callers should use functional options. The zero value matches the CLI
default: integer IDCT, fancy upsampling enabled, and IJG-compatible chroma
IDCT scaling.

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithNoSmooth(),
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
djpeg.WithNoSmooth()
```

This maps to CLI `--nosmooth`.

## Compatibility Modes

The default compatibility mode preserves the current IJG 9f exact-parity path.

```go
raster, err := djpeg.DecodeRaster(r)
```

For PDF DCT streams that should match Poppler or ImageMagick style output, use
`WithTurboFancy`:

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithTurboFancy(),
)
```

This maps to CLI `--turbo-fancy`. It uses 8x8 chroma IDCT plus fancy upsampling
instead of IJG 9f chroma IDCT scaling. Keep it opt-in because changing the
default would break IJG 9f exact parity.

Equivalent enum form:

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithCompatibility(djpeg.CompatibilityPopplerPDF),
)
```

## Input Color Space Overrides

JPEG streams embedded in containers such as PDF can have color space metadata
outside the JPEG byte stream. Use `WithInputColorSpace` when the container
knows the intended sample interpretation.

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithInputColorSpace(djpeg.InputRGB),
	djpeg.WithTurboFancy(),
)
```

Available input color spaces:

- `InputAuto`
- `InputGray`
- `InputRGB`
- `InputYCbCr`

The default is `InputAuto`, which uses the JPEG markers and component IDs.

## Options Struct

For code that stores settings, use `Options` and `DecodeWithOptions` or
`DecodeRasterWithOptions`:

```go
opts := &djpeg.Options{
	IDCT:            djpeg.IDCTInt,
	Upsampling:      djpeg.UpsamplingFancy,
	Compatibility:   djpeg.CompatibilityIJG9,
	InputColorSpace: djpeg.InputAuto,
}

raster, err := djpeg.DecodeRasterWithOptions(r, opts)
```

CLI-style strings can be parsed with:

```go
idct, err := djpeg.ParseIDCTMethod("int")
space, err := djpeg.ParseInputColorSpace("rgb")
```

## Scanline API

`NewDecoder` exposes a scanline-oriented lifecycle:

```go
dec := djpeg.NewDecoder(
	r,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithNoSmooth(),
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
for y := 0; y < out.Height; y++ {
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

Read all expected output rows before calling `Finish`. Stopping early can
surface a "too little data" decompressor error because the decoder expects the
full image to be consumed.

This API is scanline-oriented but not a bounded-memory streaming contract. It
exists so callers can avoid building a second full output image while writing
to another format.

## Error Handling

The public package exposes stable sentinel errors:

```go
var (
	ErrInvalidJPEG   = errors.New("djpeg: invalid jpeg")
	ErrUnsupported   = errors.New("djpeg: unsupported jpeg feature")
	ErrInvalidOption = errors.New("djpeg: invalid option")
)
```

Use `errors.Is`:

```go
img, err := djpeg.Decode(r)
if err != nil {
	switch {
	case errors.Is(err, djpeg.ErrUnsupported):
		return fmt.Errorf("JPEG feature is not supported: %w", err)
	case errors.Is(err, djpeg.ErrInvalidJPEG):
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
| `--dct int` | `djpeg.WithIDCT(djpeg.IDCTInt)` |
| `--dct fast` | `djpeg.WithIDCT(djpeg.IDCTFast)` |
| `--dct float` | `djpeg.WithIDCT(djpeg.IDCTFloat)` |
| `--nosmooth` | `djpeg.WithNoSmooth()` |
| `--turbo-fancy` | `djpeg.WithTurboFancy()` |
| `--input-colorspace rgb` | `djpeg.WithInputColorSpace(djpeg.InputRGB)` |
| `--input-colorspace ycbcr` | `djpeg.WithInputColorSpace(djpeg.InputYCbCr)` |
| `--input-colorspace grayscale` | `djpeg.WithInputColorSpace(djpeg.InputGray)` |

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
import djpeg "github.com/dh-kam/djpeg-go"
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
  `djpeg.Decode` explicitly when you need this decoder's parity behavior.
- The default option set is chosen to preserve IJG 9f exact parity for the
  current random100 corpus.
