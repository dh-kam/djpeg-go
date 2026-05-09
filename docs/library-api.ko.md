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
	Palette color.Palette // PixelFormatIndexed8에서 설정됨
}
```

`Pix` layout은 `Format`에 따라 달라집니다.

- `PixelFormatGray8`: 픽셀당 1 byte, 일반적인 row stride는 `width`
- `PixelFormatRGB24`: 픽셀당 R, G, B byte, 일반적인 row stride는 `width*3`
- `PixelFormatIndexed8`: 픽셀당 palette index 1 byte, `Raster.Palette` 확인

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
	case libjpeg.PixelFormatIndexed8:
		useIndexedRow(row[:width], raster.Palette)
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
	ImageWidth       int
	ImageHeight      int
	InputComponents  int
	InputColorSpace  ColorSpace
	DataPrecision    int
	MaxHSampFactor   int
	MaxVSampFactor   int
	MinDCTHScaledSize int
	MinDCTVScaledSize int
	BlockSize        int
	ScaleNum         uint
	ScaleDenom       uint
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
	RecOutbufHeight  int
	Quantized        bool
	DesiredNumColors int
	ActualNumColors  int
	RawDataOut       bool
	BufferedImage    bool
	OutputGamma      float64
	DoBlockSmoothing bool
	CCIR601Sampling  bool
	Scan             ScanParameters
}
```

`Width`와 `Height`는 선택된 output geometry입니다. `ImageWidth`와
`ImageHeight`는 JPEG header의 원본 크기를 유지합니다.

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
libjpeg.WithQuantizeColors(256)
libjpeg.WithRawDataOutput()
libjpeg.WithOutputGamma(2.2)
libjpeg.WithBlockSmoothing(false)
```

`WithNoSmooth`는 CLI `--nosmooth`에 대응합니다. `WithFast`는 현재 구현된 CLI
`--fast` subset, 즉 fast integer IDCT와 nearest-neighbor chroma upsampling에
대응합니다.

libjpeg 스타일 `--grayscale` 또는 `--rgb` 동작이 필요하거나 YCbCr sample처럼
libjpeg null-conversion output이 필요하면 output color space를 강제할 수 있습니다.

```go
raster, err := libjpeg.DecodeRaster(r, libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceGray))
// 동등한 표현:
raster, err = libjpeg.DecodeRaster(r, libjpeg.WithGrayscaleOutput())

ycbcr, err := libjpeg.DecodeRaster(r, libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceYCbCr))
```

YCbCr 및 big-gamut YCbCr output은 JPEG input color space가 요청한 output color
space와 일치할 때만 지원합니다.

`WithMaxMemory`는 decoder가 소유한 compressed scan data와 component buffer에
대략적인 상한을 적용합니다. CLI 스타일 값은 `libjpeg.ParseMemoryLimit("20m")`로
파싱할 수 있고, suffix 없는 숫자는 libjpeg와 같이 kilobyte입니다.

`WithScale`은 libjpeg 스타일 output scaling을 적용합니다. 8x8 DCT JPEG에서는
ratio가 `1/8`부터 `16/8`까지의 지원 scale grid 중 가까운 값으로 매핑됩니다.
현재 구현은 먼저 decode한 뒤 output raster를 resampling합니다.

`WithOutputGamma`와 `WithBlockSmoothing`은 libjpeg decompressor parameter에
대응합니다. Block smoothing은 progressive output pass에서 의미가 있으며,
현재 decoder는 progressive header를 읽을 수 있지만 progressive scanline 및
coefficient decoding은 아직 `ErrUnsupported`로 보고합니다.

## Buffered-Image Output Pass

`WithBufferedImage`는 libjpeg의 `buffered_image` mode를 public facade 수준에서
대응합니다. `StartDecompress` 이후 `StartOutput`을 호출하고 scanline을 읽은 다음
`FinishOutput`을 호출합니다. decoder는 baseline image raster를 보관하므로 output
pass를 반복하거나 pass 사이에서 quantized colormap을 바꿀 수 있습니다.

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithBufferedImage())
if _, err := dec.ReadHeader(); err != nil {
	return err
}
if err := dec.StartDecompress(); err != nil {
	return err
}

if ok, err := dec.StartOutput(dec.InputScanNumber()); err != nil || !ok {
	return err
}
row := make([]byte, dec.OutputConfig().Stride)
for dec.OutputScanline() < dec.OutputConfig().Height {
	if _, err := dec.ReadScanlines([][]byte{row}); err != nil {
		return err
	}
	// 다음 ReadScanlines 호출 전에 row를 사용합니다.
}
if ok, err := dec.FinishOutput(); err != nil || !ok {
	return err
}
return dec.FinishDecompress()
```

