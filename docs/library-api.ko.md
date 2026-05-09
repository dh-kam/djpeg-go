# 공개 라이브러리 API

이 문서는 root 모듈에서 노출하는 public facade를 설명합니다.

```go
import libjpeg "github.com/dh-kam/djpeg-go"
```

공개 API는 의도적으로 작게 유지합니다. `internal/*` 패키지는 숨기고, 외부 Go
모듈이 libjpeg 스타일 JPEG decompressor 동작을 사용할 수 있는 안정적인 진입점만
제공합니다. `cmd/djpeg` 명령은 frontend와 parity 도구이며 주 라이브러리 계약은
아닙니다.

## 어떤 API를 쓸지 선택하기

Go 표준 image 생태계와 함께 쓰려면 `Decode`를 사용합니다.

```go
img, err := libjpeg.Decode(r)
```

정확한 byte layout이 중요하면 `DecodeRaster`를 사용합니다. renderer, pixel 비교,
직접 Gray8 또는 RGB24 데이터를 다루는 코드에 적합합니다.

```go
raster, err := libjpeg.DecodeRaster(r)
```

scanline을 직접 가져와야 하면 `NewDecoder`를 사용합니다. 이 API는 format writer나
transcoder에 유용하지만 bounded-memory streaming을 보장하지 않습니다. 현재
decoder는 scanline을 제공하기 전에 압축 scan payload를 내부적으로 읽습니다.

```go
dec := libjpeg.NewDecoder(r)
```

## image.Image로 디코딩

`Decode`는 `image.Image`를 반환합니다. 현재 concrete type은 `image.Image`를
구현하는 `*Raster`입니다.

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

표준 `*image.RGBA`가 필요하면 raster로 디코딩한 뒤 `RGBA`를 호출합니다.

```go
raster, err := libjpeg.DecodeRaster(r)
if err != nil {
	return err
}
rgba := raster.RGBA()
```

## Raw Pixel로 디코딩

`DecodeRaster`는 top-down raster를 반환합니다.

```go
type Raster struct {
	Pix    []byte
	Stride int
	Rect   image.Rectangle
	Format PixelFormat
}
```

`Pix` layout은 `Format`에 따라 달라집니다.

- `PixelFormatGray8`: 픽셀당 1 byte, 일반적인 row stride는 `width`
- `PixelFormatRGB24`: 픽셀당 R, G, B byte, 일반적인 row stride는 `width*3`

row를 순회할 때는 항상 tightly packed라고 가정하지 말고 `Stride`를 사용하세요.

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
	}
}
```

## Metadata 읽기

표준 Go image metadata는 `DecodeConfig`로 읽습니다.

```go
cfg, err := libjpeg.DecodeConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.ColorModel)
```

raster 세부 정보까지 필요하면 `DecodeRasterConfig`를 사용합니다.

```go
cfg, err := libjpeg.DecodeRasterConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.PixelFormat, cfg.Stride)
```

`Config`는 다음 필드를 제공합니다.

```go
type Config struct {
	Width            int
	Height           int
	Components       int
	Stride           int
	PixelFormat      PixelFormat
	ColorSpace       ColorSpace
	InputComponents  int
	InputColorSpace  ColorSpace
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
}
```

## Options

대부분의 호출자는 functional option을 사용하면 됩니다. zero value는 CLI 기본값과
같습니다. 즉 integer IDCT, fancy upsampling enabled, IJG 호환 chroma IDCT
scaling입니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithIDCT(libjpeg.IDCTInt),
	libjpeg.WithNoSmooth(),
)
```

사용 가능한 IDCT method:

- `IDCTDefault`
- `IDCTInt`
- `IDCTFast`
- `IDCTFloat`

사용 가능한 upsampling mode:

- `UpsamplingDefault`
- `UpsamplingFancy`
- `UpsamplingNearest`

편의 option:

```go
libjpeg.WithNoSmooth()
libjpeg.WithFast()
libjpeg.WithGrayscaleOutput()
libjpeg.WithRGBOutput()
libjpeg.WithMaxMemory(20_000_000)
libjpeg.WithScale(1, 2)
```

