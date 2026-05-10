# Performance Report

Generated: 2026-05-08 KST

This report compares `djpeg-go` against the local IJG C reference and records
the Go CPU/memory profile hotspots for the `tests/testdata/random100` corpus.

## Scope

| Item | Value |
|---|---|
| Corpus | `tests/testdata/random100/*.jpg` |
| Image count | 100 |
| Input bytes | 20,023,823 |
| Output pixels | 21,262,941 |
| Corpus manifest hash | `89a9d42404daddfe580771451ca8be4a027a1aa524c24dd1bbf79b2beb1c2f99` |
| C reference | `/workspace/djpeg-go/jpeg-9f/.libs/djpeg` |
| C reference sha256 | `99a8364db22d2ea0ab16ab2a1241f704074dda79d33a06d53c3f99ffcd136de0` |
| Go binary | `/tmp/djpeg_go_perf` |
| Go binary sha256 | `1078757c3dfe0baeb696c2bd35d287ec66cdd7c0b88b82d2d368cdd2320c7993` |
| Go version | `go1.25.7 linux/amd64` |
| Machine | Linux WSL2, Intel Core i7-12700K, 20 logical CPUs |

The benchmark uses the same official C reference as exact parity:
`jpeg-9f/.libs/djpeg` with `LD_LIBRARY_PATH=/workspace/djpeg-go/jpeg-9f/.libs`.
The old `jpeg-6b/djpeg` reference was removed from the benchmark path because it
was a contaminated debug build that wrote text to `stdout`.

## CLI Wall Time

Command:

```bash
go build -o /tmp/djpeg_go_perf ./cmd/djpeg
scripts/perf_compare.py \
  --go-djpeg /tmp/djpeg_go_perf \
  --mode both \
  --warmup 1 \
  --repeat 5 \
  --json /tmp/djpeg_perf_compare_after.json
```

Each measurement decodes all 100 JPEGs to PPM and discards stdout.

| Mode | C min | C median | C mean | Go min | Go median | Go mean | Go/C median |
|---|---:|---:|---:|---:|---:|---:|---:|
| Default smooth (`-ppm`) | 0.393713s | 0.399530s | 0.398376s | 0.776955s | 0.778388s | 0.787209s | 1.95x |
| No smooth (`-nosmooth -ppm`) | 0.366116s | 0.369595s | 0.369536s | 0.733494s | 0.749972s | 0.755406s | 2.03x |

## Go Benchmark

Command:

```bash
go test ./tests \
  -run '^$' \
  -bench '^BenchmarkDecompressRandom100(Default|NoSmooth)$' \
  -benchmem \
  -benchtime=5x \
  -count=1
```

One benchmark operation decodes the full 100-image corpus.

| Benchmark | Time/op | Bytes/op | Allocs/op |
|---|---:|---:|---:|
| `BenchmarkDecompressRandom100Default` | 475.664ms | 94,760,723 | 6,279 |
| `BenchmarkDecompressRandom100NoSmooth` | 454.447ms | 69,096,560 | 6,273 |

Small cleanup changes reduced allocation pressure materially while keeping
exact parity:

| Mode | Before bytes/op | After bytes/op | Before allocs/op | After allocs/op |
|---|---:|---:|---:|---:|
| Default smooth | 166,780,312 | 94,760,723 | 41,761 | 6,279 |
| No smooth | 141,279,195 | 69,096,560 | 42,454 | 6,273 |

## Profile Artifacts

Generated with:

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

| Artifact | sha256 |
|---|---|
| `docs/djpeg-go-cpu.pprof` | `630dd9b7a7f6e45dd456f3c7b95600498a14a7f1ee655571c9cf51a0da13e1a4` |
| `docs/djpeg-go-mem.pprof` | `44c6a08e0804d3df727afd29cb1f756f40ba67822db526e989435ee2230fdb37` |

Profile benchmark result:

```text
BenchmarkDecompressRandom100Default-20  5  471927100 ns/op  94765440 B/op  6281 allocs/op
```

## CPU Hotspots

`go tool pprof -top -nodecount=20 docs/djpeg-go-cpu.pprof`

