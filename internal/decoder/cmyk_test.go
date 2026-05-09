package decoder

import (
	"bytes"
	"testing"

	"github.com/dh-kam/djpeg-go/internal/color"
	"github.com/dh-kam/djpeg-go/internal/marker"
)

func TestUpsampleAndConvertCMYKPassthrough(t *testing.T) {
	dec := newFourComponentDecoder(marker.CSCMYK, marker.CSCMYK)
	dec.componentBuf = [][][]byte{
		{{10, 20}},
		{{30, 40}},
		{{50, 60}},
		{{70, 80}},
	}

	out := make([]byte, 8)
	dec.upsampleAndConvert(out)

	want := []byte{10, 30, 50, 70, 20, 40, 60, 80}
	if !bytes.Equal(out, want) {
		t.Fatalf("CMYK output = %v, want %v", out, want)
	}
}

func TestUpsampleAndConvertYCCKToCMYK(t *testing.T) {
	dec := newFourComponentDecoder(marker.CSYCCK, marker.CSCMYK)
	dec.componentBuf = [][][]byte{
		{{128, 128}},
		{{128, 128}},
		{{128, 128}},
		{{17, 18}},
	}

	out := make([]byte, 8)
	dec.upsampleAndConvert(out)

	want := []byte{127, 127, 127, 17, 127, 127, 127, 18}
	if !bytes.Equal(out, want) {
		t.Fatalf("YCCK->CMYK output = %v, want %v", out, want)
	}
}

func TestUpsampleAndConvertYCCKToRGB(t *testing.T) {
	dec := newFourComponentDecoder(marker.CSYCCK, marker.CSRGB)
	dec.componentBuf = [][][]byte{
		{{128, 128}},
		{{128, 128}},
		{{128, 128}},
		{{0, 128}},
	}

	out := make([]byte, 6)
	dec.upsampleAndConvert(out)

	want := []byte{128, 128, 128, 63, 63, 63}
	if !bytes.Equal(out, want) {
		t.Fatalf("YCCK->RGB output = %v, want %v", out, want)
	}
}

func TestUpsampleAndConvertCMYKToGray(t *testing.T) {
	dec := newFourComponentDecoder(marker.CSCMYK, marker.CSGrayScale)
	dec.componentBuf = [][][]byte{
		{{0, 255}},
		{{255, 0}},
		{{255, 255}},
		{{0, 0}},
	}

	out := make([]byte, 2)
	dec.upsampleAndConvert(out)

	want := []byte{76, 150}
	if !bytes.Equal(out, want) {
		t.Fatalf("CMYK->Gray output = %v, want %v", out, want)
	}
}

func TestValidateUnsupportedFourComponentConversion(t *testing.T) {
	dec := newFourComponentDecoder(marker.CSCMYK, marker.CSRGB)
	dec.d.OutColorSpace = marker.CSYCCK
	if err := dec.validateColorConversion(); err == nil {
		t.Fatal("expected unsupported conversion error")
	}
}

func newFourComponentDecoder(jpegCS, outCS marker.ColorSpace) *Decoder {
	d := &marker.Decompressor{
		ImageWidth:               2,
		ImageHeight:              1,
		OutputWidth:              2,
		OutputHeight:             1,
		NumComponents:            4,
		OutputComponents:         4,
		OutColorComponents:       4,
		JPEGColorSpace:           jpegCS,
		OutColorSpace:            outCS,
		MaxHSampFactor:           1,
		MaxVSampFactor:           1,
		MinDCTHScaledSize:        1,
		MinDCTVScaledSize:        1,
		DoFancyUpsampling:        true,
		DisableChromaIDCTScaling: true,
		CompInfo:                 make([]marker.ComponentInfo, 4),
	}
	for i := 0; i < 4; i++ {
		d.CompInfo[i] = marker.ComponentInfo{
			ComponentIndex:  i,
			HSampFactor:     1,
			VSampFactor:     1,
			DCHScaledSize:   1,
			DCVScaledSize:   1,
			ComponentNeeded: true,
		}
	}
	info := &color.DecompressInfo{
		JpegColorSpace:     color.JCS_CMYK,
		OutColorSpace:      color.JCS_CMYK,
		NumComponents:      4,
		OutColorComponents: 4,
		OutputComponents:   4,
		CompInfo: []color.ComponentInfo{
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
			{ComponentNeeded: true},
		},
	}
	if jpegCS == marker.CSYCCK {
		info.JpegColorSpace = color.JCS_YCCK
	}
	if outCS == marker.CSYCCK {
		info.OutColorSpace = color.JCS_YCCK
	}
	dec := &Decoder{
		d:           d,
		colorConv:   color.NewColorConverter(info),
		rlColorConv: make([]byte, 1024+2*rlColorOffset),
	}
	for i := range dec.rlColorConv {
		v := i - rlColorOffset
		if v < 0 {
			dec.rlColorConv[i] = 0
		} else if v > 255 {
			dec.rlColorConv[i] = 255
		} else {
			dec.rlColorConv[i] = byte(v)
		}
	}
	return dec
}
