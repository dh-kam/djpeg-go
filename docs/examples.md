# Examples

This document shows common `djpeg-go` CLI workflows. The examples assume a
Linux amd64 debug build at `./dist/djpeg-linux-amd64-debug`.

```bash
make build
```

## Go Library API

Import the root module when another Go module needs to decode JPEG data:

```go
import djpeg "github.com/dh-kam/djpeg-go"
```

Decode to an `image.Image` for use with the Go image ecosystem:

```go
in, err := os.Open("input.jpg")
if err != nil {
	return err
}
defer in.Close()

img, err := djpeg.Decode(in)
if err != nil {
	return err
}

return png.Encode(out, img)
```

Decode to raw pixels when you need stable Gray8 or RGB24 byte layout:

```go
raster, err := djpeg.DecodeRaster(
	in,
	djpeg.WithIDCT(djpeg.IDCTInt),
)
if err != nil {
	return err
}

switch raster.Format {
case djpeg.PixelFormatGray8:
	useGray(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
case djpeg.PixelFormatRGB24:
	useRGB(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
}
```

Use the Poppler/ImageMagick-compatible chroma path from Go:

```go
raster, err := djpeg.DecodeRaster(
	in,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithTurboFancy(),
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

## Poppler/ImageMagick-Compatible 4:2:0 Output

Some PDF image streams match Poppler/ImageMagick output when libjpeg-turbo-style
8x8 chroma IDCT plus fancy upsampling is used instead of IJG 9f chroma IDCT
scaling:

```bash
./dist/djpeg-linux-amd64-debug --turbo-fancy --ppm input.jpg > output.ppm
```

## Other Output Formats

Write BMP:

```bash
./dist/djpeg-linux-amd64-debug --bmp --outfile output.bmp input.jpg
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

## Unsupported or Incomplete Options

Some `djpeg`-style flags are parsed for CLI compatibility but are not fully
implemented yet:

- `--grayscale`
- `--rgb`
- `--fast`
- `--onepass`
- `--dither`
- `--scale`
- `--maxmemory`

Arithmetic-coded and progressive JPEG files are currently rejected by the
decoder.
