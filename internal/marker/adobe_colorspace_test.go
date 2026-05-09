package marker

import "testing"

func TestDefaultDecompressParmsAdobeThreeComponentTransforms(t *testing.T) {
	tests := []struct {
		name      string
		transform uint8
		want      ColorSpace
	}{
		{name: "transform_0_rgb", transform: 0, want: CSRGB},
		{name: "transform_1_ycbcr", transform: 1, want: CSYCbCr},
		{name: "unknown_transform_assumes_ycbcr", transform: 2, want: CSYCbCr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := adobeDecompressor(3, tt.transform, []int{0x11, 0x12, 0x13})
			defaultDecompressParms(d)
			if d.JPEGColorSpace != tt.want {
				t.Fatalf("JPEGColorSpace = %v, want %v", d.JPEGColorSpace, tt.want)
			}
			if d.OutColorSpace != CSRGB {
				t.Fatalf("OutColorSpace = %v, want RGB", d.OutColorSpace)
			}
		})
	}
}

func TestDefaultDecompressParmsAdobeFourComponentTransforms(t *testing.T) {
	tests := []struct {
		name      string
		transform uint8
		want      ColorSpace
	}{
		{name: "transform_0_cmyk", transform: 0, want: CSCMYK},
		{name: "transform_1_assumes_ycck", transform: 1, want: CSYCCK},
		{name: "transform_2_ycck", transform: 2, want: CSYCCK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := adobeDecompressor(4, tt.transform, []int{0x11, 0x12, 0x13, 0x14})
			defaultDecompressParms(d)
			if d.JPEGColorSpace != tt.want {
				t.Fatalf("JPEGColorSpace = %v, want %v", d.JPEGColorSpace, tt.want)
			}
			if d.OutColorSpace != CSCMYK {
				t.Fatalf("OutColorSpace = %v, want CMYK", d.OutColorSpace)
			}
		})
	}
}

func TestDefaultDecompressParmsComponentIDsPrecedeAdobe(t *testing.T) {
	rgb := adobeDecompressor(3, 1, []int{'R', 'G', 'B'})
	defaultDecompressParms(rgb)
	if rgb.JPEGColorSpace != CSRGB {
		t.Fatalf("RGB component IDs inferred %v, want RGB", rgb.JPEGColorSpace)
	}

	ycck := adobeDecompressor(4, 0, []int{1, 2, 3, 4})
	defaultDecompressParms(ycck)
	if ycck.JPEGColorSpace != CSYCCK {
		t.Fatalf("YCCK component IDs inferred %v, want YCCK", ycck.JPEGColorSpace)
	}
}

func adobeDecompressor(numComponents int, transform uint8, componentIDs []int) *Decompressor {
	d := &Decompressor{
		NumComponents:     numComponents,
		CompInfo:          make([]ComponentInfo, numComponents),
		SawAdobeMarker:    true,
		AdobeTransform:    transform,
		BlockSize:         DCTSize,
		DoFancyUpsampling: true,
	}
	for i, id := range componentIDs {
		d.CompInfo[i] = ComponentInfo{ComponentID: id}
	}
	return d
}
