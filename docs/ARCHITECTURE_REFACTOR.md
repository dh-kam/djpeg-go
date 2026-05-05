# Architecture Refactoring Plan for djpeg-go

## Executive Summary

The djpeg-go project is a pure Go port of IJG libjpeg 9f. It currently has
three independent decompression data paths that duplicate types, logic, and
state across packages. The `decoder` package monolithically re-implements
upsampling and color conversion inline instead of using the `color` package's
existing `Upsampler`, `ColorConverter`, `MergedUpsampler`, `CoefController`,
and `MainController`. This plan addresses type duplication, interface gaps,
testability, and the max-2 pixel accuracy deviation from libjpeg-turbo.

---

## 1. Type Unification Plan

### 1.1 Problem: Three Incompatible ComponentInfo Types

Three packages define their own component info structs:

| Package | Type | File | Line |
|---------|------|------|------|
| `jpeg` | `JPEGComponentInfo` | `internal/jpeg/types.go` | 98-138 |
| `marker` | `ComponentInfo` | `internal/marker/types.go` | 160-193 |
| `color` | `ComponentInfo` | `internal/color/types.go` | 39-57 |
| `huff` | `ComponentInfo` | `internal/huff/types.go` | 61-78 |

Similarly, three packages define their own quantization table types:

| Package | Type | File |
|---------|------|------|
| `jpeg` | `JQuantTbl` (fields: `QuantVal [64]uint16`) | `internal/jpeg/types.go:74` |
| `marker` | `QuantTable` (fields: `QuantVal [64]uint16`) | `internal/marker/types.go:147` |
| `color` | `QuantTable` (fields: `QuantVal [64]int16`) | `internal/color/types.go:60` |

And three packages define Huffman table types:

| Package | Type | File |
|---------|------|------|
| `jpeg` | `JHuffTbl` | `internal/jpeg/types.go:83` |
| `marker` | `HuffTable` | `internal/marker/types.go:153` |
| `huff` | `HuffmanTable` | `internal/huff/types.go:165` |

And three packages define color space enums, natural order tables, and range
limit tables independently.

### 1.2 Solution: Canonical Types Package

Create `internal/types/` as the single source of truth for all shared types.
This package has NO dependencies on other internal packages.

**Proposed `internal/types/` contents:**

```
internal/types/
  component.go   -- ComponentInfo (unified)
  quant.go       -- QuantTable
  huffman.go     -- HuffmanTable
  colorspace.go  -- ColorSpace enum, ColorTransform enum
  dct.go         -- DCT constants, NaturalOrder tables, IDCTMethod enum
  range.go       -- RangeLimitTable, range limit constants
  scan.go        -- Scan-related types (Block, JCOEF, JSAMPLE, etc.)
  errors.go      -- Shared error codes (no message table -- that stays in jpeg)
```

**Unified `ComponentInfo`:**

```go
package types

type ComponentInfo struct {
    // Fixed over the whole image
    ComponentID     int
    ComponentIndex  int
    HSampFactor     int
    VSampFactor     int
    QuantTblNo      int

    // Per-scan
    DCTblNo int
    ACTblNo int

    // Computed during startup
    WidthInBlocks     int
    HeightInBlocks    int
    DCTHScaledSize    int
    DCTVScaledSize    int
    DownsampledWidth  int
    DownsampledHeight int
    ComponentNeeded   bool

    // Per-scan computed
    MCUWidth       int
    MCUHeight      int
    MCUBlocks      int
    MCUSampleWidth int
    LastColWidth   int
    LastRowHeight  int

    // Saved quantization table
    QuantTable *QuantTable

    // Private per-component DCT/IDCT storage
    DCTTable interface{}
}
```

**Unified `QuantTable`:**

```go
type QuantTable struct {
    QuantVal  [DCTSize2]uint16
    SentTable bool
}
```

Note: `color.QuantTable` currently uses `int16` for `QuantVal`. The IJG C code
uses `uint16`. All downstream consumers should use `uint16` and cast to `int32`
only at the point of IDCT multiplication.

**Migration Strategy:**

Phase 1 (non-breaking): Create `internal/types/` with the unified types. Add
type aliases or conversion functions in each package so existing code compiles
against both old and new types simultaneously.

Phase 2: Update `marker`, `huff`, `color`, and `decoder` to import from
`internal/types`. Remove the duplicate type definitions from each package.

Phase 3: Remove the conversion shim functions.

### 1.3 Duplicate Constants to Unify

These constants are independently defined in 3+ packages and MUST move to
`internal/types/`:

