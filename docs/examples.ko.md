# 예제

이 문서는 `djpeg-go` CLI의 일반적인 사용 흐름을 설명합니다. 예시는 Linux
amd64 debug 빌드인 `./dist/djpeg-linux-amd64-debug`를 사용한다고 가정합니다.

```bash
make build
```

## Go 라이브러리 API

다른 Go 모듈에서 JPEG 데이터를 디코딩하려면 root 모듈을 import합니다.

```go
import libjpeg "github.com/dh-kam/djpeg-go"
```

Go image 생태계와 함께 쓰려면 `image.Image`로 디코딩합니다.

```go
in, err := os.Open("input.jpg")
if err != nil {
	return err
}
defer in.Close()

img, err := libjpeg.Decode(in)
if err != nil {
	return err
}

return png.Encode(out, img)
```

Gray8 또는 RGB24 byte layout이 필요하면 raw pixel로 디코딩합니다.

```go
raster, err := libjpeg.DecodeRaster(
	in,
	libjpeg.WithIDCT(libjpeg.IDCTInt),
)
if err != nil {
	return err
}

switch raster.Format {
case libjpeg.PixelFormatGray8:
	useGray(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
case libjpeg.PixelFormatRGB24:
	useRGB(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
}
```

Go 코드에서 Poppler/ImageMagick 호환 decompressor preset을 사용:

```go
raster, err := libjpeg.DecodeRaster(
	in,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
)
```

공개 API는 의도적으로 `internal/*` 패키지를 노출하지 않습니다. 해당 패키지들은
포팅 구현 세부사항으로 유지됩니다.

자세한 라이브러리 가이드는 [library-api.ko.md](library-api.ko.md)를 참고하세요.

## 기본 디코딩

JPEG를 binary PPM 또는 PGM으로 디코딩:

```bash
./dist/djpeg-linux-amd64-debug --ppm input.jpg > output.ppm
```

RGB 이미지는 PPM(`P6`)으로, grayscale 이미지는 PGM(`P5`)으로 출력됩니다.
`--pnm`은 upstream `djpeg`와 같은 출력 경로를 가리키는 alias로 사용할 수 있습니다.

Color JPEG를 grayscale output으로 강제:

```bash
./dist/djpeg-linux-amd64-debug --grayscale --ppm input.jpg > output.pgm
```

Grayscale JPEG를 RGB output으로 강제:

```bash
./dist/djpeg-linux-amd64-debug --rgb --ppm grayscale-input.jpg > output.ppm
```

출력 파일명을 직접 지정:

```bash
./dist/djpeg-linux-amd64-debug --ppm --outfile output.ppm input.jpg
```

stdin에서 JPEG 입력 받기:

```bash
cat input.jpg | ./dist/djpeg-linux-amd64-debug --ppm > output.ppm
```

## IDCT 선택

integer IDCT 경로 사용. IJG 9f와의 exact parity 테스트에 사용하는 경로입니다.

```bash
./dist/djpeg-linux-amd64-debug --dct int --ppm input.jpg > output.ppm
```

빠른 integer IDCT variant 사용:

```bash
./dist/djpeg-linux-amd64-debug --dct fast --ppm input.jpg > output.ppm
```

libjpeg 호환 fast shorthand 사용. 현재 decoder에서는 `--dct fast --nosmooth`로
동작하며, IJG `-fast`의 color quantization 관련 동작은 quantization 지원과 함께
추가할 예정입니다.

```bash
./dist/djpeg-linux-amd64-debug --fast --ppm input.jpg > output.ppm
```

floating-point IDCT variant 사용:

```bash
./dist/djpeg-linux-amd64-debug --dct float --ppm input.jpg > output.ppm
```

현재 exact-100 parity gate에는 `--dct int`만 포함되어 있습니다.

## Upsampling

기본 모드는 가능한 경우 IJG 호환 fancy upsampling을 사용합니다.

```bash
./dist/djpeg-linux-amd64-debug --dct int --ppm input.jpg > smooth.ppm
```

fancy upsampling 비활성화:

```bash
./dist/djpeg-linux-amd64-debug --dct int --nosmooth --ppm input.jpg > nosmooth.ppm
```

현재 random100 exact-100 parity 측정에는 default 모드와 `--nosmooth` 모드가
모두 포함됩니다.

## Memory Limit

Decoder가 소유하는 buffer의 대략적인 상한을 설정합니다.

```bash
./dist/djpeg-linux-amd64-debug --maxmemory 20m --ppm input.jpg > output.ppm
```

libjpeg와 같이 suffix 없는 숫자는 kilobyte로 해석하고, `m`/`M`은 megabyte로
해석합니다. 이 제한은 compressed scan buffer와 decoded component buffer에
적용되며, caller가 소유한 output file 또는 shell redirection buffer에는 적용되지
않습니다.

## Scaling

libjpeg 스타일 `M/N` fraction으로 output을 scaling합니다.

