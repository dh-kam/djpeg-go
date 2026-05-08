package main

import (
	"fmt"
	"os"

	"github.com/dh-kam/djpeg-go/internal/djpegcli"
)

func main() {
	if err := djpegcli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
