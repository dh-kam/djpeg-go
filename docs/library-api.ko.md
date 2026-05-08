# 공개 라이브러리 API

이 문서는 root 모듈에서 노출하는 public facade를 설명합니다.

```go
import djpeg "github.com/dh-kam/djpeg-go"
```

공개 API는 의도적으로 작게 유지합니다. `internal/*` 패키지는 숨기고, 외부 Go
모듈이 IJG `djpeg`에 가까운 JPEG 디코딩 동작을 사용할 수 있는 안정적인 진입점만
제공합니다.

## 어떤 API를 쓸지 선택하기

Go 표준 image 생태계와 함께 쓰려면 `Decode`를 사용합니다.

```go
img, err := djpeg.Decode(r)
```

정확한 byte layout이 중요하면 `DecodeRaster`를 사용합니다. renderer, pixel 비교,
직접 Gray8 또는 RGB24 데이터를 다루는 코드에 적합합니다.

```go
raster, err := djpeg.DecodeRaster(r)
```

scanline을 직접 가져와야 하면 `NewDecoder`를 사용합니다. 이 API는 format writer나
transcoder에 유용하지만 bounded-memory streaming을 보장하지 않습니다. 현재
decoder는 scanline을 제공하기 전에 압축 scan payload를 내부적으로 읽습니다.

```go
dec := djpeg.NewDecoder(r)
```

## image.Image로 디코딩

`Decode`는 `image.Image`를 반환합니다. 현재 concrete type은 `image.Image`를
구현하는 `*djpeg.Raster`입니다.

```go
package main

import (
	"image/png"
	"os"

	djpeg "github.com/dh-kam/djpeg-go"
)

func main() {
	in, err := os.Open("input.jpg")
	if err != nil {
		panic(err)
	}
	defer in.Close()

	img, err := djpeg.Decode(in)
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
raster, err := djpeg.DecodeRaster(r)
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
raster, err := djpeg.DecodeRaster(r, djpeg.WithIDCT(djpeg.IDCTInt))
if err != nil {
	return err
}

width := raster.Rect.Dx()
height := raster.Rect.Dy()

for y := 0; y < height; y++ {
	row := raster.Pix[y*raster.Stride:]
	switch raster.Format {
	case djpeg.PixelFormatGray8:
		useGrayRow(row[:width])
	case djpeg.PixelFormatRGB24:
		useRGBRow(row[:width*3])
	}
}
```

## Metadata 읽기

표준 Go image metadata는 `DecodeConfig`로 읽습니다.

```go
cfg, err := djpeg.DecodeConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.ColorModel)
```

raster 세부 정보까지 필요하면 `DecodeRasterConfig`를 사용합니다.

```go
cfg, err := djpeg.DecodeRasterConfig(r)
if err != nil {
	return err
}
fmt.Println(cfg.Width, cfg.Height, cfg.PixelFormat, cfg.Stride)
```

`Config`는 다음 필드를 제공합니다.

```go
type Config struct {
	Width           int
	Height          int
	Components      int
	Stride          int
	PixelFormat     PixelFormat
	ColorSpace      ColorSpace
	InputComponents int
	InputColorSpace ColorSpace
}
```

## Options

대부분의 호출자는 functional option을 사용하면 됩니다. zero value는 CLI 기본값과
같습니다. 즉 integer IDCT, fancy upsampling enabled, IJG 호환 chroma IDCT
scaling입니다.

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithNoSmooth(),
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
djpeg.WithNoSmooth()
```

이는 CLI `--nosmooth`에 대응합니다.

## Compatibility Mode

기본 compatibility mode는 현재 IJG 9f exact-parity 경로를 유지합니다.

```go
raster, err := djpeg.DecodeRaster(r)
```

PDF DCT stream에서 Poppler 또는 ImageMagick 방식 출력과 맞춰야 하면
`WithTurboFancy`를 사용합니다.

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithTurboFancy(),
)
```

