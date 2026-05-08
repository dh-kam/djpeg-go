# Examples

This document shows common `djpeg-go` CLI workflows. The examples assume that
you built the binary as `./bin/djpeg-go`.

```bash
go build -o ./bin/djpeg-go ./cmd/djpeg
```

## Basic Decoding

Decode a JPEG to binary PPM or PGM:

```bash
./bin/djpeg-go --ppm input.jpg > output.ppm
```

The output is PPM (`P6`) for RGB images and PGM (`P5`) for grayscale images.

Write the decoded image to a named file:

```bash
./bin/djpeg-go --ppm --outfile output.ppm input.jpg
```

Read JPEG data from stdin:

```bash
cat input.jpg | ./bin/djpeg-go --ppm > output.ppm
```

## IDCT Selection

Use the integer IDCT path. This is the path used for exact parity testing
against IJG 9f:

```bash
./bin/djpeg-go --dct int --ppm input.jpg > output.ppm
```

Use the faster integer IDCT variant:

```bash
./bin/djpeg-go --dct fast --ppm input.jpg > output.ppm
```

Use the floating-point IDCT variant:

```bash
./bin/djpeg-go --dct float --ppm input.jpg > output.ppm
```

Only `--dct int` is currently part of the exact-100 parity gate.

## Upsampling

Default mode uses IJG-compatible fancy upsampling where applicable:

```bash
./bin/djpeg-go --dct int --ppm input.jpg > smooth.ppm
```

Disable fancy upsampling:

```bash
./bin/djpeg-go --dct int --nosmooth --ppm input.jpg > nosmooth.ppm
```

Both default and `--nosmooth` modes are included in the current exact-100
random100 parity run.

## Other Output Formats

Write BMP:

```bash
./bin/djpeg-go --bmp --outfile output.bmp input.jpg
```

Write Targa:

```bash
./bin/djpeg-go --targa --outfile output.tga input.jpg
```

Write Utah RLE:

```bash
./bin/djpeg-go --rle --outfile output.rle input.jpg
```

GIF output is limited. Grayscale images can be written with a generated
grayscale palette:

```bash
./bin/djpeg-go --gif --outfile gray.gif grayscale-input.jpg
```

Color GIF requires indexed-color data. The current CLI does not provide a
fully verified color quantization path, so PPM/BMP/Targa are better choices
for RGB input.

## Verbose Output

Print input and output metadata to stderr:

```bash
./bin/djpeg-go --verbose --ppm input.jpg > output.ppm
```

## CPU Profiling

The CLI supports an internal CPU profile flag:

```bash
./bin/djpeg-go --cpuprofile cpu.pprof --ppm input.jpg > output.ppm
go tool pprof -top cpu.pprof
```

For benchmark profiles, prefer the Go benchmark harness:

```bash
go test ./cmd/djpeg \
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
