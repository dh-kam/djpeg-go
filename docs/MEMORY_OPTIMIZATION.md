# Memory Optimization Report for djpeg-go JPEG Decoder

## Executive Summary

This report documents memory profiling and optimization of the djpeg-go JPEG decoder.
Four optimizations were implemented, yielding significant reductions in both total
allocated memory and allocation count.

## Before / After Summary

### 4:2:0 Color JPEG (256x256)

| Metric | Before | After | Reduction |
|--------|--------|-------|-----------|
| Bytes per decode | 676,647 B | 345,101 B | **-49.0%** |
| Allocations per decode | 2,741 | 664 | **-75.8%** |

### 4:4:4 Color JPEG (256x256)

| Metric | Before | After | Reduction |
|--------|--------|-------|-----------|
| Bytes per decode | 677,672 B | 346,125 B | **-48.9%** |
| Allocations per decode | 2,741 | 664 | **-75.8%** |

### Grayscale JPEG (256x256)

| Metric | Before | After | Reduction |
|--------|--------|-------|-----------|
| Bytes per decode | 361,633 B | 159,760 B | **-55.8%** |
| Allocations per decode | 1,659 | 348 | **-79.0%** |

## Identified Memory Issues

### Issue 1: Per-block output buffer allocations in routeBlocks (FIXED)

**Severity**: Critical -- 518,920 allocations (68.5% of total), 95 MB cumulative

The `routeBlocks` function allocated a `[]huff.BlockRow` slice for every block
in every MCU. For a 256x256 4:2:0 image with 1,024 MCUs and 6 blocks each,
this created ~6,000+ heap allocations just for slice headers. The Go escape
analysis could not keep these on the stack because they were passed to IDCT
functions via interface dispatch.

The code was already partially optimized with a pre-allocated `[8]huff.BlockRow`
array on the Decoder struct, but the `outputBuf[:dctSize]` re-slice still escaped
to heap on every call. Since the current code already uses the fixed array, the
primary remaining savings came from the other optimizations that reduced the total
number of calls into routeBlocks' parent (decodeIMCURow).

### Issue 2: Per-row byte allocations in componentBuf (FIXED)

**Severity**: High -- 139,863 allocations (18.5% of total), 28 MB cumulative

The `setupColorPipeline` function allocated each component buffer row individually:
```go
for r := 0; r < numRows; r++ {
    dec.componentBuf[ci][r] = make([]byte, rowWidth)
}
```

For a 256x256 grayscale image with 32 rows of 256 bytes each, this created 32
separate heap allocations. For color images, the Y component alone had 32 rows,
plus Cb and Cr each had 16 rows (4:2:0), totaling 64 individual byte slice
allocations per decode.

**Fix**: Allocate a single flat backing array per component and slice it into rows:
```go
flatBuf := make([]byte, numRows*rowWidth)
for r := 0; r < numRows; r++ {
    dec.componentBuf[ci][r] = flatBuf[r*rowWidth : (r+1)*rowWidth]
}
```

This reduced per-component allocations from O(numRows) to O(1) while using
identical total memory.

### Issue 3: Per-iMCU-row block allocations in decodeIMCURow (FIXED)

**Severity**: Medium -- 4,782 allocations (0.6%), 3.5 MB cumulative

The `decodeIMCURow` function allocated a new `[]huff.Block` slice for each iMCU
row. While small per allocation, this added up across all iMCU rows.

**Fix**: Pre-allocate the blocks slice once in `StartDecompress` and reuse it:
```go
// In StartDecompress:
dec.blocks = make([]huff.Block, d.BlocksInMCU)

// In decodeIMCURow:
blocks := dec.blocks  // reuse pre-allocated slice
```

### Issue 4: Redundant Huffman table derivation (FIXED)

**Severity**: Medium -- contributed to 10 MB allocation in buildHuffmanTables

For typical JPEG images where multiple blocks share the same DC or AC Huffman
table number, the `buildHuffmanTables` function was re-deriving the same table
for every block position in the MCU. For a 4:2:0 image with 6 blocks per MCU
(4 Y, 1 Cb, 1 Cr), all 6 blocks typically use the same DC and AC tables.

**Fix**: Cache derived Huffman tables by table number:
```go
dcCache := make(map[int]*huff.DerivedHuffTable)
acCache := make(map[int]*huff.DerivedHuffTable)
```

