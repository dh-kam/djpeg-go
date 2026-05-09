# Examples

This document shows common `djpeg-go` CLI workflows. The examples assume a
Linux amd64 debug build at `./dist/djpeg-linux-amd64-debug`.

```bash
make build
```

## Go Library API

Import the root module when another Go module needs to decode JPEG data:

```go
import libjpeg "github.com/dh-kam/djpeg-go"
```

Decode to an `image.Image` for use with the Go image ecosystem:

```go
in, err := os.Open("input.jpg")
if err != nil {
	return err
}
defer in.Close()

img, err := libjpeg.Decode(in)
if err != nil {
	return err
}

return png.Encode(out, img)
```

Decode to raw pixels when you need stable Gray8 or RGB24 byte layout:

```go
raster, err := libjpeg.DecodeRaster(
	in,
	libjpeg.WithIDCT(libjpeg.IDCTInt),
)
if err != nil {
	return err
}

switch raster.Format {
case libjpeg.PixelFormatGray8:
	useGray(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
case libjpeg.PixelFormatRGB24:
	useRGB(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
}
```

Use the Poppler/ImageMagick-compatible decompressor preset from Go:

```go
raster, err := libjpeg.DecodeRaster(
	in,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
)
```

The public API intentionally does not expose `internal/*` packages. Those
packages remain implementation details for the port.

For a detailed library guide, see [library-api.md](library-api.md).

## Basic Decoding

Decode a JPEG to binary PPM or PGM:

```bash
./dist/djpeg-linux-amd64-debug --ppm input.jpg > output.ppm
```

The output is PPM (`P6`) for RGB images and PGM (`P5`) for grayscale images.
`--pnm` is accepted as the upstream `djpeg` alias for the same output path.

Force grayscale output from a color JPEG:

```bash
./dist/djpeg-linux-amd64-debug --grayscale --ppm input.jpg > output.pgm
```

Force RGB output from a grayscale JPEG:

```bash
./dist/djpeg-linux-amd64-debug --rgb --ppm grayscale-input.jpg > output.ppm
```

Write the decoded image to a named file:

```bash
./dist/djpeg-linux-amd64-debug --ppm --outfile output.ppm input.jpg
```

Read JPEG data from stdin:

```bash
cat input.jpg | ./dist/djpeg-linux-amd64-debug --ppm > output.ppm
```

## IDCT Selection

Use the integer IDCT path. This is the path used for exact parity testing
against IJG 9f:

```bash
./dist/djpeg-linux-amd64-debug --dct int --ppm input.jpg > output.ppm
```

Use the faster integer IDCT variant:

```bash
./dist/djpeg-linux-amd64-debug --dct fast --ppm input.jpg > output.ppm
```

Use the libjpeg-compatible fast shorthand. In the current decoder this maps to
`--dct fast --nosmooth`; the color-quantization side of IJG `-fast` will be
added when quantization support lands.

```bash
./dist/djpeg-linux-amd64-debug --fast --ppm input.jpg > output.ppm
```

Use the floating-point IDCT variant:

```bash
./dist/djpeg-linux-amd64-debug --dct float --ppm input.jpg > output.ppm
```

Only `--dct int` is currently part of the exact-100 parity gate.

## Upsampling

Default mode uses IJG-compatible fancy upsampling where applicable:

```bash
./dist/djpeg-linux-amd64-debug --dct int --ppm input.jpg > smooth.ppm
```

Disable fancy upsampling:

```bash
./dist/djpeg-linux-amd64-debug --dct int --nosmooth --ppm input.jpg > nosmooth.ppm
```

Both default and `--nosmooth` modes are included in the current exact-100
random100 parity run.

## Memory Limit

Set an approximate upper bound for decoder-owned buffers:

```bash
./dist/djpeg-linux-amd64-debug --maxmemory 20m --ppm input.jpg > output.ppm
```

As in libjpeg, a bare number is interpreted as kilobytes and `m`/`M` means
megabytes. The limit applies to the decoder's compressed scan buffer and decoded
component buffers, not to caller-owned output files or shell redirection.

## Scaling

Scale output by a libjpeg-style `M/N` fraction:

```bash
./dist/djpeg-linux-amd64-debug --scale 1/2 --ppm input.jpg > half.ppm
```

For 8x8 DCT JPEGs, ratios map to the closest supported scale size from `1/8`
through `16/8`. Scaling is applied in the decoder's scaled-IDCT output path, so
scanline and file output use the same scaled dimensions.

