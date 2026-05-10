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
	Palette color.Palette // set for PixelFormatIndexed8
}
```

`Pix` layout depends on `Format`:

- `PixelFormatGray8`: one byte per pixel, row stride is usually `width`
- `PixelFormatRGB24`: R, G, B bytes per pixel, row stride is usually `width*3`
- `PixelFormatIndexed8`: one palette index per pixel; inspect `Raster.Palette`

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
	case libjpeg.PixelFormatIndexed8:
		useIndexedRow(row[:width], raster.Palette)
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
	ImageWidth       int
	ImageHeight      int
	InputComponents  int
	InputColorSpace  ColorSpace
	DataPrecision    int
	MaxHSampFactor   int
	MaxVSampFactor   int
	MinDCTHScaledSize int
	MinDCTVScaledSize int
	BlockSize        int
	ScaleNum         uint
	ScaleDenom       uint
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
	RecOutbufHeight  int
	Quantized        bool
	DesiredNumColors int
	ActualNumColors  int
	RawDataOut       bool
	BufferedImage    bool
	OutputGamma      float64
	DoBlockSmoothing bool
	CCIR601Sampling  bool
	Scan             ScanParameters
}
```

`Width` and `Height` describe the selected output geometry. `ImageWidth` and
`ImageHeight` preserve the original JPEG header dimensions.

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
libjpeg.WithQuantizeColors(256)
libjpeg.WithRawDataOutput()
libjpeg.WithOutputGamma(2.2)
libjpeg.WithBlockSmoothing(false)
```

`WithNoSmooth` maps to CLI `--nosmooth`. `WithFast` maps to the currently
implemented subset of CLI `--fast`: fast integer IDCT plus nearest-neighbor
chroma upsampling.

Force an output color space when callers need libjpeg-style `--grayscale` or
`--rgb` behavior, or when they need a libjpeg null-conversion output such as
YCbCr samples:

```go
raster, err := libjpeg.DecodeRaster(r, libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceGray))
// Equivalent:
raster, err = libjpeg.DecodeRaster(r, libjpeg.WithGrayscaleOutput())

ycbcr, err := libjpeg.DecodeRaster(r, libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceYCbCr))
```

YCbCr and big-gamut YCbCr output are supported only when the JPEG input color
space matches the requested output color space.

`WithMaxMemory` applies an approximate upper bound to decoder-owned compressed
scan data and component buffers. CLI-style values can be parsed with
`libjpeg.ParseMemoryLimit("20m")`; bare numbers are kilobytes, matching libjpeg.

`WithScale` applies libjpeg-style output scaling. For 8x8 DCT JPEGs, ratios map
to the closest supported scale size from `1/8` through `16/8`. The decoder uses
the libjpeg-style scaled-IDCT output path for both raster and raw component
output.

`WithOutputGamma` and `WithBlockSmoothing` mirror libjpeg decompressor
parameters. Progressive scanline output is available through a pure Go fallback
path, including scaled and quantized raster output. Progressive raw component
output is also available for one-shot APIs and seekable low-level decoder
inputs. One-shot APIs buffer non-seekable readers as needed; low-level
`Decoder` progressive output requires a seekable input. Progressive coefficient
decoding is still reported as `ErrUnsupported`.

## Buffered-Image Output Passes

`WithBufferedImage` mirrors libjpeg's `buffered_image` mode at the public
facade level. After `StartDecompress`, call `StartOutput`, read scanlines, and
then call `FinishOutput`. The decoder keeps a baseline image raster so callers
can replay output passes or switch quantized colormaps between passes.

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithBufferedImage())
if _, err := dec.ReadHeader(); err != nil {
	return err
}
if err := dec.StartDecompress(); err != nil {
	return err
}

if ok, err := dec.StartOutput(dec.InputScanNumber()); err != nil || !ok {
	return err
}
row := make([]byte, dec.OutputConfig().Stride)
for dec.OutputScanline() < dec.OutputConfig().Height {
	if _, err := dec.ReadScanlines([][]byte{row}); err != nil {
		return err
	}
	// use row before the next ReadScanlines call
}
if ok, err := dec.FinishOutput(); err != nil || !ok {
	return err
}
return dec.FinishDecompress()
```