현재는 sequential JPEG replay path입니다. Progressive header는 읽을 수 있지만
decompression 시작 시 progressive input은 아직 `ErrUnsupported`로 보고되므로
progressive incremental display는 미지원입니다.

## Quantized Output

`WithQuantizeColors`는 libjpeg decompressor parameter인 `quantize_colors`와
`desired_number_of_colors`에 대응합니다. 반환 raster는 `PixelFormatIndexed8`이고
Go `color.Palette`를 함께 제공합니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithQuantizeColors(64),
	libjpeg.WithDitherMode(libjpeg.DitherFloydSteinberg),
)
if err != nil {
	return err
}

_ = raster.Palette
```

Generated RGB palette는 기본적으로 libjpeg 스타일의 image-derived two-pass
선택을 사용합니다. `djpeg --onepass` 플래그와 같은 더 빠른 fixed color-cube
경로가 필요하면 `WithQuantizationMode(libjpeg.QuantizationOnePass)`를 사용합니다.
IJG libjpeg와 동일하게 generated RGB palette는 최소 8개 이상의 requested color가
필요하며, external palette는 하나 이상의 entry를 가질 수 있습니다.

external colormap mode가 필요하면 palette를 직접 넘깁니다.

```go
raster, err := libjpeg.DecodeRaster(
	r,
	libjpeg.WithColormap(color.Palette{
		color.RGBA{0, 0, 0, 255},
		color.RGBA{255, 255, 255, 255},
	}),
	libjpeg.WithDitherMode(libjpeg.DitherNone),
)
```

`Decoder.NewColormap`은 buffered quantized raster 경로에서 libjpeg의
`jpeg_new_colormap()`에 대응합니다. 새 external palette로 전환하고
`OutputScanline`을 0으로 되돌려 다른 indexed pass를 출력할 수 있게 합니다.

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithQuantizeColors(64))
if _, err := dec.ReadHeader(); err != nil {
	return err
}
if err := dec.StartDecompress(); err != nil {
	return err
}

if err := dec.NewColormap(color.Palette{
	color.RGBA{0, 0, 0, 255},
	color.RGBA{255, 255, 255, 255},
}); err != nil {
	return err
}
```

Quantized output은 현재 grayscale과 RGB output을 지원합니다. CMYK/YCCK
quantized output은 `ErrUnsupported`를 반환합니다.

## Coefficient Output

`DecodeCoefficients`와 `Decoder.ReadCoefficients`는 baseline
`jpeg_read_coefficients()` 경로에 대응합니다. 반환 block은 IDCT, color
conversion, upsampling 이전의 quantized DCT coefficient이며 natural row-major
순서입니다.

```go
components, cfg, err := libjpeg.DecodeCoefficients(r)
if err != nil {
	return err
}
_ = cfg

for _, component := range components {
	for by := 0; by < component.HeightInBlocks; by++ {
		row := component.Blocks[by*component.WidthInBlocks : (by+1)*component.WidthInBlocks]
		useCoefficientBlocks(component.Component.Index, row)
	}
}
```

decoder lifecycle에서 직접 사용하려면 다음처럼 호출합니다.

```go
dec := libjpeg.NewDecoder(r)
if _, err := dec.ReadHeader(); err != nil {
	return err
}
components, err := dec.ReadCoefficients()
if err != nil {
	return err
}
_ = components
```

Progressive coefficient decoding은 아직 `ErrUnsupported`로 보고됩니다.
Sequential arithmetic-coded coefficient decoding은 지원합니다.

## Raw Component Output

`DecodeRawComponents`와 `Decoder.ReadRawData`는 libjpeg의 `raw_data_out` /
`jpeg_read_raw_data()` 경로에 대응합니다. 반환되는 component는 color conversion과
upsampling 이전의 downsampled plane입니다.

