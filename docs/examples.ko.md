# 예제

이 문서는 `djpeg-go` CLI의 일반적인 사용 흐름을 설명합니다. 예시는
바이너리를 `./bin/djpeg-go`로 빌드했다고 가정합니다.

```bash
go build -o ./bin/djpeg-go ./cmd/djpeg
```

## 기본 디코딩

JPEG를 binary PPM 또는 PGM으로 디코딩:

```bash
./bin/djpeg-go --ppm input.jpg > output.ppm
```

RGB 이미지는 PPM(`P6`)으로, grayscale 이미지는 PGM(`P5`)으로 출력됩니다.

출력 파일명을 직접 지정:

```bash
./bin/djpeg-go --ppm --outfile output.ppm input.jpg
```

stdin에서 JPEG 입력 받기:

```bash
cat input.jpg | ./bin/djpeg-go --ppm > output.ppm
```

## IDCT 선택

integer IDCT 경로 사용. IJG 9f와의 exact parity 테스트에 사용하는 경로입니다.

```bash
./bin/djpeg-go --dct int --ppm input.jpg > output.ppm
```

빠른 integer IDCT variant 사용:

```bash
./bin/djpeg-go --dct fast --ppm input.jpg > output.ppm
```

floating-point IDCT variant 사용:

```bash
./bin/djpeg-go --dct float --ppm input.jpg > output.ppm
```

현재 exact-100 parity gate에는 `--dct int`만 포함되어 있습니다.

## Upsampling

기본 모드는 가능한 경우 IJG 호환 fancy upsampling을 사용합니다.

```bash
./bin/djpeg-go --dct int --ppm input.jpg > smooth.ppm
```

fancy upsampling 비활성화:

```bash
./bin/djpeg-go --dct int --nosmooth --ppm input.jpg > nosmooth.ppm
```

현재 random100 exact-100 parity 측정에는 default 모드와 `--nosmooth` 모드가
모두 포함됩니다.

## 다른 출력 포맷

BMP 출력:

```bash
./bin/djpeg-go --bmp --outfile output.bmp input.jpg
```

Targa 출력:

```bash
./bin/djpeg-go --targa --outfile output.tga input.jpg
```

Utah RLE 출력:

```bash
./bin/djpeg-go --rle --outfile output.rle input.jpg
```

GIF 출력은 제한적입니다. grayscale 이미지는 생성된 grayscale palette로 쓸 수
있습니다.

```bash
./bin/djpeg-go --gif --outfile gray.gif grayscale-input.jpg
```

Color GIF에는 indexed-color 데이터가 필요합니다. 현재 CLI는 완전히 검증된 color
quantization 경로를 제공하지 않으므로 RGB 입력에는 PPM/BMP/Targa 사용을
권장합니다.

## Verbose 출력

입력과 출력 metadata를 stderr로 출력:

```bash
./bin/djpeg-go --verbose --ppm input.jpg > output.ppm
```

## CPU 프로파일링

CLI는 내부 CPU profile flag를 지원합니다.

```bash
./bin/djpeg-go --cpuprofile cpu.pprof --ppm input.jpg > output.ppm
go tool pprof -top cpu.pprof
```

벤치마크 profile은 Go benchmark harness 사용을 권장합니다.

```bash
go test ./cmd/djpeg \
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