- `DCTSize = 8`, `DCTSize2 = 64` -- in `jpeg`, `marker`, `huff`, `color`
- `MaxJSample = 255`, `CenterJSample = 128` -- in `jpeg`, `huff`, `color`
- `RangeBits`, `RangeCenter`, `RangeMask`, `RangeSubset` -- in `jpeg`, `huff`
- `NumQuantTbls = 4`, `NumHuffTbls = 4` -- in `jpeg`, `marker`
- `MaxCompsInScan = 4`, `MaxBlocksInMCU = 10` -- in `jpeg`, `marker`, `huff`
- `NaturalOrder` tables (6 variants) -- in `jpeg`, `marker`, `huff`
- Color space enum values -- in `jpeg`, `marker`, `color`

### 1.4 Packages to Deprecate After Unification

The `internal/jpeg/` package becomes largely redundant once types move to
`internal/types/`. Its remaining useful pieces (`DataSource`, error handling,
memory management) should be evaluated:

- `jpeg.DataSource` / `jpeg.SourceManager` -- keep in `jpeg` or move to
  `internal/source/`
- `jpeg.ErrorManager` / error codes -- keep in `jpeg` (it is the error handling hub)
- `jpeg.MemoryManager` / virtual arrays -- keep in `jpeg` (GC-based, lightweight)
- `jpeg.JPEGDecompress` -- this large struct is only used by the `jpeg` package
  itself and is NOT used by `marker.Decompressor` or `decoder.Decoder`. It can
  stay in `jpeg` for now, but should eventually be merged with
  `marker.Decompressor` (see Section 5).

---

## 2. Interface Design

### 2.1 Current State

The `jpeg` package defines Go interfaces (`EntropyDecoder`, `InverseDCT`,
`Upsampler`, `ColorDeconverter`, etc.) in `internal/jpeg/types.go:166-265`, but
NO code actually implements or uses these interfaces. The actual pipeline is
wired together ad-hoc in `internal/decoder/decoder.go`.

The `color` package defines concrete structs (`ColorConverter`, `Upsampler`,
`MergedUpsampler`, `CoefController`, `MainController`, `PostProcessor`) but
these are NOT connected behind a common pipeline interface.

### 2.2 Proposed Pipeline Interfaces

Define these interfaces in `internal/types/` (or a new `internal/pipeline/`
package):

```go
package pipeline

import "github.com/djpeg-go/djpeg-go/internal/types"

// EntropyDecoder decodes MCU coefficients from the compressed bitstream.
type EntropyDecoder interface {
    // StartPass initializes for a new scan pass.
    StartPass(cinfo *DecompressContext)

    // DecodeMCU decodes one MCU into the provided block array.
    // Returns true on success, false on suspension/marker.
    DecodeMCU(cinfo *DecompressContext, blocks []types.Block) bool

    // FinishPass completes the scan pass.
    FinishPass(cinfo *DecompressContext)
}

// InverseDCT performs dequantization and inverse DCT on one coefficient block.
type InverseDCT interface {
    // StartPass initializes IDCT (builds multiplier tables, etc.)
    StartPass(cinfo *DecompressContext)

    // InverseDCTBlock performs IDCT on one block for the given component.
    InverseDCTBlock(ci int, coefBlock []types.JCOEF,
        outputBuf [][]byte, outputCol int)
}

// ColorConverter converts from JPEG color space to output color space.
type ColorConverter interface {
    // StartPass initializes for a new pass.
    StartPass(cinfo *DecompressContext)

    // ColorConvert converts numRows of input component data to interleaved output.
    ColorConvert(inputBuf [][][]byte, inputRow int,
        outputBuf [][]byte, numRows int)
}

// Upsampler performs chroma upsampling on component data.
type Upsampler interface {
    // StartPass initializes for a new pass.
    StartPass()

    // NeedContextRows returns true if this upsampler needs context rows
    // from adjacent MCU rows (fancy upsampling).
    NeedContextRows() bool

    // Upsample performs upsampling and writes to outputBuf.
    Upsample(inputBuf [][][]byte, inRowGroupCtr *int,
        outputBuf [][]byte, outRowCtr *int, outRowsAvail int)
}

// OutputWriter writes decoded pixel data to a destination format.
type OutputWriter interface {
    Start(w io.Writer, info *OutputInfo) error
    WriteScanline(line []byte) error
    Finish() error
}
```

### 2.3 DecompressContext

The pipeline interfaces reference a `DecompressContext` that holds all shared
decompression state. This replaces both `marker.Decompressor` and the
hand-assembled `color.DecompressInfo`:

```go
type DecompressContext struct {
    // Image dimensions
    ImageWidth, ImageHeight     int
    OutputWidth, OutputHeight   int
    NumComponents               int
    OutColorComponents          int
    OutputComponents            int

    // Sampling
    MaxHSampFactor, MaxVSampFactor int
    MinDCTHScaledSize, MinDCTVScaledSize int

    // Color space
    JPEGColorSpace types.ColorSpace
    OutColorSpace  types.ColorSpace
    ColorTransform types.ColorTransform

    // Component info (unified type)
    CompInfo []types.ComponentInfo

    // Scan state
    CompsInScan    int
    CurCompInfo    [MaxCompsInScan]*types.ComponentInfo
    MCUsPerRow     int
    MCURowsInScan  int
    BlocksInMCU    int
    MCUMembership  [DMaxBlocksInMCU]int
    Ss, Se, Ah, Al int

    // Scan counters
    InputIMCURow   int
    OutputIMCURow  int
    TotalIMCURows  int
    OutputScanline int

    // Tables
    QuantTbls  [NumQuantTbls]*types.QuantTable
    DCHuffTbls [NumHuffTbls]*types.HuffmanTable
    ACHuffTbls [NumHuffTbls]*types.HuffmanTable

    // Flags
    ProgressiveMode bool
    ArithCode       bool
    QuantizeColors  bool
    DoBlockSmoothing bool

    // Range limit table (shared reference)
    RangeLimit *types.RangeLimitTable
}
```

### 2.4 Pipeline Wiring

The `decoder` package's `StartDecompress` should construct the pipeline:

```
EntropyDecoder --> CoefController --> InverseDCT --> MainController
                                                          |
                                              ColorConverter + Upsampler
                                                          |
                                                    PostProcessor
                                                          |
                                                    OutputWriter
```

Currently, `decoder.go:routeBlocks()` (line 370) manually does IDCT, and
`decoder.go:upsampleAndConvert()` (line 426) manually does upsampling + color
conversion. These should be replaced by calls through the pipeline interfaces.

---

## 3. Testability Improvements

### 3.1 Current Problems

1. **No unit tests for IDCT accuracy.** The IDCT implementations in `huff/`
   (`idct_int.go`, `idct_fst.go`, `idct_flt.go`) have tests only for the
   multiplier table construction, not for actual IDCT output accuracy.

2. **No tests for color conversion accuracy.** The `color/color.go` YCbCr->RGB
   conversion is tested only for basic setup, not for pixel-level accuracy.

3. **No tests for fancy upsampling accuracy.** The `color/merge.go` merged
   upsampler has zero tests.

4. **`decoder.go` is 928 lines of monolithic code** that combines bit reading,
   MCU decoding, IDCT routing, upsampling, and color conversion. None of these
   stages can be tested in isolation.

5. **No reference comparison tests.** There are no tests that compare output
   against libjpeg-turbo reference PPM files.

### 3.2 Proposed Test Architecture

#### 3.2.1 IDCT Accuracy Tests (`internal/huff/idct_test.go`)