| Function | Flat | Flat % | Cum | Cum % |
|---|---:|---:|---:|---:|
| `huff.DecodeMCUSequential` | 0.82s | 27.42% | 1.65s | 55.18% |
| `huff.HuffDecodeFast` | 0.47s | 15.72% | 0.62s | 20.74% |
| `huff.IDCTISlowImpl` | 0.45s | 15.05% | 0.52s | 17.39% |
| `decoder.(*Decoder).upsampleAndConvert` | 0.40s | 13.38% | 0.40s | 13.38% |
| `huff.HuffExtend` | 0.21s | 7.02% | 0.21s | 7.02% |
| `huff.IDCT16x16Impl` | 0.16s | 5.35% | 0.16s | 5.35% |
| `huff.fillBitBuffer` | 0.13s | 4.35% | 0.13s | 4.35% |
| `decoder.(*Decoder).readAllScanData` | 0.04s | 1.34% | 0.06s | 2.01% |

Interpretation:

- Huffman entropy decode is the dominant CPU cost.
- Default smooth mode still pays for IJG-compatible 16x16 chroma IDCT on 4:2:0 images.
- `upsampleAndConvert` is a meaningful but smaller CPU target after removing row-level temporary allocation.

## Memory Hotspots

`go tool pprof -top -alloc_space -nodecount=20 docs/djpeg-go-mem.pprof`

| Function | Alloc space | % |
|---|---:|---:|
| `decoder.(*Decoder).setupColorPipeline` | 397.09MB | 66.64% |
| `decoder.(*Decoder).readAllScanData` | 130.46MB | 21.89% |
| `os.readFileContents` | 42.25MB | 7.09% |
| `color.(*ColorConverter).buildYccRGBTable` | 8.51MB | 1.43% |
| `huff.MakeDerivedHuffTable` | 6.52MB | 1.09% |

Interpretation:

- The largest memory cost is structural: the decoder currently allocates full-image component buffers, then serves scanlines from those buffers.
- `readAllScanData` also reads the entropy stream into memory before Huffman decode.
- Long-lived in-use memory after the benchmark is small; the issue is allocation volume and cache locality, not retained heap.

## Cleanup Performed

- Removed scratch/debug files that broke or polluted builds: root `main` experiments, duplicate `cmd/djpeg/main.go_*`, debug decoder tests, stale generated PPM/binary/cache files.
- Removed contaminated `jpeg-6b` benchmark reference and stale `docs/performance.ko.md`.
- Replaced the old one-shot `scripts/benchmark.sh` with a reproducible wrapper around `scripts/perf_compare.py`.
- Added random100 benchmarks in `tests/cmd_benchmark_test.go`.
- Fixed skipped fixture paths in root and decoder memory benchmarks.
- Removed dead `uint32 << 32` Huffman fast path that made `go vet ./...` fail.
- Removed unused decoder fields and unused h2v2 helper.
- Reused h2v2 fancy chroma work buffers instead of allocating per output row.
- Added scan-data capacity hint when the source reader exposes `Len()`.
- Reworked scalar marker reads to use `markerReader` scratch storage, reducing
  random100 benchmark allocations from about 40k/op to about 6.3k/op.

## Expert Review Summary

Eight requested review roles were assigned in two waves due the active-agent
limit. The shared conclusions were consistent:

- Highest-impact fix: convert the decoder from full-image buffering to an iMCU-row streaming/sliding-window pipeline. This should cut `setupColorPipeline` allocation substantially, but has high exact-parity risk around fancy upsampling context rows, padding, and restart markers.
- Second structural fix: unify marker and entropy input ownership so `readAllScanData` and restart-marker stripping are not separate copy/compact stages.
- Low-risk CPU work remains in Huffman decode, `NaturalOrder`/bounds-check elimination, and color conversion table indexing.
- Low-risk allocation work remains in shared color conversion tables, Huffman
  derived table workspaces, and eventually replacing row-slice component planes.
- The 16x16 scaled IDCT is required for IJG 9f exact default smooth parity. A faster 8x8 chroma + fused h2v2 path is possible only as a separate non-exact or opt-in mode.
- Test gates should always include `go test ./...`, `go vet ./...`, and `scripts/exact100_compare.py --mode both`.

## Recommended Next Steps

1. Build a small benchmark guard around `BenchmarkDecompressRandom100Default` and `NoSmooth` with `-count=5` and historical JSON output.
2. Prototype iMCU-row component-buffer streaming behind a feature flag, then run exact-100 after each step.
3. Move restart marker handling down into the entropy source before removing `readAllScanData`.
4. Add focused tests for restart markers, 4:2:2, grayscale, progressive decoding, and non-PPM output formats before broader pipeline refactors.
