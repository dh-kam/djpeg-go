# 예제

이 문서는 `djpeg-go` CLI의 일반적인 사용 흐름을 설명합니다. 예시는 Linux
amd64 debug 빌드인 `./dist/djpeg-linux-amd64-debug`를 사용한다고 가정합니다.

```bash
make build
```

## Go 라이브러리 API

다른 Go 모듈에서 JPEG 데이터를 디코딩하려면 root 모듈을 import합니다.

```go
import djpeg "github.com/dh-kam/djpeg-go"
```

Go image 생태계와 함께 쓰려면 `image.Image`로 디코딩합니다.

```go
in, err := os.Open("input.jpg")
if err != nil {
	return err
}
defer in.Close()

img, err := djpeg.Decode(in)
if err != nil {
	return err
}

return png.Encode(out, img)
```

Gray8 또는 RGB24 byte layout이 필요하면 raw pixel로 디코딩합니다.

```go
raster, err := djpeg.DecodeRaster(
	in,
	djpeg.WithIDCT(djpeg.IDCTInt),
)
if err != nil {
	return err
}

switch raster.Format {
case djpeg.PixelFormatGray8:
	useGray(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
case djpeg.PixelFormatRGB24:
	useRGB(raster.Pix, raster.Rect.Dx(), raster.Rect.Dy(), raster.Stride)
}
```

Go 코드에서 Poppler/ImageMagick 호환 chroma 경로를 사용:

```go
raster, err := djpeg.DecodeRaster(
	in,
	djpeg.WithIDCT(djpeg.IDCTInt),
	djpeg.WithTurboFancy(),
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

## Poppler/ImageMagick 호환 4:2:0 출력

일부 PDF image stream은 IJG 9f chroma IDCT scaling 대신 libjpeg-turbo 방식의
8x8 chroma IDCT와 fancy upsampling을 사용할 때 Poppler/ImageMagick 출력과
일치합니다.

```bash
./dist/djpeg-linux-amd64-debug --turbo-fancy --ppm input.jpg > output.ppm
```

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

## 미지원 또는 미완성 옵션

일부 `djpeg` 스타일 flag는 CLI 호환성을 위해 parsing하지만 아직 완전히 구현되지
않았습니다.

- `--grayscale`
- `--rgb`
- `--fast`
- `--onepass`
- `--dither`
- `--scale`
- `--maxmemory`

Arithmetic-coded JPEG와 progressive JPEG는 현재 decoder에서 거부합니다.