`WithNoSmooth`는 CLI `--nosmooth`에 대응합니다. `WithFast`는 현재 구현된 CLI
`--fast` subset, 즉 fast integer IDCT와 nearest-neighbor chroma upsampling에
대응합니다.

libjpeg 스타일 `--grayscale` 또는 `--rgb` 동작이 필요하면 output color space를
강제할 수 있습니다.

```go
raster, err := libjpeg.DecodeRaster(r, libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceGray))
// 동등한 표현:
raster, err = libjpeg.DecodeRaster(r, libjpeg.WithGrayscaleOutput())
```

`WithMaxMemory`는 decoder가 소유한 compressed scan data와 component buffer에
대략적인 상한을 적용합니다. CLI 스타일 값은 `libjpeg.ParseMemoryLimit("20m")`로
파싱할 수 있고, suffix 없는 숫자는 libjpeg와 같이 kilobyte입니다.

`WithScale`은 libjpeg 스타일 output scaling을 적용합니다. 8x8 DCT JPEG에서는
ratio가 `1/8`부터 `16/8`까지의 지원 scale grid 중 가까운 값으로 매핑됩니다.
현재 구현은 먼저 decode한 뒤 output raster를 resampling합니다.

## Compatibility Mode

기본 compatibility mode는 현재 IJG 9f exact-parity 경로를 유지합니다.

```go
raster, err := libjpeg.DecodeRaster(r)
```

PDF DCT stream에서 Poppler 또는 ImageMagick 방식 출력과 맞춰야 하면 Poppler
compatibility preset을 사용합니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
)
```

이는 libjpeg 스타일 integer IDCT 기본값, 8x8 chroma IDCT, fancy upsampling을
사용하며 Poppler/ImageMagick 출력과 맞는 PDF fixture 계열을 대상으로 합니다.
기본값으로 바꾸면 IJG 9f exact parity가 깨질 수 있으므로 opt-in으로 유지합니다.

CLI/debug 도구 기준의 동등한 표현:

```bash
djpeg --compatibility poppler-pdf --ppm input.jpg > output.ppm
```

`WithTurboFancy()`와 `--turbo-fancy`는 deprecated alias로 유지합니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithIDCT(libjpeg.IDCTInt),
	libjpeg.WithTurboFancy(),
)
```

고급 사용자는 profile 중 chroma IDCT scaling만 직접 override할 수 있습니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
	libjpeg.WithChromaIDCTScaling(true),
)
```

RGB 계열 stream은 inverse color transform도 override할 수 있습니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithInputColorSpace(libjpeg.InputRGB),
	libjpeg.WithColorTransform(libjpeg.ColorTransformSubtractGreen),
)
```

## Input Color Space Override

PDF 같은 container에 포함된 JPEG stream은 JPEG byte stream 외부에 color space
metadata가 있을 수 있습니다. container가 sample 해석 방식을 알고 있다면
`WithInputColorSpace`를 사용합니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithInputColorSpace(libjpeg.InputRGB),
	libjpeg.WithTurboFancy(),
)
```

사용 가능한 input color space:

- `InputAuto`
- `InputGray`
- `InputRGB`
- `InputYCbCr`
- `InputCMYK`
- `InputYCCK`

기본값은 `InputAuto`이며 JPEG marker와 component ID를 사용합니다.

4-component PDF stream은 raw CMYK 출력을 요청할 수 있습니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithInputColorSpace(libjpeg.InputYCCK),
	libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceCMYK),
)
// raster.Format은 PixelFormatCMYK32입니다.
```

IJG compatibility를 위해 component ID가 Adobe APP14 transform hint보다
우선합니다. component ID가 애매하면 4-component stream에서 Adobe transform
`0`은 CMYK로, transform `2`와 알 수 없는 4-component transform 값은 YCCK로
추론합니다. 명시적인 `WithInputColorSpace`는 항상 이 marker 추론보다
우선합니다.