```bash
./dist/djpeg-linux-amd64-debug --scale 1/2 --ppm input.jpg > half.ppm
```

8x8 DCT JPEG에서는 ratio가 `1/8`부터 `16/8`까지의 지원 scale grid 중 가까운
값으로 매핑됩니다. 현재 구현은 먼저 decode한 뒤 output raster를 resampling하므로
dimension은 libjpeg scale grid를 따르지만, pixel 값까지 scaled-IDCT exact를
목표로 하지는 않습니다.

## Color Quantization

RGB output을 생성된 palette로 줄입니다.

```bash
./dist/djpeg-linux-amd64-debug --colors 216 --ppm input.jpg > quantized.ppm
```

Quantized output의 dithering mode를 선택합니다.

```bash
./dist/djpeg-linux-amd64-debug --colors 216 --dither ordered --ppm input.jpg > quantized.ppm
```

지원하는 dither mode는 `fs`, `ordered`, `none`입니다. `--onepass`는 libjpeg CLI
호환성을 위해 허용합니다. 현재 generated palette 경로는 deterministic one-pass
color-cube quantizer입니다.

GIF 또는 PPM 파일에서 external palette를 사용합니다.

```bash
./dist/djpeg-linux-amd64-debug --map palette.ppm --ppm input.jpg > mapped.ppm
```

RGB GIF output은 최대 256색 quantization을 자동으로 활성화합니다.

```bash
./dist/djpeg-linux-amd64-debug --gif --outfile output.gif input.jpg
```

upstream 호환 uncompressed GIF 출력:

```bash
./dist/djpeg-linux-amd64-debug --gif0 --outfile output.gif input.jpg
```

## Poppler/ImageMagick 호환 4:2:0 출력

일부 PDF image stream은 IJG 9f chroma IDCT scaling 대신 libjpeg-turbo 방식의
8x8 chroma IDCT와 fancy upsampling을 사용할 때 Poppler/ImageMagick 출력과
일치합니다.

```bash
./dist/djpeg-linux-amd64-debug --compatibility poppler-pdf --ppm input.jpg > output.ppm
```

`--turbo-fancy`는 이 profile의 deprecated alias로 유지합니다.

## 다른 출력 포맷

BMP 출력:

```bash
./dist/djpeg-linux-amd64-debug --bmp --outfile output.bmp input.jpg
```

Targa 출력:

```bash
./dist/djpeg-linux-amd64-debug --targa --outfile output.tga input.jpg
```

Utah RLE 출력:

```bash
./dist/djpeg-linux-amd64-debug --rle --outfile output.rle input.jpg
```

GIF 출력은 제한적입니다. grayscale 이미지는 생성된 grayscale palette로 쓸 수
있습니다.

```bash
./dist/djpeg-linux-amd64-debug --gif --outfile gray.gif grayscale-input.jpg
```

Color GIF에는 indexed-color 데이터가 필요합니다. 현재 CLI는 완전히 검증된 color
quantization 경로를 제공하지 않으므로 RGB 입력에는 PPM/BMP/Targa 사용을
권장합니다.

## Verbose 출력

입력과 출력 metadata를 stderr로 출력:

```bash
./dist/djpeg-linux-amd64-debug --verbose --ppm input.jpg > output.ppm
```

`--debug`는 upstream `djpeg`와 같이 `--verbose`의 alias로 사용할 수 있습니다.

## CPU 프로파일링

CLI는 내부 CPU profile flag를 지원합니다.

```bash
./dist/djpeg-linux-amd64-debug --cpuprofile cpu.pprof --ppm input.jpg > output.ppm
go tool pprof -top cpu.pprof
```

벤치마크 profile은 Go benchmark harness 사용을 권장합니다.

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

## Exact Parity 확인

Go CLI를 빌드하고 로컬 IJG 9f C reference와 비교:

```bash
go build -o /tmp/djpeg_go_exact100 ./cmd/djpeg
scripts/exact100_compare.py \
  --mode both \
  --go-djpeg /tmp/djpeg_go_exact100 \
  --json /tmp/exact100.json \
  --quiet-ok
```

이 스크립트는 raw 파일 전체가 아니라 PNM pixel payload를 비교합니다. 따라서
무해한 PPM/PGM header formatting 차이는 결과에 영향을 주지 않습니다.

## C와 Go 성능 비교

random100 CLI wall-time 비교 실행:

```bash
go build -o /tmp/djpeg_go_perf ./cmd/djpeg
scripts/perf_compare.py \
  --go-djpeg /tmp/djpeg_go_perf \
  --mode both \
  --warmup 1 \
  --repeat 5 \
  --json /tmp/djpeg_perf.json
```

최신 보고서는 [performance.md](performance.md)에 있습니다.

Sequential arithmetic-coded JPEG는 지원합니다. Progressive JPEG header는
조회할 수 있지만, progressive scanline 및 coefficient decoding은 현재 decoder에서
거부합니다.