```go
components, cfg, err := libjpeg.DecodeRawComponents(r)
if err != nil {
	return err
}
_ = cfg.RawDataOut

for _, component := range components {
	for y := 0; y < component.Height; y++ {
		row := component.Pix[y*component.Stride : y*component.Stride+component.Width]
		useRawComponentRow(component.Component.Index, row)
	}
}
```

scanline lifecycle에서 직접 사용하려면 다음처럼 호출합니다.

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithRawDataOutput())
if _, err := dec.ReadHeader(); err != nil {
	return err
}
if err := dec.StartDecompress(); err != nil {
	return err
}
components, err := dec.ReadRawData()
if err != nil {
	return err
}
_ = components
return dec.FinishDecompress()
```

`jpeg_read_raw_data()` 호출 단위와 맞추려면 `ReadRawDataRows`를 사용합니다.
`RawDataLinesPerIMCURow`는 필요한 `maxLines` 값을 반환합니다.

```go
linesPerIMCU := dec.RawDataLinesPerIMCURow()
for dec.OutputScanline() < dec.OutputConfig().Height {
	components, rows, err := dec.ReadRawDataRows(linesPerIMCU)
	if err != nil {
		return err
	}
	if rows == 0 {
		break
	}
	useRawIMCURow(components)
}
```

Raw component output은 quantized output과 함께 사용할 수 없습니다. facade의
post-decode `WithScale` 경로도 raw mode에서는 비활성화됩니다.

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

## Mutable Decoder Parameters

`NewDecoder`는 C API lifecycle을 선호하는 호출자를 위해 libjpeg 스타일 setter도
제공합니다. Header를 읽고 container metadata를 확인한 뒤,
`StartDecompress` 전에 decompression parameter를 설정할 수 있습니다.

```go
dec := libjpeg.NewDecoder(r)
header, err := dec.ReadHeader()
if err != nil {
	return err
}

if pdfColorSpaceIsRGB(header) {
	if err := dec.SetInputColorSpace(libjpeg.InputRGB); err != nil {
		return err
	}
}
if err := dec.SetOutputColorSpace(libjpeg.ColorSpaceRGB); err != nil {
	return err
}
if err := dec.SetCompatibility(libjpeg.CompatibilityPopplerPDF); err != nil {
	return err
}

if err := dec.StartDecompress(); err != nil {
	return err
}
```

현재 mutable method는 `SetIDCT`, `SetUpsampling`, `SetCompatibility`,
`SetChromaIDCTScaling`, `SetInputColorSpace`, `SetOutputColorSpace`,
`SetColorTransform`, `SetScale`, `SetRawDataOutput`, `SetBufferedImage`,
`SetMaxMemory`, `SetOutputGamma`, `SetBlockSmoothing`, `SetQuantizeColors`,
`SetDitherMode`, `SetColormap`입니다. 모두 `StartDecompress` 전에 호출해야
하며, output이 시작된 뒤 변경하면 `ErrInvalidOption`을 반환합니다.

`SetProgressMonitor`로 이후 read 호출에서 사용할 progress callback을 설치하거나
교체할 수 있습니다.

```go
dec.SetProgressMonitor(func(p libjpeg.Progress) {
	_ = p.PassCounter
	_ = p.PassLimit
})
```

## Header State

`Config`에는 libjpeg field와 helper API에 대응하는 decompressor header state가
포함됩니다.

- `Baseline`, `Progressive`, `Arithmetic`은 SOF coding mode를 나타냅니다.
- `HasMultipleScans`는 `jpeg_has_multiple_scans()`에 대응합니다.
- `InputComplete`는 config가 만들어진 시점의 `jpeg_input_complete()` 값입니다.
- `SawJFIFMarker`, `JFIFMajorVersion`, `JFIFMinorVersion`, `DensityUnit`,
  `XDensity`, `YDensity`는 JFIF APP0 metadata입니다.
- `SawAdobeMarker`, `AdobeTransform`은 Adobe APP14 metadata입니다.
- `CCIR601Sampling`은 JFIF extension sampling flag입니다.
- `Scan`은 현재 SOS/per-scan field를 노출합니다. component count, MCU
  geometry, `Ss`, `Se`, `Ah`, `Al`, derived limiting spectral end를 포함합니다.

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
_ = dec.CCIR601Sampling()
_ = dec.ScanParameters()
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
Decoder 생성 시 `WithSavedMarkers`를 쓰거나, `ReadHeader` 전에
`Decoder.SaveMarkers`를 호출할 수 있습니다.

```go
dec := libjpeg.NewDecoder(r)
if err := dec.SaveMarkers(libjpeg.MarkerAPP14, 65533); err != nil {
	return err
}
if err := dec.SaveMarkers(libjpeg.MarkerCOM, 65533); err != nil {
	return err
}

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