## Header State

`Config`에는 libjpeg field와 helper API에 대응하는 decompressor header state가
포함됩니다.

- `Baseline`, `Progressive`, `Arithmetic`은 SOF coding mode를 나타냅니다.
- `HasMultipleScans`는 `jpeg_has_multiple_scans()`에 대응합니다.
- `InputComplete`는 config가 만들어진 시점의 `jpeg_input_complete()` 값입니다.
- `SawJFIFMarker`, `JFIFMajorVersion`, `JFIFMinorVersion`, `DensityUnit`,
  `XDensity`, `YDensity`는 JFIF APP0 metadata입니다.
- `SawAdobeMarker`, `AdobeTransform`은 Adobe APP14 metadata입니다.

Scanline decoder에서도 같은 상태를 직접 확인할 수 있습니다.

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
```

`Abort`는 현재 decompression operation을 중단하고 public decoder state를
초기화합니다. 다른 stream을 디코딩하려면 새 reader로 decoder를 새로 만드세요.

```go
dec.Abort()
```

## Saved Markers

Scanline decoder는 header를 읽는 동안 APPn 또는 COM marker payload를 저장할
수 있습니다. 이는 libjpeg의 `jpeg_save_markers()` lifecycle과 같습니다.
`ReadHeader` 전에 저장할 marker를 지정하고, 이후 `Markers()`로 확인합니다.

```go
dec := libjpeg.NewDecoder(
	r,
	libjpeg.WithSavedMarkers(libjpeg.MarkerAPP14, 65533),
	libjpeg.WithSavedMarkers(libjpeg.MarkerCOM, 65533),
)

if _, err := dec.ReadHeader(); err != nil {
	return err
}

for _, marker := range dec.Markers() {
	_ = marker.Code
	_ = marker.OriginalLength
	_ = marker.Data
}
```

APP0부터 APP15까지와 COM marker만 허용합니다. APP0과 APP14는 저장하지 않아도
JFIF/Adobe 동작을 위해 내부적으로 계속 파싱됩니다.

## Tables

`ReadHeader` 이후에는 parsing된 DQT와 DHT table을 복사본으로 확인할 수 있습니다.
이는 libjpeg decompressor field인 `quant_tbl_ptrs`, `dc_huff_tbl_ptrs`,
`ac_huff_tbl_ptrs`에 대응하되, decoder가 소유한 mutable storage를 직접 노출하지
않습니다.

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
```

## Options Struct

설정을 저장해야 하는 코드에서는 `Options`와 `DecodeWithOptions` 또는
`DecodeRasterWithOptions`를 사용합니다.

```go
opts := &libjpeg.Options{
	IDCT:              libjpeg.IDCTInt,
	Upsampling:        libjpeg.UpsamplingFancy,
	Compatibility:     libjpeg.CompatibilityIJG9,
	InputColorSpace:   libjpeg.InputAuto,
	OutputColorSpace:  libjpeg.ColorSpaceUnknown,
	ColorTransform:    libjpeg.ColorTransformDefault,
	ChromaIDCTScaling: libjpeg.ChromaIDCTScalingDefault,
}

raster, err := libjpeg.DecodeRasterWithOptions(r, opts)
```

CLI 스타일 문자열은 다음 함수로 파싱할 수 있습니다.

```go
idct, err := libjpeg.ParseIDCTMethod("int")
space, err := libjpeg.ParseInputColorSpace("rgb")
```

## Scanline API

`NewDecoder`는 scanline 중심 lifecycle을 제공합니다.

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

if err := dec.Start(); err != nil {
	return err
}

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

`SkipScanlines`는 libjpeg의 `jpeg_skip_scanlines()`에 대응합니다. output
scanline cursor를 전진시키고, 이미지 하단에서 멈추며, 실제로 skip한 row 수를
반환합니다.

`Finish`를 호출하기 전에 예상 output row를 모두 읽으세요. 중간에 멈추면 decoder가
전체 이미지를 소비하지 않았다고 보고 "too little data" 오류를 반환할 수
있습니다.