Add a test that uses the standard IEEE 1180 test vectors (the same ones used
to validate libjpeg's IDCT):

```go
func TestIDCTISlowAccuracy(t *testing.T) {
    // Use known input coefficients and expected output from IEEE 1180 test.
    // For each of 64 test blocks, verify max pixel error is 0 or 1.
    // Compare against libjpeg-turbo reference output.
}

func TestIDCTISlowPeano(t *testing.T) {
    // Walk through all possible quantization values to find worst-case error.
    // The C code passes with max error 1 for ISLOW method.
}
```

#### 3.2.2 Color Conversion Tests (`internal/color/color_test.go`)

Add pixel-level accuracy tests:

```go
func TestYCbCrToRGBAccuracy(t *testing.T) {
    // For all Y in [0,255], Cb in [0,255], Cr in [0,255]:
    //   Compute R,G,B using our ColorConverter
    //   Compute reference using libjpeg-turbo's formulas
    //   Verify exact match (0 error)
    //
    // The formulas are deterministic fixed-point arithmetic.
    // With identical constants, the output MUST match exactly.
}

func TestYCbCrToRGBSpecific(t *testing.T) {
    // Test specific known values:
    // Y=0, Cb=128, Cr=128 -> R=0, G=0, B=0 (black)
    // Y=255, Cb=128, Cr=128 -> R=255, G=255, B=255 (white)
    // Y=76, Cb=85, Cr=255 -> R=255, G=0, B=0 (pure red via BT.601)
}
```

#### 3.2.3 Upsampling Tests (`internal/color/upsample_test.go`)

Add fancy upsampling tests with reference data:

```go
func TestH2V2FancyUpsampleAccuracy(t *testing.T) {
    // Create a 4x4 chroma grid with known values.
    // Compute expected output using libjpeg-turbo's h2v2_fancy_upsample C code.
    // Verify exact match, pixel by pixel.
}

func TestMergedUpsampleAccuracy(t *testing.T) {
    // Feed known Y, Cb, Cr rows into MergedUpsampler.
    // Compare output RGB pixels against independently computed reference.
}
```

#### 3.2.4 End-to-End Reference Tests

```go
func TestDecodeVsLibjpegTurboPPM(t *testing.T) {
    // For each test JPEG file:
    //   1. Decode with djpeg-go to get raw pixels
    //   2. Read the reference PPM produced by libjpeg-turbo's djpeg -dct int
    //   3. Compare pixel by pixel
    //   4. Assert max pixel difference == 0
    //
    // Test files should cover:
    //   - Grayscale (1x1 sampling)
    //   - YCbCr 4:4:4 (h1v1 sampling)
    //   - YCbCr 4:2:2 (h2v1 sampling)
    //   - YCbCr 4:2:0 (h2v2 sampling)
    //   - Various quantization table values
    //   - Restart markers
}
```

#### 3.2.5 Make Pipeline Stages Independently Callable

Each pipeline stage should be testable with constructed input data:

```go
// In internal/huff/huffman_test.go:
func TestDecodeMCUSequentialKnownInput(t *testing.T) {
    // Construct a bitstream with known Huffman-encoded MCU data.
    // Call DecodeMCUSequential.
    // Verify the decoded coefficient blocks match expected values.
}

// In internal/decoder/pipeline_test.go:
func TestIDCTOnly(t *testing.T) {
    // Create a coefficient block with known DC/AC values.
    // Call PerformIDCT.
    // Verify output samples match reference.
}
```

---

## 4. Accuracy Fix Plan (0 Pixel Error)

### 4.1 Current Status

Max pixel difference: 2 vs libjpeg-turbo reference. The goal is exact 0 match.

### 4.2 Root Cause Analysis

The max=2 pixel error most likely comes from one or more of these sources:

#### 4.2.1 IDCT Range Limiting Discrepancy

**File:** `internal/huff/idct_int.go`, line 262-265

```go
func rangeLimitGet(rl *RangeLimitTable, value int) JSAMPLE {
    idx := value & RangeMask
    return rl[idx]
}
```

**Problem:** The `RangeLimitTable` layout in `huff/types.go:108-132` uses
`RangeSubset` (= 384) as the offset for the identity region. The C code uses
`RANGE_CENTER` (= 512) as the bias added inside the IDCT, then indexes
`sample_range_limit - CENTERJSAMPLE`, which is effectively offset by
`CENTERJSAMPLE << RANGE_BITS` = 512. The table offset must match EXACTLY.

**Check:** Verify that the RangeLimitTable indices used by `rangeLimitGet`
exactly match the C code's `range_limit[RIGHT_SHIFT(...) & RANGE_MASK]` pattern
where `range_limit = cinfo->sample_range_limit - CENTERJSAMPLE`.

**File:** `internal/huff/idct_int.go`, lines 173-175 and 243-250:

```go
// Pass 2 even-part setup:
z2 := workspace[wsptr+0] +
    (int32(RangeCenter)<<(islowPass1Bits+3))+
    (int32(1)<<(islowPass1Bits+2))
```

This adds `RANGE_CENTER << 5 = 512 << 5 = 16384` plus fudge `1 << 4 = 16` to
the DC term. The C code does:

```c
z2 = (INT32) wsptr[0] +
    ((((INT32) RANGE_CENTER) << (PASS1_BITS+3)) + (ONE << (PASS1_BITS+2)));
```

Verify: `PASS1_BITS = 2`, so `RANGE_CENTER << 5 = 16384` and `ONE << 4 = 16`.
This appears correct.

**File:** `internal/huff/idct_int.go`, line 243:

```go
outptr[outIdx+0] = rangeLimitGet(rangeLimit, int((tmp10+tmp3)>>(islowConstBits+islowPass1Bits+3)))
```

The total right shift is `13 + 2 + 3 = 18`. The C code does
`RIGHT_SHIFT(tmp10+tmp3, CONST_BITS+PASS1_BITS+3)` = 18. This appears correct.

**BUT:** The C code indexes the result as:
```c
range_limit[RIGHT_SHIFT(tmp10 + tmp3, CONST_BITS+PASS1_BITS+3) & RANGE_MASK]
```

The `& RANGE_MASK` (0x3FF = 1023) is applied BEFORE the table lookup. In our
Go code, `rangeLimitGet` does `value & RangeMask`. Since `RangeMask = 1023`,
this should match. But the table layout must also match.

**Fix 1:** Verify the RangeLimitTable entries at every relevant index. The C
table is:

```
Index 0..383:    0 (negative values clamped to black)
Index 384..639:  0..255 (identity)
Index 640..1023: 255 (positive values clamped to white)
```

Our Go table in `huff/types.go:108-132` uses `RangeSubset = 384` as the
identity start, which is correct. But compare the EXACT values at boundary
indices.

#### 4.2.2 Fancy Upsampling Formula Difference

**File:** `internal/decoder/decoder.go`, lines 509-583

The inline h2v2 fancy upsampling in `upsampleAndConvert` computes chroma values
differently from the C code's `h2v2_fancy_upsample` in libjpeg's `jdsample.c`.

The C code for h2v2 fancy horizontal interpolation is:

```c
outptr0[0] = (JSAMPLE) ((inptr0[0] * 3 + inptr0[1] + 2) >> 2);
// for middle columns:
outptr0[outcol] = (JSAMPLE) ((inptr0[thiscol] * 3 + inptr0[thiscol-1] + 2) >> 2);
outptr0[outcol+1] = (JSAMPLE) ((inptr0[thiscol] * 3 + inptr0[thiscol+1] + 2) >> 2);
```

Then vertical interpolation is SEPARATE from horizontal:

```c
outptr0[outcol] = range_limit[...]; // after horizontal
// Then vertical interpolation in a second pass:
outptr1 = above row weighted average
```

**In our Go code**, the decoder does H+V interpolation simultaneously in a
combined formula (lines 549-563). This combined approach may produce different
rounding than the C code's two-pass approach.

**Specific issue:** The C code's h2v2_fancy_upsample does horizontal
interpolation FIRST into a temporary buffer, THEN does vertical interpolation
from the horizontally-interpolated values. Our code does both in one step:

```go
// Our combined H+V (decoder.go:568-573):
if evenRow {
    cb = (3*cbH + cbHA + 2) >> 2  // cbH is already H-interpolated
    cr = (3*crH + crHA + 2) >> 2
}
```

The rounding in `(3*A + B + 2) >> 2` applied TWICE (once for H, once for V)
can accumulate differently than doing it once on combined values.

**Fix 2:** Refactor the fancy upsampling to exactly match the C code's
two-pass structure:
1. First pass: horizontal fancy upsampling for each of the 3 context rows
   (above, current, below).
2. Second pass: vertical fancy upsampling from the H-upsampled rows.

This is what the `color` package's `H2V2FancyUpsample` function does, but the
`decoder` package doesn't use it. After refactoring (Section 2), the decoder
should delegate to `color.Upsampler` or `color.MergedUpsampler`, which have
the correct two-pass logic.

#### 4.2.3 Restart Marker Handling

**File:** `internal/decoder/decoder.go`, lines 278-316

The `readAllScanData` function strips restart markers (FF D0..D7) from the
scan data stream (line 293-297):

```go
if next >= 0xD0 && next <= 0xD7 {
    // Restart marker: strip it from the stream.
    copy(buf[i:], buf[i+2:])
    end -= 2
    continue
}
```

But the Huffman decoder's `fillBitBuffer` in `huff/huffman.go:109-188` handles
byte stuffing (FF 00 -> FF data byte) and marker detection internally. If
restart markers are already stripped, the decoder won't see them, and the DC
prediction reset at restart boundaries in `decodeIMCURow` (lines 337-345) may
not happen at the right positions.

The C code's approach is different: the entropy decoder processes restart
markers inline and resets state when they are encountered. Stripping them from
the bitstream before decoding means the restart interval counter and actual
data positions may get out of sync.

**Fix 3:** Either:
(a) Keep restart markers in the scan data and handle them in the entropy
    decoder's bit reader (matching C behavior), or
(b) Track restart positions explicitly when stripping markers and reset DC
    state at the correct MCU boundaries.

Option (a) is simpler and more faithful to the C code.

#### 4.2.4 Quantization Table Value Type Mismatch

**File:** `internal/decoder/decoder.go`, line 758

```go
dec.quantTables[ci][i] = int32(qt.QuantVal[i])
```

The `marker.QuantTable.QuantVal` is `uint16`. Casting to `int32` is fine for
values up to 32767, but if the ISLOW IDCT expects unsigned values and the
multiplication overflows differently with signed vs unsigned intermediate
results, this could cause errors.

The C code uses `INT32` (signed) for IDCT multipliers, and the quantization
values are `UINT16`. The multiplication `coef * quantval` in C is done with
signed arithmetic (JCOEF is `short`, quant table value is loaded as INT32).
Our code should match this exactly.

**Fix 4:** Verify that all intermediate arithmetic in `idct_int.go` uses the
same signed/unsigned semantics as the C code. Specifically, check that
`multiply16c16` produces the same results for negative coefficients as the C
`MULTIPLY` macro.

#### 4.2.5 Color Conversion Rounding

**File:** `internal/color/color.go`, lines 146-156

```go
cc.CrRTab[i] = (fix(1.402) * x + OneHalf) >> ScaleBits
```

The `fix()` function in `color/types.go:138`:

```go
func fix(x float64) int32 {
    return int32(x*65536.0 + 0.5)
}
```

The `+ 0.5` rounding in `fix()` may differ from the C code's integer constant
definitions. The C code uses:

```c
#define FIX(x)  ((INT32) ((x) * (1L<<SCALEBITS) + 0.5))
```

This is the same formula. But the key question is whether `fix(1.402)` produces
the EXACT same int32 value as the C preprocessor. Let's verify:

```
fix(1.402) = int32(1.402 * 65536.0 + 0.5) = int32(91910.272 + 0.5) = int32(91910.772) = 91910
C:        FIX(1.402) = (INT32)(1.402 * 65536 + 0.5) = 91910
```

These match. But what about:

```
fix(0.714136286) = int32(0.714136286 * 65536.0 + 0.5) = int32(46803.26... + 0.5) = 46803
```

The C code uses `-FIX(0.714136286)` for CrGTab. Since the C FIX macro and our
Go `fix()` function use the same formula, these should match.

**Fix 5:** Create a test that computes all the `fix()` values and compares them
against the known C preprocessor output. Verify every coefficient constant.

#### 4.2.6 Range Limit Table in decoder.go

**File:** `internal/decoder/decoder.go`, lines 696-705

```go
func rangeLimitClamp(rl *huff.RangeLimitTable, v int) byte {
    if v < 0 {
        return 0
    }
    if v > 255 {
        return 255
    }
    return byte(v)
}
```

This simple clamping function is used in `upsampleAndConvert` for YCbCr->RGB
output. But the IDCT output goes through the `RangeLimitTable` via
`rangeLimitGet`. If the YCbCr->RGB conversion adds a value that pushes the
result outside [0, 255], this simple clamping should produce the same result
as the C code's range_limit table.

The C code for YCbCr->RGB conversion uses:

```c
r = range_limit[y + Crrtab[Cr]];
```

where `range_limit` is the same table used by the IDCT. This table already has
the CENTERJSAMPLE offset baked in. But in our decoder, after the IDCT, the
Y/Cb/Cr values are already in [0, 255] range (they are `uint8`/`JSAMPLE`). The
color conversion then adds signed offsets from the lookup tables. The result
can go below 0 or above 255.

The C code uses `range_limit[y + Crrtab[Cr]]` where `Crrtab[Cr]` is already
the descaled Cr->R contribution. The range_limit table has entries for
negative indices (via the -CENTERJSAMPLE offset). Our simple clamp should be
equivalent because:

- `y` is in [0, 255]
- `Crrtab[Cr]` is in roughly [-180, +180]
- Sum is in roughly [-180, 435]
- Clamp to [0, 255] is equivalent to the table lookup

**But:** The C code's range_limit lookup uses `& RANGE_MASK` (bitwise AND with
1023), not simple clamping. For values outside [-384, 639], the table wraps
around. However, in practice, the sums stay within [-384, 639], so clamping
and table lookup should produce the same result.