## Marker Processors

`WithMarkerProcessor`와 `Decoder.SetMarkerProcessor`는 APPn/COM marker에 대한
libjpeg의 `jpeg_set_marker_processor()` lifecycle에 대응합니다. `ReadHeader`
전에 processor를 설정하면 callback은 marker payload의 복사본을 받고, error를
반환해 header parsing을 중단할 수 있습니다.

```go
dec := libjpeg.NewDecoder(
	r,
	libjpeg.WithMarkerProcessor(libjpeg.MarkerAPP2, func(marker libjpeg.Marker) error {
		_ = marker.Code
		_ = marker.OriginalLength
		_ = marker.Data
		return nil
	}),
)

if _, err := dec.ReadHeader(); err != nil {
	return err
}
```

같은 marker code에 marker processor와 saved-marker retention을 함께 설정하면
processor가 우선합니다.

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

for _, component := range dec.Components() {
	_ = component.ID
	_ = component.HSampFactor
	_ = component.VSampFactor
	_ = component.QuantizationTableIndex
}

_ = dec.RestartInterval()

arith, ok := dec.ArithmeticConditioningTable(0)
if ok {
	_ = arith.DCLower
	_ = arith.DCUpper
	_ = arith.ACK
}
_ = dec.ArithmeticConditioningTables()
```

Arithmetic conditioning table은 libjpeg의 `arith_dc_L`, `arith_dc_U`,
`arith_ac_K` 배열에 대응하며 table selector 0부터 15까지 지원합니다. Stream에
DAC marker가 없더라도 default 값을 확인할 수 있고, DAC override는 header parsing
이후 반영됩니다.

Table-only stream에서는 `ReadHeaderRequireImage(false)`를 호출합니다.
`HeaderTablesOnly` 상태에서는 image config는 비어 있지만, EOI 전에 parsing된
quantization/Huffman table은 table helper로 계속 확인할 수 있습니다. 이는
abbreviated stream을 위한 libjpeg의 permanent-table 동작에 맞춘 것입니다.

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
	QuantizeColors:    true,
	DesiredNumColors:  64,
	DitherMode:        libjpeg.DitherFloydSteinberg,
	QuantizationMode:  libjpeg.QuantizationDefault,
	RawDataOut:        false,
	OutputGamma:       1.0,
	BlockSmoothing:    libjpeg.BlockSmoothingDefault,
	ProgressMonitor:   nil,
}

raster, err := libjpeg.DecodeRasterWithOptions(r, opts)
```

CLI 스타일 문자열은 다음 함수로 파싱할 수 있습니다.

```go
idct, err := libjpeg.ParseIDCTMethod("int")
space, err := libjpeg.ParseInputColorSpace("rgb")
dither, err := libjpeg.ParseDitherMode("fs")
```

`ParseInputColorSpace`는 `big-gamut-rgb`, `big-gamut-ycbcr` 같은 libjpeg 9
big-gamut alias도 받습니다.

## Progress Monitor

`WithProgressMonitor`는 facade 수준에서 libjpeg의 `jpeg_progress_mgr` counter에
대응합니다. callback은 scanline, raw-data, coefficient read 중 호출됩니다.