이 API는 scanline-oriented API이지만 bounded-memory streaming 계약은 아닙니다.
다른 포맷으로 쓰는 동안 두 번째 전체 output image를 만들지 않기 위한 용도로
제공합니다. `WithScale`은 현재 full output raster를 buffering하고 resampling한 뒤
scaled scanline을 제공합니다.

## Error Handling

public package는 안정적인 sentinel error를 제공합니다.

```go
var (
	ErrInvalidJPEG   = errors.New("djpeg: invalid jpeg")
	ErrUnsupported   = errors.New("djpeg: unsupported jpeg feature")
	ErrInvalidOption = errors.New("djpeg: invalid option")
	ErrMemoryLimit   = errors.New("djpeg: memory limit exceeded")
)
```

`errors.Is`로 확인하세요.

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

현재 progressive JPEG와 arithmetic-coded JPEG는 지원하지 않습니다.

## CLI Flag와 API Option 대응

| CLI flag | Public API |
| --- | --- |
| `--dct int` | `libjpeg.WithIDCT(libjpeg.IDCTInt)` |
| `--dct fast` | `libjpeg.WithIDCT(libjpeg.IDCTFast)` |
| `--dct float` | `libjpeg.WithIDCT(libjpeg.IDCTFloat)` |
| `--fast` | `libjpeg.WithFast()` |
| `--nosmooth` | `libjpeg.WithNoSmooth()` |
| `--grayscale` | `libjpeg.WithGrayscaleOutput()` |
| `--rgb` | `libjpeg.WithRGBOutput()` |
| `--maxmemory 20m` | `limit, _ := libjpeg.ParseMemoryLimit("20m"); libjpeg.WithMaxMemory(limit)` |
| `--scale 1/2` | `libjpeg.WithScale(1, 2)` |
| `--compatibility poppler-pdf` | `libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF)` |
| `--turbo-fancy` | `libjpeg.WithTurboFancy()` deprecated alias |
| `--input-colorspace rgb` | `libjpeg.WithInputColorSpace(libjpeg.InputRGB)` |
| `--color-transform subtract-green` | `libjpeg.WithColorTransform(libjpeg.ColorTransformSubtractGreen)` |
| `--input-colorspace ycbcr` | `libjpeg.WithInputColorSpace(libjpeg.InputYCbCr)` |
| `--input-colorspace grayscale` | `libjpeg.WithInputColorSpace(libjpeg.InputGray)` |

`--ppm`, `--bmp`, `--targa`, `--rle`, `--gif` 같은 CLI output flag는 아직 public
decode facade에 포함하지 않았습니다. 라이브러리에서는 `DecodeRaster`로 픽셀을
얻은 뒤 application code에서 원하는 output format으로 쓰세요.

## 외부 모듈에서 사용하기

다른 모듈에서:

```bash
go get github.com/dh-kam/djpeg-go
```

root package를 import합니다.

```go
import libjpeg "github.com/dh-kam/djpeg-go"
```

`github.com/dh-kam/djpeg-go/internal/...`는 import하지 마세요. 해당 패키지는 구현
세부사항이며, Go의 `internal` package 규칙에 따라 외부 consumer가 사용할 수
없도록 의도적으로 막혀 있습니다.

## Compatibility Notes

- baseline, non-progressive JPEG가 지원 경로입니다.
- progressive JPEG와 arithmetic-coded JPEG는 `ErrUnsupported`를 반환합니다.
- `Decode`는 `image.Image` interface 뒤에 `*Raster`를 반환합니다. RGBA allocation을
  강제하지 않으면서 raw byte 접근을 유지하기 위한 선택입니다.
- package는 `image.RegisterFormat`를 자동 호출하지 않습니다. 이 decoder의 parity
  동작이 필요하면 `libjpeg.Decode`를 명시적으로 사용하세요.
- 기본 option set은 현재 random100 corpus 기준 IJG 9f exact parity를 유지하도록
  선택했습니다.