이는 CLI `--turbo-fancy`에 대응합니다. IJG 9f chroma IDCT scaling 대신 8x8
chroma IDCT와 fancy upsampling을 사용합니다. 기본값으로 바꾸면 IJG 9f exact
parity가 깨질 수 있으므로 opt-in으로 유지합니다.

동등한 enum 형태:

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithCompatibility(djpeg.CompatibilityPopplerPDF),
)
```

## Input Color Space Override

PDF 같은 container에 포함된 JPEG stream은 JPEG byte stream 외부에 color space
metadata가 있을 수 있습니다. container가 sample 해석 방식을 알고 있다면
`WithInputColorSpace`를 사용합니다.

```go
raster, err := djpeg.DecodeRaster(
	r,
	djpeg.WithInputColorSpace(djpeg.InputRGB),
	djpeg.WithTurboFancy(),
)
```

사용 가능한 input color space:

- `InputAuto`
- `InputGray`
- `InputRGB`
- `InputYCbCr`

기본값은 `InputAuto`이며 JPEG marker와 component ID를 사용합니다.

## Options Struct

설정을 저장해야 하는 코드에서는 `Options`와 `DecodeWithOptions` 또는
`DecodeRasterWithOptions`를 사용합니다.

```go
opts := &djpeg.Options{
	IDCT:            djpeg.IDCTInt,
	Upsampling:      djpeg.UpsamplingFancy,
	Compatibility:   djpeg.CompatibilityIJG9,
	InputColorSpace: djpeg.InputAuto,
}

raster, err := djpeg.DecodeRasterWithOptions(r, opts)
```

CLI 스타일 문자열은 다음 함수로 파싱할 수 있습니다.

```go
idct, err := djpeg.ParseIDCTMethod("int")
space, err := djpeg.ParseInputColorSpace("rgb")
```

## Scanline API

`NewDecoder`는 scanline 중심 lifecycle을 제공합니다.

```go
dec := djpeg.NewDecoder(
	r,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithNoSmooth(),
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
for y := 0; y < out.Height; y++ {
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

`Finish`를 호출하기 전에 예상 output row를 모두 읽으세요. 중간에 멈추면 decoder가
전체 이미지를 소비하지 않았다고 보고 "too little data" 오류를 반환할 수
있습니다.

이 API는 scanline-oriented API이지만 bounded-memory streaming 계약은 아닙니다.
다른 포맷으로 쓰는 동안 두 번째 전체 output image를 만들지 않기 위한 용도로
제공합니다.

## Error Handling

public package는 안정적인 sentinel error를 제공합니다.

```go
var (
	ErrInvalidJPEG   = errors.New("djpeg: invalid jpeg")
	ErrUnsupported   = errors.New("djpeg: unsupported jpeg feature")
	ErrInvalidOption = errors.New("djpeg: invalid option")
)
```

`errors.Is`로 확인하세요.

```go
img, err := djpeg.Decode(r)
if err != nil {
	switch {
	case errors.Is(err, djpeg.ErrUnsupported):
		return fmt.Errorf("JPEG feature is not supported: %w", err)
	case errors.Is(err, djpeg.ErrInvalidJPEG):
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
| `--dct int` | `djpeg.WithIDCT(djpeg.IDCTInt)` |
| `--dct fast` | `djpeg.WithIDCT(djpeg.IDCTFast)` |
| `--dct float` | `djpeg.WithIDCT(djpeg.IDCTFloat)` |
| `--nosmooth` | `djpeg.WithNoSmooth()` |
| `--turbo-fancy` | `djpeg.WithTurboFancy()` |
| `--input-colorspace rgb` | `djpeg.WithInputColorSpace(djpeg.InputRGB)` |
| `--input-colorspace ycbcr` | `djpeg.WithInputColorSpace(djpeg.InputYCbCr)` |
| `--input-colorspace grayscale` | `djpeg.WithInputColorSpace(djpeg.InputGray)` |

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
import djpeg "github.com/dh-kam/djpeg-go"
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
  동작이 필요하면 `djpeg.Decode`를 명시적으로 사용하세요.
- 기본 option set은 현재 random100 corpus 기준 IJG 9f exact parity를 유지하도록
  선택했습니다.
