package djpeggo_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/djpegcli"
	"github.com/dh-kam/djpeg-go/internal/output"
)

type subsampling string

const (
	subsampling420 subsampling = "4:2:0"
	subsampling444 subsampling = "4:4:4"
)

func TestGoNoSmooth420DiffersFromDefault(t *testing.T) {
	root := testRepoRoot(t)
	sample := findRandom100JPEG(t, root, subsampling420)

	defaultPPM := decodeWithGoDjpeg(t, sample, false)
	noSmoothPPM := decodeWithGoDjpeg(t, sample, true)

	if bytes.Equal(defaultPPM, noSmoothPPM) {
		t.Fatalf("Go --nosmooth output matched default/fancy output for 4:2:0 sample %s", sample)
	}
}

func TestGoNoSmoothMatchesCleanCDjpeg(t *testing.T) {
	root := testRepoRoot(t)
	cases := []struct {
		name string
		subsampling
	}{
		{name: "420", subsampling: subsampling420},
		{name: "444", subsampling: subsampling444},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sample := findRandom100JPEG(t, root, tc.subsampling)
			cPPM, cRef := decodeWithCleanCDjpeg(t, root, sample)
			goPPM := decodeWithGoDjpeg(t, sample, true)

			if !bytes.Equal(goPPM, cPPM) {
				t.Fatalf("Go --nosmooth output differed from C djpeg -nosmooth for %s sample %s using %s", tc.subsampling, sample, cRef)
			}
		})
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func findRandom100JPEG(t *testing.T, root string, want subsampling) string {
	t.Helper()

	pattern := filepath.Join(root, "tests", "testdata", "random100", "*.jpg")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("invalid random100 glob %q: %v", pattern, err)
	}
	if len(matches) == 0 {
		t.Skipf("tests/testdata/random100 missing: no files matched %s", pattern)
	}
	sort.Strings(matches)

	for _, path := range matches {
		got, err := jpegSubsampling(path)
		if err != nil {
			t.Logf("skipping %s: %v", path, err)
			continue
		}
		if got == want {
			return path
		}
	}
	t.Skipf("random100 testdata has no %s JPEG sample", want)
	return ""
}

func decodeWithGoDjpeg(t *testing.T, jpegPath string, noSmooth bool) []byte {
	t.Helper()

	data, err := os.ReadFile(jpegPath)
	if err != nil {
		t.Skipf("JPEG sample missing: %v", err)
	}

	var out bytes.Buffer
	opts := &djpegcli.Options{
		Format:    output.FormatPPM,
		NoSmooth:  noSmooth,
		DctMethod: "int",
	}
	if err := djpegcli.Decompress(bytes.NewReader(data), &out, opts); err != nil {
		t.Fatalf("Go djpeg decode failed for %s: %v", jpegPath, err)
	}
	if _, _, _, _, err := parsePNM(out.Bytes()); err != nil {
		t.Fatalf("Go djpeg produced invalid PPM for %s: %v", jpegPath, err)
	}
	return out.Bytes()
}

func decodeWithCleanCDjpeg(t *testing.T, root, jpegPath string) ([]byte, string) {
	t.Helper()

	var reasons []string
	for _, candidate := range cDjpegCandidates(root) {
		ppm, err := runCDjpeg(candidate, jpegPath)
		if err == nil {
			return ppm, candidate.path
		}
		reasons = append(reasons, fmt.Sprintf("%s: %v", candidate.path, err))
	}

	t.Skipf("no clean C djpeg reference available for %s (%v)", jpegPath, reasons)
	return nil, ""
}

type cDjpegCandidate struct {
	path string
	env  []string
}

func cDjpegCandidates(root string) []cDjpegCandidate {
	var candidates []cDjpegCandidate

	if envPath := os.Getenv("DJPEG_C"); envPath != "" {
		candidates = append(candidates, cDjpegCandidate{path: envPath})
	}

	jpeg9Lib := filepath.Join(root, "jpeg-9f", ".libs")
	candidates = append(candidates, cDjpegCandidate{
		path: filepath.Join(jpeg9Lib, "djpeg"),
		env:  append(os.Environ(), "LD_LIBRARY_PATH="+prependEnvPath(jpeg9Lib, os.Getenv("LD_LIBRARY_PATH"))),
	})

	return candidates
}

func prependEnvPath(first, rest string) string {
	if rest == "" {
		return first
	}
	return first + string(os.PathListSeparator) + rest
}