This is currently a decoded-raster replay path. Progressive inputs can be
displayed when the source is seekable, but progressive incremental display from
partially decoded scans is not yet available.

## Quantized Output

`WithQuantizeColors` mirrors libjpeg's `quantize_colors` and
`desired_number_of_colors` decompressor parameters. It returns
`PixelFormatIndexed8` with a Go `color.Palette`.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithQuantizeColors(64),
	libjpeg.WithDitherMode(libjpeg.DitherFloydSteinberg),
)
if err != nil {
	return err
}

_ = raster.Palette
```

Generated RGB palettes use libjpeg-style two-pass, image-derived selection by
default. Use `WithQuantizationMode(libjpeg.QuantizationOnePass)` to select the
faster fixed color-cube path used by the `djpeg --onepass` flag. CMYK and YCCK
output use fixed generated 4-component palettes. As in IJG libjpeg, generated
RGB palettes require at least 8 requested colors; external palettes may contain
one or more entries.

For external colormap mode, pass a palette explicitly:

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithColormap(color.Palette{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
	}),
	libjpeg.WithDitherMode(libjpeg.DitherNone),
)
```

`Decoder.NewColormap` mirrors libjpeg's `jpeg_new_colormap()` for the buffered
quantized raster path. It switches to a new external palette and resets
`OutputScanline` to zero so the caller can emit another indexed pass.

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithQuantizeColors(64))
if _, err := dec.ReadHeader(); err != nil {
	return err
}
if err := dec.StartDecompress(); err != nil {
	return err
}

if err := dec.NewColormap(color.Palette{
	color.RGBA{0, 0, 0, 255},
	color.RGBA{255, 255, 255, 255},
}); err != nil {
	return err
}
```

Quantized output currently supports grayscale, RGB, YCbCr, big-gamut YCbCr,
CMYK, and YCCK output. For YCbCr and big-gamut YCbCr indexed rasters,
`Raster.Palette` exposes displayable RGB entries. For YCCK indexed rasters,
`Raster.Palette` exposes the palette as Go `color.CMYK` entries for normal
`image.Image` interoperability.

## Coefficient Output

`DecodeCoefficients` and `Decoder.ReadCoefficients` expose a baseline
`jpeg_read_coefficients()` path. The returned blocks are quantized DCT
coefficients in natural row-major order, before IDCT, color conversion, or
upsampling.

```go
components, cfg, err := libjpeg.DecodeCoefficients(r)
if err != nil {
	return err
}
_ = cfg

for _, component := range components {
	for by := 0; by < component.HeightInBlocks; by++ {
		row := component.Blocks[by*component.WidthInBlocks : (by+1)*component.WidthInBlocks]
		useCoefficientBlocks(component.Component.Index, row)
	}
}
```

The scanline decoder form is:

```go
dec := libjpeg.NewDecoder(r)
if _, err := dec.ReadHeader(); err != nil {
	return err
}
components, err := dec.ReadCoefficients()
if err != nil {
	return err
}
_ = components
```

Progressive coefficient decoding is still reported as `ErrUnsupported`.
Sequential arithmetic-coded coefficient decoding is supported.

## Raw Component Output

`DecodeRawComponents` and `Decoder.ReadRawData` expose libjpeg's
`raw_data_out` / `jpeg_read_raw_data()` path. The returned components are
downsampled planes before color conversion and upsampling.

```go
components, cfg, err := libjpeg.DecodeRawComponents(r)
if err != nil {
	return err
}
_ = cfg.RawDataOut

for _, component := range components {
	for y := 0; y < component.Height; y++ {
		row := component.Pix[y*component.Stride : y*component.Stride+component.Width]
		useRawComponentRow(component.Component.Index, row)
	}
}
```

The scanline form is:

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithRawDataOutput())
if _, err := dec.ReadHeader(); err != nil {
	return err
}
if err := dec.StartDecompress(); err != nil {
	return err
}
components, err := dec.ReadRawData()
if err != nil {
	return err
}
_ = components
return dec.FinishDecompress()
```