**Fix 6:** To guarantee 0 error, replace the simple `rangeLimitClamp` with a
table lookup that exactly matches the C code's pattern:

```go
func rangeLimitLookup(rl *types.RangeLimitTable, v int) byte {
    idx := (v + types.RangeSubset) & types.RangeMask
    return rl[idx]
}
```

This matches `range_limit[y + Crrtab[Cr]]` where `range_limit` is offset by
`CENTERJSAMPLE << RANGE_BITS`.

### 4.3 Prioritized Fix List

| Priority | Fix | Location | Expected Impact |
|----------|-----|----------|-----------------|
| P0 | Fix fancy upsampling to use two-pass C algorithm | `decoder.go:426-694` | Likely fixes max=2 error |
| P1 | Use table-based range limiting in YCbCr->RGB | `decoder.go:696-705` | May fix edge-case errors |
| P2 | Fix restart marker handling | `decoder.go:256-316` | Fixes images with restart markers |
| P3 | Verify all fix() constants match C preprocessor | `color/types.go:138` | Prevents subtle rounding errors |
| P4 | Add IEEE 1180 IDCT validation tests | New test file | Validates IDCT accuracy |

### 4.4 Verification Protocol

After each fix, run:

```bash
# Generate reference PPM with libjpeg-turbo:
djpeg -dct int -ppm test_image.jpg > reference.ppm

# Generate our output:
go run ./cmd/djpeg -dct int -ppm test_image.jpg > our_output.ppm

# Compare:
cmp reference.ppm our_output.ppm  # Must be identical
```

