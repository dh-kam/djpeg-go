# djpeg-go

`djpeg-go` is a Pure Go port of the JPEG decompression path from the
Independent JPEG Group's JPEG software. The current reference upstream is
IJG libjpeg 9f, released on 14-Jan-2024.

The decoder is implemented in Go and does not link to libjpeg at runtime. The
local `jpeg-9f/` tree is kept as an upstream reference for parity and
performance testing.

## Status

- Baseline, non-progressive JPEG decoding is the primary supported path.
- Arithmetic-coded and progressive JPEG files are currently rejected.
- PPM/PGM output is the most thoroughly verified CLI output path.
- BMP, Targa, RLE, and limited GIF writer code exists, but exact parity is
  currently measured against PPM/PGM output.
- Exact-100 parity against IJG 9f `djpeg -dct int` is currently achieved for
  the `testdata/random100` corpus in both default smooth and `--nosmooth`
  modes.

See [docs/exact-100-result.md](docs/exact-100-result.md) and
[docs/performance.md](docs/performance.md) for the latest measured results.

## Upstream

This project is based in part on the work of the Independent JPEG Group. The
Go implementation follows IJG data structures, marker parsing, entropy decode,
IDCT behavior, color conversion, and selected `djpeg` output semantics where
they are relevant to the port.

Important upstream references:

- `jpeg-9f/README`: upstream IJG license and documentation
- `jpeg-9f/jd*.c`, `jpeg-9f/jidct*.c`, `jpeg-9f/jdsample.c`,
  `jpeg-9f/jdcolor.c`: decoder behavior used as the reference
- `jpeg-9f/.libs/djpeg`: local C reference binary used by parity/performance
  scripts when available

This project is not affiliated with the Independent JPEG Group.

## Build

```bash
go build -o ./bin/djpeg-go ./cmd/djpeg
```

To install the CLI into your `GOBIN`:

```bash
go install ./cmd/djpeg
```

## Quick Use

Decode a JPEG to raw binary PPM/PGM:

```bash
./bin/djpeg-go --ppm input.jpg > output.ppm
```

Disable fancy upsampling:

```bash
./bin/djpeg-go --dct int --nosmooth --ppm input.jpg > output.ppm
```

Write to a file:

```bash
./bin/djpeg-go --ppm --outfile output.ppm input.jpg
```

More examples are in [docs/examples.md](docs/examples.md).

## Verification

Run the Go test suite:

```bash
go test ./...
go vet ./...
```

Run exact parity against the IJG 9f C reference:

```bash
go build -o /tmp/djpeg_go_exact100 ./cmd/djpeg
scripts/exact100_compare.py \
  --mode both \
  --go-djpeg /tmp/djpeg_go_exact100 \
  --quiet-ok
```

Run the C-vs-Go performance comparison:

```bash
go build -o /tmp/djpeg_go_perf ./cmd/djpeg
scripts/perf_compare.py \
  --go-djpeg /tmp/djpeg_go_perf \
  --mode both \
  --warmup 1 \
  --repeat 5
```

## Repository Layout

- `cmd/djpeg`: command-line interface compatible with the `djpeg` workflow
- `internal/decoder`: high-level JPEG decompression pipeline
- `internal/marker`: marker parsing and decompressor metadata
- `internal/huff`: Huffman entropy decode and IDCT implementations
- `internal/color`: IJG-inspired color conversion and upsampling support code
- `internal/output`: PPM/PGM, BMP, GIF, Targa, and RLE output writers
- `scripts`: parity and performance harnesses
- `docs`: accuracy, performance, and usage documentation
- `jpeg-9f`: upstream IJG reference source and local C reference build

## License

The original Go code in this repository is licensed under the MIT License. See
[LICENSE](LICENSE).

Portions are derived from or based on the Independent JPEG Group's software and
remain subject to the IJG license conditions. In particular, this software is
based in part on the work of the Independent JPEG Group. See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and `jpeg-9f/README`.