To mirror one `jpeg_read_raw_data()` call at a time, use `ReadRawDataRows`.
`RawDataLinesPerIMCURow` returns the required `maxLines` value.

```go
linesPerIMCU := dec.RawDataLinesPerIMCURow()
for dec.OutputScanline() < dec.OutputConfig().Height {
	components, rows, err := dec.ReadRawDataRows(linesPerIMCU)
	if err != nil {
		return err
	}
	if rows == 0 {
		break
	}
	useRawIMCURow(components)
}
```

Raw component output cannot be combined with quantized output. `WithScale` is
supported in raw mode through libjpeg-compatible DCT-scaled output dimensions;
no post-decode resizing is applied. `WithBufferedImage` is supported with raw
output: call `StartOutput` before `ReadRawData` or `ReadRawDataRows`, and call
`FinishOutput` before `FinishDecompress`. A later `StartOutput` replays the
buffered raw component rows.

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

## Mutable Decoder Parameters

`NewDecoder` also exposes libjpeg-style setter methods for callers that prefer
the C API lifecycle: read the header, inspect container metadata, then set
decompression parameters before `StartDecompress`.

```go
dec := libjpeg.NewDecoder(r)
header, err := dec.ReadHeader()
if err != nil {
	return err
}

if pdfColorSpaceIsRGB(header) {
	if err := dec.SetInputColorSpace(libjpeg.InputRGB); err != nil {
		return err
	}
}
if err := dec.SetOutputColorSpace(libjpeg.ColorSpaceRGB); err != nil {
	return err
}
if err := dec.SetCompatibility(libjpeg.CompatibilityPopplerPDF); err != nil {
	return err
}

if err := dec.StartDecompress(); err != nil {
	return err
}
```

The mutable methods currently include `SetIDCT`, `SetUpsampling`,
`SetCompatibility`, `SetChromaIDCTScaling`, `SetInputColorSpace`,
`SetOutputColorSpace`, `SetColorTransform`, `SetScale`, `SetRawDataOutput`,
`SetBufferedImage`, `SetMaxMemory`, `SetOutputGamma`, `SetBlockSmoothing`,
`SetQuantizeColors`, `SetDitherMode`, and `SetColormap`. They must be called
before `StartDecompress`; changing these parameters after output starts returns
`ErrInvalidOption`.

`SetProgressMonitor` can be used to install or replace the progress callback
for later read calls:

```go
dec.SetProgressMonitor(func(p libjpeg.Progress) {
	_ = p.PassCounter
	_ = p.PassLimit
})
```

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
- `CCIR601Sampling` exposes the JFIF extension sampling flag.
- `Scan` exposes the current SOS/per-scan fields: component count, MCU
  geometry, `Ss`, `Se`, `Ah`, `Al`, and the derived limiting spectral end.

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
_ = dec.CCIR601Sampling()
_ = dec.ScanParameters()
```

`Abort` stops the current decompression operation and clears the public decoder
state. To decode another stream, create a new decoder with a new reader.

```go
dec.Abort()
```

## Saved Markers

The scanline decoder can retain APPn or COM marker payloads while reading the
header. This mirrors libjpeg's `jpeg_save_markers()` lifecycle: configure marker
retention before `ReadHeader`, then inspect `Markers()`. Use either
`WithSavedMarkers` when constructing the decoder or `Decoder.SaveMarkers` before
`ReadHeader`.

```go
dec := libjpeg.NewDecoder(r)
if err := dec.SaveMarkers(libjpeg.MarkerAPP14, 65533); err != nil {
	return err
}
if err := dec.SaveMarkers(libjpeg.MarkerCOM, 65533); err != nil {
	return err
}

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

## Marker Processors

`WithMarkerProcessor` and `Decoder.SetMarkerProcessor` mirror libjpeg's
`jpeg_set_marker_processor()` lifecycle for APPn and COM markers. Configure the
processor before `ReadHeader`; the callback receives a copy of the marker
payload and may return an error to stop header parsing.