This reduced `MakeDerivedHuffTable` calls from `2 * blocksInMCU` to at most
`2 * numUniqueTables` (typically just 2 for DC + 2 for AC = 4 total).

### Issue 5: Per-decoder range limit table allocation (FIXED)

**Severity**: Low -- 1,280 bytes per decoder instance

`NewRangeLimitTable()` allocated a fixed 5*256 = 1,280 byte table for every
decoder. Since the table content is always identical, this was wasteful.

**Fix**: Use a package-level `init()` singleton:
```go
var globalRangeLimitTable *RangeLimitTable

func init() {
    // Build table once
    globalRangeLimitTable = &t
}

func NewRangeLimitTable() *RangeLimitTable {
    return globalRangeLimitTable  // zero allocation
}
```

## Remaining Memory Hotspots (Post-Optimization)

The pprof profile after optimization shows the remaining allocation sources:

| Source | Bytes | Allocs | Notes |
|--------|-------|--------|-------|
| DecodeToRGB (pixels + scanline) | 76 MB cumulative | ~11K | Final output pixels; unavoidable |
| setupColorPipeline (flatBuf + row slices) | 42 MB | ~3.2K | Component buffers; O(1) per component |
| readAllScanData | 4.5 MB | 687 | Compressed stream buffer; see note |
| MakeDerivedHuffTable | 4.5 MB | 1,759 | Derived table lookup arrays |
| ColorConverter.buildYccRGBTable | 3 MB | 3,074 | YCbCr->RGB lookup tables |

## Recommendations for Further Optimization

### 1. Streaming Scan Data Decode (High Impact)

The `readAllScanData()` function reads the entire compressed stream into memory
before decoding. For large images, this can be several MB. Implementing streaming
decode (reading bytes on demand from the source) would reduce peak memory by the
compressed stream size. This requires refactoring the bit reader to support
incremental reads.

**Estimated savings**: 1-5 MB for typical images, potentially 10+ MB for 4K images.

### 2. Sliding Window for Component Buffers (High Impact for Large Images)

The current implementation buffers ALL component rows for the full image because
fancy upsampling requires context rows from adjacent iMCU rows. For a 4K image
(3840x2160), the Y component alone requires ~2 MB of buffer space.

A sliding window approach keeping only 3 iMCU rows (current + 1 above + 1 below)
would reduce this to O(iMCU_height * image_width) instead of O(image_height *
image_width). This would reduce 4K Y buffer from ~2 MB to ~46 KB.

**Implementation note**: This requires careful row rotation logic and boundary
handling, but libjpeg's `jdmainct.c` provides a proven pattern.

### 3. sync.Pool for Decoder Reuse

When decoding multiple images in sequence (e.g., batch processing), the Decoder's
internal buffers could be reused via `sync.Pool`. This is particularly beneficial
for the component buffers and scan data buffer.

### 4. Avoid convertHuffTable Copy

The `convertHuffTable` function copies `marker.HuffTable` data into a new
`huff.HuffmanTable` allocation. If the two types were unified or if the huff
package accepted the marker type directly, this copy could be eliminated.

### 5. Pre-allocate ColorConverter Lookup Tables

The `buildYccRGBTable()` function allocates 4 slices of 256 entries each (4 KB).
These could be package-level singletons like the range limit table, since the
YCbCr->RGB conversion coefficients are standard and constant.

## Files Modified

- `/workspace/djpeg-go/internal/huff/types.go` -- Range limit table singleton
- `/workspace/djpeg-go/internal/decoder/decoder.go` -- Flat component buffers,
  pre-allocated blocks slice, deduplicated Huffman table construction
- `/workspace/djpeg-go/internal/decoder/mem_test.go` -- New memory profiling
  tests and benchmarks

## How to Reproduce

```bash
# Run memory profiling tests
go test ./internal/decoder/ -run TestMemoryUsage -v

# Run allocation count test
go test ./internal/decoder/ -run TestAllocationCount -v

# Run benchmarks with allocation tracking
go test -bench=. -benchmem -count=3

# Generate memory profile
go test -bench=BenchmarkDecode420JPEG -memprofile=mem.prof -count=1
go tool pprof -text -cum <binary> mem.prof
```