## Color Quantization

Reduce RGB output to a generated palette:

```bash
./dist/djpeg-linux-amd64-debug --colors 216 --ppm input.jpg > quantized.ppm
```

Select a dithering mode for quantized output:

```bash
./dist/djpeg-linux-amd64-debug --colors 216 --dither ordered --ppm input.jpg > quantized.ppm
```

Supported dither modes are `fs`, `ordered`, and `none`. `--onepass` is accepted
for libjpeg CLI compatibility and selects the faster generated color-cube
palette instead of the default image-derived two-pass palette.

Use an external palette from a GIF or PPM file:

```bash
./dist/djpeg-linux-amd64-debug --map palette.ppm --ppm input.jpg > mapped.ppm
```

RGB GIF output automatically enables quantization to at most 256 colors:

```bash
./dist/djpeg-linux-amd64-debug --gif --outfile output.gif input.jpg
```

Use upstream-compatible uncompressed GIF output:

```bash
./dist/djpeg-linux-amd64-debug --gif0 --outfile output.gif input.jpg
```

## Poppler/ImageMagick-Compatible 4:2:0 Output

Some PDF image streams match Poppler/ImageMagick output when libjpeg-turbo-style
8x8 chroma IDCT plus fancy upsampling is used instead of IJG 9f chroma IDCT
scaling:

```bash
./dist/djpeg-linux-amd64-debug --compatibility poppler-pdf --ppm input.jpg > output.ppm
```

`--turbo-fancy` is retained as a deprecated alias for this profile.

## Other Output Formats

Write BMP:

```bash
./dist/djpeg-linux-amd64-debug --bmp --outfile output.bmp input.jpg
```

Write OS/2 1.x BMP:

```bash
./dist/djpeg-linux-amd64-debug --os2 --outfile output.bmp input.jpg
```

Write Targa:

```bash
./dist/djpeg-linux-amd64-debug --targa --outfile output.tga input.jpg
```

Write Utah RLE:

```bash
./dist/djpeg-linux-amd64-debug --rle --outfile output.rle input.jpg
```

GIF output is limited. Grayscale images can be written with a generated
grayscale palette:

```bash
./dist/djpeg-linux-amd64-debug --gif --outfile gray.gif grayscale-input.jpg
```

Color GIF requires indexed-color data. The current CLI does not provide a
fully verified color quantization path, so PPM/BMP/Targa are better choices
for RGB input.

## Verbose Output

Print input and output metadata to stderr:

```bash
./dist/djpeg-linux-amd64-debug --verbose --ppm input.jpg > output.ppm
```

`--debug` is accepted as the upstream `djpeg` alias for `--verbose`.

## CPU Profiling

The CLI supports an internal CPU profile flag:

```bash
./dist/djpeg-linux-amd64-debug --cpuprofile cpu.pprof --ppm input.jpg > output.ppm
go tool pprof -top cpu.pprof
```

For benchmark profiles, prefer the Go benchmark harness:

```bash
go test ./tests \
  -run '^$' \
  -bench '^BenchmarkDecompressRandom100Default$' \
  -benchmem \
  -benchtime=5x \
  -count=1 \
  -cpuprofile docs/djpeg-go-cpu.pprof \
  -memprofile docs/djpeg-go-mem.pprof
```

## Exact Parity Check

Build the Go CLI and compare it with the local IJG 9f C reference:

```bash
go build -o /tmp/djpeg_go_exact100 ./cmd/djpeg
scripts/exact100_compare.py \
  --mode both \
  --go-djpeg /tmp/djpeg_go_exact100 \
  --json /tmp/exact100.json \
  --quiet-ok
```

The script compares PNM pixel payloads instead of raw file bytes so harmless
PPM/PGM header formatting differences do not affect the result.

## C vs Go Performance

Run the random100 CLI wall-time comparison:

```bash
go build -o /tmp/djpeg_go_perf ./cmd/djpeg
scripts/perf_compare.py \
  --go-djpeg /tmp/djpeg_go_perf \
  --mode both \
  --warmup 1 \
  --repeat 5 \
  --json /tmp/djpeg_perf.json
```

The latest report is in [performance.md](performance.md).

Sequential arithmetic-coded JPEG files are supported. Progressive JPEG headers
can be inspected, but progressive scanline and coefficient decoding are
currently rejected by the decoder.