```go
dec := libjpeg.NewDecoder(
	r,
	libjpeg.WithMarkerProcessor(libjpeg.MarkerAPP2, func(marker libjpeg.Marker) error {
		_ = marker.Code
		_ = marker.OriginalLength
		_ = marker.Data
		return nil
	}),
)

if _, err := dec.ReadHeader(); err != nil {
	return err
}
```

If a marker processor and saved-marker retention are both configured for the
same marker code, the processor takes precedence.

## Tables

After `ReadHeader`, callers can inspect parsed DQT and DHT tables through
copy-returning helpers. This mirrors the libjpeg decompressor fields
`quant_tbl_ptrs`, `dc_huff_tbl_ptrs`, and `ac_huff_tbl_ptrs` without exposing
mutable decoder-owned storage.

```go
dec := libjpeg.NewDecoder(r)
if _, err := dec.ReadHeader(); err != nil {
	return err
}

qt, ok := dec.QuantizationTable(0)
if ok {
	_ = qt.Values
}

dc, ok := dec.HuffmanTable(0, libjpeg.HuffmanTableDC)
if ok {
	_ = dc.Bits
	_ = dc.Values
}

for _, component := range dec.Components() {
	_ = component.ID
	_ = component.HSampFactor
	_ = component.VSampFactor
	_ = component.QuantizationTableIndex
}

_ = dec.RestartInterval()

arith, ok := dec.ArithmeticConditioningTable(0)
if ok {
	_ = arith.DCLower
	_ = arith.DCUpper
	_ = arith.ACK
}
_ = dec.ArithmeticConditioningTables()
```

Arithmetic conditioning tables mirror libjpeg's `arith_dc_L`, `arith_dc_U`,
and `arith_ac_K` arrays for table selectors 0 through 15. Defaults are exposed
even when the stream does not carry DAC markers, and DAC overrides are visible
after header parsing.

For table-only streams, call `ReadHeaderRequireImage(false)`. A
`HeaderTablesOnly` status leaves the image config empty, but quantization and
Huffman tables parsed before EOI remain available through the table helpers,
matching libjpeg's permanent-table behavior for abbreviated streams.

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
	QuantizeColors:    true,
	DesiredNumColors:  64,
	DitherMode:        libjpeg.DitherFloydSteinberg,
	QuantizationMode:  libjpeg.QuantizationDefault,
	RawDataOut:        false,
	OutputGamma:       1.0,
	BlockSmoothing:    libjpeg.BlockSmoothingDefault,
	ProgressMonitor:   nil,
}