Test with these JPEG files:
- `test_gray.jpg` (1x1 sampling, grayscale)
- `test_color.jpg` (1x1 sampling, YCbCr 4:4:4)
- `test_420.jpg` (2x2 sampling, YCbCr 4:2:0)
- A 4:2:2 test image (create if not present)
- A restart-marker test image (create with `cjpeg -restart 1`)

---

## 5. Package Reorganization

### 5.1 Current Package Layout

```
internal/
  jpeg/     (8 files) -- Core types, errors, memory, datasource
  marker/   (6 files) -- Marker parsing, decompressor API, input ctl, master
  huff/     (8 files) -- Huffman/arith decode, IDCT
  color/    (7 files) -- Color convert, upsampling, coef ctl, main ctl, merged
  output/   (7 files) -- PPM/BMP/GIF/TGA/RLE writers
  decoder/  (1 file)  -- Pipeline integration (monolithic)
```

### 5.2 Proposed Package Layout

```
internal/
  types/       -- Canonical shared types (NEW)
    component.go
    quant.go
    huffman.go
    colorspace.go
    dct.go         -- DCT constants, natural order tables, IDCT method
    range.go       -- Range limit table and constants
    scan.go        -- Block, JCOEF, JSAMPLE types

  source/      -- Data source management (from jpeg/)
    datasource.go  -- io.Reader adapter, buffer management

  marker/      -- Marker parsing (REDUCED, no duplicate types)
    reader.go      -- Marker reading (from marker_reader.go)
    input.go       -- Input controller (from input_controller.go)
    master.go      -- Master decompressor (from master.go + api.go)
    natural.go     -- Natural order tables (or import from types/)

  entropy/     -- Entropy decoding (from huff/, RENAMED)
    huffman.go     -- Huffman decoding (from huff/huffman.go)
    arith.go       -- Arithmetic decoding (from huff/arith.go)
    arith_tab.go   -- Arithmetic tables (from huff/arith_tab.go)

  idct/        -- Inverse DCT (from huff/, SPLIT OUT)
    manager.go     -- IDCT dispatch, multiplier tables (from huff/idct.go)
    islow.go       -- Accurate integer IDCT (from huff/idct_int.go)
    ifast.go       -- Fast integer IDCT (from huff/idct_fst.go)
    float.go       -- Floating-point IDCT (from huff/idct_flt.go)

  color/       -- Color conversion + upsampling (REDUCED, no duplicate types)
    convert.go     -- YCbCr->RGB etc. (from color/color.go)
    upsample.go    -- Simple upsampling (from color/upsample.go)
    merge.go       -- Merged upsampling (from color/merge.go)

  pipeline/    -- Pipeline orchestration (NEW, replaces decoder/)
    context.go     -- DecompressContext
    interfaces.go  -- Pipeline interfaces
    builder.go     -- Pipeline construction logic

  coef/        -- Coefficient controller (from color/coef.go)
    coef.go        -- CoefController

  buffer/      -- Main buffer controller (from color/mainctl.go)
    mainctl.go     -- MainController

  post/        -- Post-processing (from color/postproc.go)
    postproc.go    -- PostProcessor

  output/      -- Output writers (UNCHANGED)
    output.go
    ppm.go
    bmp.go
    gif.go
    targa.go
    rle.go
    colormap.go

  jpeg/        -- Error handling + legacy compat (REDUCED)
    errors.go      -- Error codes and message table
    memory.go      -- Virtual array support (until no longer needed)
```