func runCDjpeg(candidate cDjpegCandidate, jpegPath string) ([]byte, error) {
	path := candidate.path
	if filepath.Base(path) == path {
		found, err := exec.LookPath(path)
		if err != nil {
			return nil, err
		}
		path = found
	} else if st, err := os.Stat(path); err != nil {
		return nil, err
	} else if st.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}

	cmd := exec.Command(path, "-dct", "int", "-nosmooth", "-ppm", jpegPath)
	if len(candidate.env) > 0 {
		cmd.Env = candidate.env
	}
	ppm, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if _, _, _, _, err := parsePNM(ppm); err != nil {
		return nil, err
	}
	return ppm, nil
}

func jpegSubsampling(path string) (subsampling, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return "", errors.New("missing JPEG SOI")
	}

	for pos := 2; pos < len(data); {
		for pos < len(data) && data[pos] != 0xff {
			pos++
		}
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos >= len(data) {
			break
		}
		marker := data[pos]
		pos++

		if marker == 0xd9 || marker == 0xda {
			break
		}
		if marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if pos+2 > len(data) {
			return "", errors.New("truncated marker length")
		}
		segLen := int(data[pos])<<8 | int(data[pos+1])
		if segLen < 2 || pos+segLen > len(data) {
			return "", errors.New("invalid marker length")
		}
		seg := data[pos+2 : pos+segLen]
		pos += segLen

		if isSOFMarker(marker) {
			return subsamplingFromSOF(seg)
		}
	}

	return "", errors.New("SOF marker not found")
}

func isSOFMarker(marker byte) bool {
	switch marker {
	case 0xc0, 0xc1, 0xc2, 0xc3, 0xc5, 0xc6, 0xc7, 0xc9, 0xca, 0xcb, 0xcd, 0xce, 0xcf:
		return true
	default:
		return false
	}
}

func subsamplingFromSOF(seg []byte) (subsampling, error) {
	if len(seg) < 6 {
		return "", errors.New("short SOF segment")
	}
	nc := int(seg[5])
	if nc != 3 {
		return "", fmt.Errorf("component count = %d, want 3", nc)
	}
	if len(seg) < 6+3*nc {
		return "", errors.New("truncated SOF components")
	}

	ySampling := seg[7]
	cbSampling := seg[10]
	crSampling := seg[13]
	if cbSampling != 0x11 || crSampling != 0x11 {
		return "", fmt.Errorf("unsupported chroma sampling Cb=%#x Cr=%#x", cbSampling, crSampling)
	}

	switch ySampling {
	case 0x22:
		return subsampling420, nil
	case 0x11:
		return subsampling444, nil
	default:
		return "", fmt.Errorf("unsupported Y sampling %#x", ySampling)
	}
}

func parsePNM(data []byte) (kind string, width, height, components int, err error) {
	pos := 0
	nextToken := func() (string, error) {
		for {
			for pos < len(data) && isPNMSpace(data[pos]) {
				pos++
			}
			if pos < len(data) && data[pos] == '#' {
				for pos < len(data) && data[pos] != '\n' {
					pos++
				}
				continue
			}
			break
		}
		if pos >= len(data) {
			return "", errors.New("unexpected end of PNM header")
		}
		start := pos
		for pos < len(data) && !isPNMSpace(data[pos]) && data[pos] != '#' {
			pos++
		}
		return string(data[start:pos]), nil
	}

	kind, err = nextToken()
	if err != nil {
		return "", 0, 0, 0, err
	}
	if kind != "P5" && kind != "P6" {
		return "", 0, 0, 0, fmt.Errorf("PNM kind = %q, want P5 or P6", kind)
	}
	wToken, err := nextToken()
	if err != nil {
		return "", 0, 0, 0, err
	}
	hToken, err := nextToken()
	if err != nil {
		return "", 0, 0, 0, err
	}
	maxToken, err := nextToken()
	if err != nil {
		return "", 0, 0, 0, err
	}
	width, err = strconv.Atoi(wToken)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("invalid PNM width %q", wToken)
	}
	height, err = strconv.Atoi(hToken)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("invalid PNM height %q", hToken)
	}
	maxVal, err := strconv.Atoi(maxToken)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("invalid PNM max value %q", maxToken)
	}
	if width <= 0 || height <= 0 || maxVal != 255 {
		return "", 0, 0, 0, fmt.Errorf("invalid PNM metadata width=%d height=%d max=%d", width, height, maxVal)
	}
	if pos >= len(data) || !isPNMSpace(data[pos]) {
		return "", 0, 0, 0, errors.New("PNM header missing payload separator")
	}
	pos++

	components = 3
	if kind == "P5" {
		components = 1
	}
	wantLen := pos + width*height*components
	if len(data) != wantLen {
		return "", 0, 0, 0, fmt.Errorf("PNM length = %d, want %d", len(data), wantLen)
	}
	return kind, width, height, components, nil
}

func isPNMSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}
