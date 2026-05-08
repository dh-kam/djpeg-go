package djpeg_test

import (
	"fmt"
	"os"

	djpeg "github.com/dh-kam/djpeg-go"
)

func ExampleDecodeRaster() {
	f, err := os.Open("tests/testdata/gray_8x8.jpg")
	if err != nil {
		return
	}
	defer f.Close()

	raster, err := djpeg.DecodeRaster(f)
	if err != nil {
		return
	}
	fmt.Println(raster.Rect.Dx(), raster.Rect.Dy(), raster.Format)
	// Output: 8 8 gray8
}
