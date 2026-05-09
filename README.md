# djpeg-go

`djpeg-go` is a Pure Go port of the JPEG decompressor path from the
Independent JPEG Group's JPEG software. The library goal is a libjpeg-compatible
decompressor API; the `cmd/djpeg` binary is a CLI compatibility and debug tool
built on top of that API. The current reference upstream is IJG libjpeg 9f,
released on 14-Jan-2024.

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
  the `tests/testdata/random100` corpus in both default smooth and `--nosmooth`
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
make build
```

To install the CLI into your `GOBIN`:

```bash
go install ./cmd/djpeg
```

Build one release target:

```bash
make linux-amd64-release VERSION=v0.1.0-202605.1-9f
```

Build all release targets:

```bash
make release VERSION=v0.1.0-202605.1-9f
```

Release tags use `vSEMVER-YYYYMM.seq-upstreamversion`, for example
`v0.1.0-202605.1-9f`. To print the next tag for the current month:

```bash
make bump-up SEMVER=0.1.0 UPSTREAM_VERSION=9f
```

## Quick Use

Use the decoder as a Go library:

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

Use raw pixels when exact byte layout matters:

```go
raster, err := libjpeg.DecodeRaster(
	input,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
)
if err != nil {
	return err
}
// raster.Pix is top-down Gray8 or RGB24 data with raster.Stride bytes per row.
```

The import path still reflects the current repository name. The examples alias
the root package as `libjpeg` because the public API is the Poppler/go-pdf
integration surface; `djpeg` parity remains a regression test and CLI frontend.

Decode a JPEG to raw binary PPM/PGM:

```bash
./dist/djpeg-linux-amd64-debug --ppm input.jpg > output.ppm
```

Disable fancy upsampling:

```bash
./dist/djpeg-linux-amd64-debug --dct int --nosmooth --ppm input.jpg > output.ppm
```

Match Poppler/ImageMagick-style output for PDF 4:2:0 DCT streams:

```bash
./dist/djpeg-linux-amd64-debug --compatibility poppler-pdf --ppm input.jpg > output.ppm
```

The older `--turbo-fancy` flag remains as a deprecated alias for that
compatibility profile.

Write to a file:

```bash
./dist/djpeg-linux-amd64-debug --ppm --outfile output.ppm input.jpg
```

More examples are in [docs/examples.md](docs/examples.md). The public Go
library facade is documented in [docs/library-api.md](docs/library-api.md).

## Verification

Run the Go test suite:

```bash
make test
make vet
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

- package root (`github.com/dh-kam/djpeg-go`): public Go library API
- `cmd/djpeg`: command-line interface compatible with the `djpeg` workflow
- `internal/djpegcli`: Cobra/Viper command orchestration for the CLI
- `internal/decoder`: high-level JPEG decompression pipeline
- `internal/marker`: marker parsing and decompressor metadata
- `internal/huff`: Huffman entropy decode and IDCT implementations
- `internal/color`: IJG-inspired color conversion and upsampling support code
- `internal/output`: PPM/PGM, BMP, GIF, Targa, and RLE output writers
- `tests`: integration, CLI, parity, benchmark tests and shared fixtures
- `scripts`: parity and performance harnesses
- `docs`: accuracy, performance, and usage documentation
- `jpeg-9f`: upstream IJG reference source and local C reference build

## CI and Release

GitHub Actions runs `make vet`, `make test`, and command builds on each push
and pull request. The manual Release workflow computes the next bump-up tag,
builds static release binaries for Linux, macOS, and Windows on amd64/arm64,
creates the git tag, and uploads the binaries to the GitHub release.

## License

The original Go code in this repository is licensed under the MIT License. See
[LICENSE](LICENSE).

Portions are derived from or based on the Independent JPEG Group's software and
remain subject to the IJG license conditions. In particular, this software is
based in part on the work of the Independent JPEG Group. See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and `jpeg-9f/README`.