### 5.3 Migration Order

1. **Create `internal/types/`** -- extract shared types from all packages.
   All other packages compile unchanged (they import `types` alongside their
   local definitions).

2. **Create `internal/idct/`** -- split IDCT code out of `huff/`.
   The `huff` package becomes `entropy/`.

3. **Create `internal/pipeline/`** -- extract the pipeline wiring from
   `decoder/decoder.go` into small, testable functions.

4. **Deprecate `internal/jpeg/`** -- move `DataSource` to `source/`, keep
   error handling in `jpeg/` until all consumers are migrated.

5. **Deprecate old `decoder/`** -- replace with `pipeline/` based
   implementation. The `DecodeToRGB` convenience function moves to a thin
   wrapper in `pipeline/`.

### 5.4 Import Graph (Target State)

```
cmd/djpeg/main.go
    --> internal/pipeline/   (DecodeToRGB, NewPipeline)
    --> internal/output/     (Writer, Format)

internal/pipeline/
    --> internal/types/      (shared types)
    --> internal/marker/     (header parsing, decompressor)
    --> internal/entropy/    (Huffman/arithmetic decoding)
    --> internal/idct/       (IDCT implementations)
    --> internal/color/      (color conversion, upsampling)
    --> internal/coef/       (coefficient controller)
    --> internal/buffer/     (main buffer controller)
    --> internal/post/       (post-processing)
    --> internal/source/     (data source)

internal/marker/
    --> internal/types/      (shared types)
    --> internal/source/     (io.Reader adapter)

internal/entropy/
    --> internal/types/      (shared types)

internal/idct/
    --> internal/types/      (shared types)

internal/color/
    --> internal/types/      (shared types)
```

No circular dependencies. Each package is independently testable.

---

## 6. Implementation Checklist

### Phase 1: Foundation (no behavior changes)

- [ ] Create `internal/types/` with unified types
- [ ] Add type aliases in `marker`, `huff`, `color` pointing to `types`
- [ ] Verify all tests still pass
- [ ] Add IEEE 1180 IDCT accuracy tests
- [ ] Add YCbCr->RGB pixel-level accuracy tests
- [ ] Add reference PPM comparison test infrastructure