```go
dec := libjpeg.NewDecoder(r, libjpeg.WithProgressMonitor(func(p libjpeg.Progress) {
	_ = p.PassCounter
	_ = p.PassLimit
	_ = p.CompletedPasses
	_ = p.TotalPasses
}))
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

// libjpeg return code가 필요하면 low-level form을 사용할 수 있습니다.
header, status, err := dec.ReadHeaderRequireImage(true)
if err != nil {
	return err
}
_ = status

dimensions, err := dec.CalcOutputDimensions()
if err != nil {
	return err
}
_ = dimensions.RecOutbufHeight

if err := dec.Start(); err != nil {
	return err
}

xOffset, width, err := dec.CropScanline(0, header.Width)
if err != nil {
	return err
}
_ = xOffset
_ = width

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

고급 호출자는 `ConsumeInput`으로 marker input을 직접 진행할 수 있습니다. 이는
libjpeg의 `jpeg_consume_input()` return code에 대응합니다.

```go
status, err := dec.ConsumeInput()
if err != nil {
	return err
}
if status == libjpeg.InputReachedSOS {
	_ = dec.Header()
}
```

`CalcOutputDimensions`는 libjpeg의 `jpeg_calc_output_dimensions()`에
대응합니다. decompression을 시작하기 전에 최종 output geometry가 필요하면
`ReadHeader` 이후 호출하세요.

`SkipScanlines`는 libjpeg의 `jpeg_skip_scanlines()`에 대응합니다. output
scanline cursor를 전진시키고, 이미지 하단에서 멈추며, 실제로 skip한 row 수를
반환합니다.

`CropScanline`은 facade 수준에서 libjpeg의 `jpeg_crop_scanline()`에 대응합니다.
`Start` 이후, row를 읽거나 skip하기 전에 호출하세요. 반환된 offset과 width는
실제 crop 영역이며, width는 output image의 오른쪽 경계에 맞게 clamp됩니다.

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

현재 progressive scanline 및 coefficient decoding은 지원하지 않습니다.
Progressive header는 `ReadHeader`와 `DecodeRasterConfig`로 조회할 수 있습니다.

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
| `--output-colorspace ycbcr` | `libjpeg.WithOutputColorSpace(libjpeg.ColorSpaceYCbCr)` |
| `--maxmemory 20m` | `limit, _ := libjpeg.ParseMemoryLimit("20m"); libjpeg.WithMaxMemory(limit)` |
| `--scale 1/2` | `libjpeg.WithScale(1, 2)` |
| `--colors 64` | `libjpeg.WithQuantizeColors(64)` |
| `--dither fs` | `libjpeg.WithDitherMode(libjpeg.DitherFloydSteinberg)` |
| `--onepass` | `libjpeg.WithQuantizationMode(libjpeg.QuantizationOnePass)` |
| `--compatibility poppler-pdf` | `libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF)` |
| `--turbo-fancy` | `libjpeg.WithTurboFancy()` deprecated alias |
| `--input-colorspace rgb` | `libjpeg.WithInputColorSpace(libjpeg.InputRGB)` |
| `--color-transform subtract-green` | `libjpeg.WithColorTransform(libjpeg.ColorTransformSubtractGreen)` |
| `--input-colorspace ycbcr` | `libjpeg.WithInputColorSpace(libjpeg.InputYCbCr)` |
| `--input-colorspace grayscale` | `libjpeg.WithInputColorSpace(libjpeg.InputGray)` |
| `--input-colorspace big-gamut-rgb` | `libjpeg.WithInputColorSpace(libjpeg.InputBigGamutRGB)` |
| `--input-colorspace big-gamut-ycbcr` | `libjpeg.WithInputColorSpace(libjpeg.InputBigGamutYCbCr)` |

`--ppm`, `--bmp`, `--os2`, `--targa`, `--rle`, `--gif`, `--gif0` 같은 CLI output
flag는 아직 public decode facade에 포함하지 않았습니다. 라이브러리에서는
`DecodeRaster`로 픽셀을 얻은 뒤 application code에서 원하는 output format으로
쓰세요.

`cmd/djpeg` output writer는 현재 Gray8과 RGB24 raster를 받습니다. 다른
`--output-colorspace` 값은 parity/debug 및 library API coverage를 위해 노출되어
있지만, 대응 command writer가 추가되기 전까지는 `DecodeRaster` 사용이 필요할 수
있습니다.

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

- baseline 및 extended sequential non-progressive JPEG가 지원 경로이며,
  Huffman과 arithmetic entropy coding을 지원합니다.
- progressive JPEG header는 읽을 수 있지만 progressive scanline 및
  coefficient decoding은 `ErrUnsupported`를 반환합니다.
- `Decode`는 `image.Image` interface 뒤에 `*Raster`를 반환합니다. RGBA allocation을
  강제하지 않으면서 raw byte 접근을 유지하기 위한 선택입니다.
- package는 `image.RegisterFormat`를 자동 호출하지 않습니다. 이 decoder의 parity
  동작이 필요하면 `libjpeg.Decode`를 명시적으로 사용하세요.
- 기본 option set은 현재 random100 corpus 기준 IJG 9f exact parity를 유지하도록
  선택했습니다.
