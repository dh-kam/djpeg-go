package djpeg

import "fmt"

// JPEG marker codes supported by WithSavedMarkers.
const (
	MarkerAPP0  = 0xE0
	MarkerAPP1  = 0xE1
	MarkerAPP2  = 0xE2
	MarkerAPP3  = 0xE3
	MarkerAPP4  = 0xE4
	MarkerAPP5  = 0xE5
	MarkerAPP6  = 0xE6
	MarkerAPP7  = 0xE7
	MarkerAPP8  = 0xE8
	MarkerAPP9  = 0xE9
	MarkerAPP10 = 0xEA
	MarkerAPP11 = 0xEB
	MarkerAPP12 = 0xEC
	MarkerAPP13 = 0xED
	MarkerAPP14 = 0xEE
	MarkerAPP15 = 0xEF
	MarkerCOM   = 0xFE
)

// MarkerAPP returns the JPEG APPn marker code for n in [0, 15].
func MarkerAPP(n int) (int, error) {
	if n < 0 || n > 15 {
		return 0, fmt.Errorf("%w: APP marker index %d out of range", ErrInvalidOption, n)
	}
	return MarkerAPP0 + n, nil
}

// Marker is APPn or COM marker payload data from a JPEG header.
type Marker struct {
	Code           int
	OriginalLength uint
	Data           []byte
}

// MarkerProcessor handles an APPn or COM marker while ReadHeader scans the
// JPEG header. Returning an error stops header parsing.
type MarkerProcessor func(Marker) error

func validSavedMarkerCode(code int) bool {
	return code == MarkerCOM || (code >= MarkerAPP0 && code <= MarkerAPP15)
}
