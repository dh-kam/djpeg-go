# djpeg-go

`djpeg-go`는 Independent JPEG Group의 JPEG 소프트웨어 중 decompressor 경로를
Pure Go로 포팅한 프로젝트입니다. 라이브러리 목표는 libjpeg 호환 decompressor
API이고, `cmd/djpeg` 바이너리는 그 API 위에 얹힌 CLI 호환/디버그 도구입니다.
현재 기준 upstream은 2024년 1월 14일에 배포된 IJG libjpeg 9f입니다.

디코더는 Go로 구현되어 있으며 런타임에 libjpeg를 링크하지 않습니다. 로컬의
`jpeg-9f/` 트리는 parity와 성능 테스트를 위한 upstream 기준 구현으로
보관합니다.

## 현재 상태

- baseline 및 extended sequential non-progressive JPEG 디코딩이 주 지원
  경로이며, Huffman과 arithmetic entropy coding을 지원합니다.
- progressive JPEG scanline, raw component, Huffman coefficient output을
  지원합니다. Progressive arithmetic coefficient decoding은 아직 거부합니다.
- CLI 출력 중 PPM/PGM 경로가 가장 많이 검증되어 있습니다.
- BMP, Targa, RLE, 제한적인 GIF writer 코드가 있지만 exact parity 측정은
  현재 PPM/PGM 출력 기준입니다.
- `tests/testdata/random100` 코퍼스 기준 IJG 9f `djpeg -dct int`와 default smooth,
  `--nosmooth` 양쪽 모두 Exact-100 parity를 달성했습니다.

최신 측정 결과는 [docs/exact-100-result.md](docs/exact-100-result.md)와
[docs/performance.md](docs/performance.md)를 참고하세요.

## Upstream

이 프로젝트는 Independent JPEG Group의 작업을 일부 기반으로 합니다. Go
구현은 포팅에 필요한 범위에서 IJG의 자료 구조, marker parsing, entropy
decode, IDCT 동작, color conversion, 일부 `djpeg` 출력 동작을 기준으로
삼았습니다.

중요한 upstream 기준 파일:

- `jpeg-9f/README`: upstream IJG 라이선스와 문서
- `jpeg-9f/jd*.c`, `jpeg-9f/jidct*.c`, `jpeg-9f/jdsample.c`,
  `jpeg-9f/jdcolor.c`: 디코더 동작 기준
- `jpeg-9f/.libs/djpeg`: parity와 성능 스크립트에서 사용하는 로컬 C 기준
  바이너리

이 프로젝트는 Independent JPEG Group과 제휴되어 있지 않습니다.

## 빌드

```bash
make build
```

`GOBIN`에 CLI를 설치하려면:

```bash
go install ./cmd/djpeg
```

단일 release 타깃 빌드:

```bash
make linux-amd64-release VERSION=v0.1.0-202605.1-9f
```

전체 release 타깃 빌드:

```bash
make release VERSION=v0.1.0-202605.1-9f
```

릴리스 태그는 `vSEMVER-YYYYMM.seq-upstreamversion` 형식을 사용합니다.
예시는 `v0.1.0-202605.1-9f`입니다. 현재 월 기준 다음 태그를 출력하려면:

```bash
make bump-up SEMVER=0.1.0 UPSTREAM_VERSION=9f
```

## 빠른 사용법

Go 라이브러리로 사용:

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

정확한 byte layout이 필요하면 raw pixel raster를 사용할 수 있습니다.

```go
raster, err := libjpeg.DecodeRaster(
	input,
	libjpeg.WithCompatibility(libjpeg.CompatibilityPopplerPDF),
)
if err != nil {
	return err
}
// raster.Pix는 top-down Gray8 또는 RGB24 데이터이며 row당 raster.Stride byte입니다.
```

현재 import path는 repository 이름을 따릅니다. 예제에서 root package를 `libjpeg`로
alias하는 이유는 공개 API의 주 대상이 Poppler/go-pdf 연동면이기 때문입니다.
`djpeg` parity는 회귀 테스트와 CLI frontend로 유지합니다.

JPEG를 raw binary PPM/PGM으로 디코딩:

```bash
./dist/djpeg-linux-amd64-debug --ppm input.jpg > output.ppm
```

fancy upsampling 비활성화:

```bash
./dist/djpeg-linux-amd64-debug --dct int --nosmooth --ppm input.jpg > output.ppm
```

PDF 4:2:0 DCT stream에서 Poppler/ImageMagick 방식 출력과 맞추기:

```bash
./dist/djpeg-linux-amd64-debug --compatibility poppler-pdf --ppm input.jpg > output.ppm
```

기존 `--turbo-fancy` 플래그는 이 compatibility profile의 deprecated alias로
유지합니다.

파일로 출력:

```bash
./dist/djpeg-linux-amd64-debug --ppm --outfile output.ppm input.jpg
```

더 많은 예시는 [docs/examples.ko.md](docs/examples.ko.md)에 있습니다. 공개 Go
라이브러리 facade는 [docs/library-api.ko.md](docs/library-api.ko.md)에 자세히
정리되어 있습니다.

## 검증

Go 테스트 실행:

```bash
make test
make vet
```

IJG 9f C 기준 구현과 exact parity 확인:

```bash
go build -o /tmp/djpeg_go_exact100 ./cmd/djpeg
scripts/exact100_compare.py \
  --mode both \
  --go-djpeg /tmp/djpeg_go_exact100 \
  --quiet-ok
```

C와 Go 성능 비교:

```bash
go build -o /tmp/djpeg_go_perf ./cmd/djpeg
scripts/perf_compare.py \
  --go-djpeg /tmp/djpeg_go_perf \
  --mode both \
  --warmup 1 \
  --repeat 5
```

## 저장소 구조

- package root (`github.com/dh-kam/djpeg-go`): 공개 Go 라이브러리 API
- `cmd/djpeg`: `djpeg` 워크플로와 맞춘 CLI
- `internal/djpegcli`: CLI용 Cobra/Viper command orchestration
- `internal/decoder`: 상위 JPEG 디코딩 파이프라인
- `internal/marker`: marker parsing과 decompressor metadata
- `internal/huff`: Huffman entropy decode와 IDCT 구현
- `internal/color`: IJG 기반 color conversion 및 upsampling 보조 코드
- `internal/output`: PPM/PGM, BMP, GIF, Targa, RLE writer
- `tests`: 통합, CLI, parity, benchmark 테스트와 공용 fixture
- `scripts`: parity 및 성능 측정 하네스
- `docs`: 정확도, 성능, 사용 문서
- `jpeg-9f`: upstream IJG 기준 소스와 로컬 C reference build

## CI와 Release

GitHub Actions는 push와 pull request마다 `make vet`, `make test`, command
build를 실행합니다. 수동 Release workflow는 다음 bump-up 태그를 계산하고,
Linux/macOS/Windows amd64/arm64 정적 release 바이너리를 빌드한 뒤 git tag를
생성하고 GitHub Release에 바이너리를 업로드합니다.

## 라이선스

이 저장소의 원본 Go 코드는 MIT License로 배포합니다. [LICENSE](LICENSE)를
참고하세요.

일부 코드는 Independent JPEG Group의 소프트웨어에서 파생되었거나 이를
기반으로 하며, 해당 부분에는 IJG 라이선스 조건이 함께 적용됩니다. 특히 이
소프트웨어는 Independent JPEG Group의 작업을 일부 기반으로 합니다.
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)와 `jpeg-9f/README`를
참고하세요.