raster, err := libjpeg.DecodeRasterWithOptions(r, opts)
```

CLI-style strings can be parsed with:

```go
idct, err := libjpeg.ParseIDCTMethod("int")
space, err := libjpeg.ParseInputColorSpace("rgb")
dither, err := libjpeg.ParseDitherMode("fs")
```

`ParseInputColorSpace` also accepts libjpeg 9 big-gamut aliases such as
`big-gamut-rgb` and `big-gamut-ycbcr`.

## Progress Monitor

`WithProgressMonitor` mirrors libjpeg's `jpeg_progress_mgr` counters at the
facade level. The callback is invoked during scanline, raw-data, and
coefficient reads.

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithProgressMonitor(func(p libjpeg.Progress) {
	_ = p.PassCounter
	_ = p.PassLimit
	_ = p.CompletedPasses
	_ = p.TotalPasses
}))
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

// Equivalent low-level form when the caller needs the libjpeg return code:
header, status, err := dec.ReadHeaderRequireImage(true)
if err != nil {
	return err
}
_ = status

dimensions, err := dec.CalcOutputDimensions()
if err != nil {
	return err
}
_ = dimensions.RecOutbufHeight

if err := dec.Start(); err != nil {
	return err
}

xOffset, width, err := dec.CropScanline(0, header.Width)
if err != nil {
	return err
}
_ = xOffset
_ = width

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

Advanced callers can drive marker input explicitly with `ConsumeInput`, which
mirrors libjpeg's `jpeg_consume_input()` return codes.

```go
status, err := dec.ConsumeInput()
if err != nil {
	return err
}
if status == libjpeg.InputReachedSOS {
	_ = dec.Header()
}
```

`CalcOutputDimensions` mirrors libjpeg's `jpeg_calc_output_dimensions()`.
Call it after `ReadHeader` if you need the final output geometry before
starting decompression.

`SkipScanlines` mirrors libjpeg's `jpeg_skip_scanlines()`: it advances the
output scanline cursor, stops at the bottom of the image, and returns the
number of rows actually skipped.

`CropScanline` mirrors libjpeg's `jpeg_crop_scanline()` at the facade level.
Call it after `Start` and before reading or skipping any rows. The returned
offset and width are the actual crop region; width is clamped to the right edge
of the output image.

Read all expected output rows before calling `Finish`. Stopping early can
surface a "too little data" decompressor error because the decoder expects the
full image to be consumed.

This API is scanline-oriented but not a bounded-memory streaming contract. It
exists so callers can avoid building a second full output image while writing
to another format. `WithScale` is applied by the decoder before scanlines are
served.

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

Currently unsupported JPEG features include progressive coefficient decoding.
Progressive headers can be inspected through `ReadHeader` and
`DecodeRasterConfig`; progressive scanlines and raw component output are
available through one-shot APIs and seekable low-level decoder inputs.

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
| `--output-colorspace ycbcr` | `libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceYCbCr)` |
| `--maxmemory 20m` | `limit, _ := libjpeg.ParseMemoryLimit("20m"); libjpeg.WithMaxMemory(limit)` |
| `--scale 1/2` | `libjpeg.WithScale(1, 2)` |
| `--colors 64` | `libjpeg.WithQuantizeColors(64)` |
| `--dither fs` | `libjpeg.WithDitherMode(libjpeg.DitherFloydSteinberg)` |
| `--onepass` | `libjpeg.WithQuantizationMode(libjpeg.QuantizationOnePass)` |
| `--compatibility poppler-pdf` | `libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF)` |
| `--turbo-fancy` | `libjpeg.WithTurboFancy()` deprecated alias |
| `--input-colorspace rgb` | `libjpeg.WithInputColorSpace(libjpeg.InputRGB)` |
| `--color-transform subtract-green` | `libjpeg.WithColorTransform(libjpeg.ColorTransformSubtractGreen)` |
| `--input-colorspace ycbcr` | `libjpeg.WithInputColorSpace(libjpeg.InputYCbCr)` |
| `--input-colorspace grayscale` | `libjpeg.WithInputColorSpace(libjpeg.InputGray)` |
| `--input-colorspace big-gamut-rgb` | `libjpeg.WithInputColorSpace(libjpeg.InputBigGamutRGB)` |
| `--input-colorspace big-gamut-ycbcr` | `libjpeg.WithInputColorSpace(libjpeg.InputBigGamutYCbCr)` |

CLI output flags such as `--ppm`, `--bmp`, `--os2`, `--targa`, `--rle`,
`--gif`, and `--gif0` are not part of the public decode facade yet. Use
`DecodeRaster` and write your desired output format in application code.

The `cmd/djpeg` output writers currently accept Gray8 and RGB24 rasters. Other
`--output-colorspace` values are exposed for parity/debugging and library API
coverage, but may require `DecodeRaster` until matching command writers exist.

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

- Baseline and extended sequential non-progressive JPEGs are the supported
  paths, including Huffman and arithmetic entropy coding.
- Progressive JPEG scanlines and raw component output can be decoded by
  one-shot APIs and seekable low-level decoder inputs. Progressive coefficient
  decoding returns `ErrUnsupported`.
- `Decode` returns a `*Raster` behind the `image.Image` interface. This keeps
  raw bytes available without forcing an RGBA allocation.
- The package does not call `image.RegisterFormat` automatically. Use
  `libjpeg.Decode` explicitly when you need this decoder's parity behavior.
- The default option set is chosen to preserve IJG 9f exact parity for the
  current random100 corpus.