### Phase 2: Accuracy Fixes

- [ ] Fix fancy upsampling in `decoder.go` to use two-pass algorithm
  (or delegate to `color.Upsampler`)
- [ ] Replace `rangeLimitClamp` with table-based lookup matching C code
- [ ] Fix restart marker handling in scan data reading
- [ ] Verify all `fix()` constants match C preprocessor output
- [ ] Run reference comparison tests against libjpeg-turbo output
- [ ] Achieve 0 max pixel error on all test images

### Phase 3: Pipeline Refactoring

- [ ] Create `internal/pipeline/` with `DecompressContext` and interfaces
- [ ] Split `huff/` into `internal/entropy/` and `internal/idct/`
- [ ] Extract coefficient controller to `internal/coef/`
- [ ] Extract main controller to `internal/buffer/`
- [ ] Extract post-processor to `internal/post/`
- [ ] Wire pipeline through interfaces in `pipeline/builder.go`
- [ ] Update `cmd/djpeg/main.go` to use new pipeline
- [ ] Remove old `decoder/` package

### Phase 4: Cleanup

- [ ] Remove `internal/jpeg/` after all consumers migrated
- [ ] Remove type aliases from Phase 1
- [ ] Remove duplicate constant definitions
- [ ] Remove duplicate natural order tables
- [ ] Verify full test suite passes
- [ ] Verify 0 pixel error on comprehensive test set

---

## 7. Key File References

### Files with accuracy issues

| File | Lines | Issue |
|------|-------|-------|
| `internal/decoder/decoder.go` | 426-694 | Inline fancy upsampling differs from C two-pass |
| `internal/decoder/decoder.go` | 696-705 | Simple clamp vs table-based range limit |
| `internal/decoder/decoder.go` | 278-316 | Restart marker stripping may break MCU sync |
| `internal/color/color.go` | 146-156 | Verify fix() constants match C exactly |

### Files with duplicate types

| File | Lines | Duplicate Type |
|------|-------|----------------|
| `internal/jpeg/types.go` | 98-138 | `JPEGComponentInfo` |
| `internal/marker/types.go` | 160-193 | `ComponentInfo` |
| `internal/color/types.go` | 39-57 | `ComponentInfo` |
| `internal/huff/types.go` | 61-78 | `ComponentInfo` |
| `internal/jpeg/types.go` | 74-80 | `JQuantTbl` |
| `internal/marker/types.go` | 147-150 | `QuantTable` |
| `internal/color/types.go` | 60-62 | `QuantTable` |
| `internal/jpeg/types.go` | 83-91 | `JHuffTbl` |
| `internal/marker/types.go` | 153-157 | `HuffTable` |
| `internal/huff/types.go` | 165-169 | `HuffmanTable` |

### Files with duplicate constants

| File | Lines | Duplicate Constants |
|------|-------|---------------------|
| `internal/jpeg/constants.go` | 6-17 | DCTSize, DCTSize2, NumQuantTbls, etc. |
| `internal/marker/types.go` | 68-80 | Same constants |
| `internal/huff/types.go` | 17-26 | Same constants |
| `internal/color/types.go` | 120-134 | Same constants |

### Files with duplicate natural order tables

| File | Lines | Tables |
|------|-------|--------|
| `internal/jpeg/types.go` | 604-672 | `JPEGNaturalOrder` + 6 variants |
| `internal/marker/natural_order.go` | 1-71 | `NaturalOrder` + 6 variants |
| `internal/huff/types.go` | 250-262 | `NaturalOrder` (1 table only) |

### Files with pipeline logic that should use color/ package

| File | Lines | Logic |
|------|-------|-------|
| `internal/decoder/decoder.go` | 426-694 | Fancy upsampling (should use `color.Upsampler` or `color.MergedUpsampler`) |
| `internal/decoder/decoder.go` | 784-883 | Color pipeline setup (builds `color.DecompressInfo` by hand-copying fields) |
| `internal/decoder/decoder.go` | 370-421 | Block routing (should use `color.CoefController`) |

---

## 8. Summary

The path to 0 pixel error runs through three specific fixes:

1. **Fix the fancy upsampling algorithm** in `decoder.go` to match the C code's
   two-pass (horizontal then vertical) structure instead of the current
   combined formula.

2. **Use table-based range limiting** for YCbCr->RGB conversion instead of
   simple min/max clamping.

3. **Fix restart marker handling** to preserve DC prediction synchronization.

The path to maximum testability runs through:

1. **Unify types** into `internal/types/` to eliminate conversion overhead.
2. **Define pipeline interfaces** so each stage can be tested in isolation.
3. **Split the monolithic decoder** into `pipeline/`, `entropy/`, `idct/`,
   `coef/`, `buffer/`, `post/` packages.
4. **Add reference comparison tests** against libjpeg-turbo PPM output.
