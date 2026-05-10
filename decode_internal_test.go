package djpeg

import (
	"errors"
	"testing"
)

func TestIsUnsupportedErrorDoesNotKeyOnSupportedCodingNames(t *testing.T) {
	for _, msg := range []string{
		"jpeg: progressive AC scan must contain one block per MCU",
		"jpeg: arithmetic scan failed checksum",
	} {
		if isUnsupportedError(errors.New(msg)) {
			t.Fatalf("isUnsupportedError(%q) = true, want false", msg)
		}
	}
}

func TestIsUnsupportedErrorKeepsExplicitUnsupportedSignals(t *testing.T) {
	for _, msg := range []string{
		"jpeg: unsupported arithmetic table selector",
		"jpeg: feature not yet supported",
	} {
		if !isUnsupportedError(errors.New(msg)) {
			t.Fatalf("isUnsupportedError(%q) = false, want true", msg)
		}
	}
}
