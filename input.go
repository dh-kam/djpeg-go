package djpeg

import "github.com/dh-kam/djpeg-go/internal/marker"

// HeaderStatus is the public return code for ReadHeaderRequireImage.
type HeaderStatus int

const (
	HeaderSuspended  HeaderStatus = marker.JPEGSuspended
	HeaderOK         HeaderStatus = marker.JPEGHeaderOK
	HeaderTablesOnly HeaderStatus = marker.JPEGHeaderTablesOnly
)

func (s HeaderStatus) String() string {
	switch s {
	case HeaderSuspended:
		return "suspended"
	case HeaderOK:
		return "ok"
	case HeaderTablesOnly:
		return "tables-only"
	default:
		return "unknown"
	}
}

// InputStatus is the public return code for ConsumeInput.
type InputStatus int

const (
	InputSuspended     InputStatus = marker.JPEGSuspended
	InputReachedSOS    InputStatus = marker.JPEGReachedSOS
	InputReachedEOI    InputStatus = marker.JPEGReachedEOI
	InputRowCompleted  InputStatus = 3
	InputScanCompleted InputStatus = 4
)

func (s InputStatus) String() string {
	switch s {
	case InputSuspended:
		return "suspended"
	case InputReachedSOS:
		return "reached-sos"
	case InputReachedEOI:
		return "reached-eoi"
	case InputRowCompleted:
		return "row-completed"
	case InputScanCompleted:
		return "scan-completed"
	default:
		return "unknown"
	}
}
