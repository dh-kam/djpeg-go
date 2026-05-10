package output

import "image/color"

func colorSpaceCanWriteRGB(cs ColorSpace) bool {
	switch cs {
	case ColorSpaceRGB, ColorSpaceYCbCr, ColorSpaceBigGamutYCbCr, ColorSpaceCMYK, ColorSpaceYCCK:
		return true
	default:
		return false
	}
}

func rgbScanline(line []byte, width int, cs ColorSpace) []byte {
	switch cs {
	case ColorSpaceRGB:
		return line
	case ColorSpaceYCbCr, ColorSpaceBigGamutYCbCr:
		out := make([]byte, width*3)
		for i := 0; i < width; i++ {
			s := i * 3
			if cs == ColorSpaceBigGamutYCbCr {
				out[s], out[s+1], out[s+2] = bigGamutYCbCrToRGB(line[s], line[s+1], line[s+2])
			} else {
				out[s], out[s+1], out[s+2] = color.YCbCrToRGB(line[s], line[s+1], line[s+2])
			}
		}
		return out
	case ColorSpaceCMYK, ColorSpaceYCCK:
		out := make([]byte, width*3)
		for i := 0; i < width; i++ {
			s := i * 4
			d := i * 3
			out[d], out[d+1], out[d+2] = cmykLikeToRGB(line[s], line[s+1], line[s+2], line[s+3], cs)
		}
		return out
	default:
		return line
	}
}

func rgbColormap(cm *Colormap, cs ColorSpace) *Colormap {
	if cm == nil || cm.NumColors <= 0 {
		return cm
	}
	if cs == ColorSpaceRGB {
		return cm
	}
	red := make([]uint8, cm.NumColors)
	green := make([]uint8, cm.NumColors)
	blue := make([]uint8, cm.NumColors)
	for i := 0; i < cm.NumColors; i++ {
		red[i], green[i], blue[i] = colormapEntryToRGB(cm, byte(i), cs)
	}
	return &Colormap{Maps: [][]uint8{red, green, blue}, NumColors: cm.NumColors}
}

func colormapEntryToRGB(cm *Colormap, idx byte, cs ColorSpace) (byte, byte, byte) {
	switch cs {
	case ColorSpaceGrayscale:
		v := cm.Maps[0][idx]
		return v, v, v
	case ColorSpaceRGB:
		return cm.Maps[0][idx], cm.Maps[1][idx], cm.Maps[2][idx]
	case ColorSpaceYCbCr:
		return color.YCbCrToRGB(cm.Maps[0][idx], cm.Maps[1][idx], cm.Maps[2][idx])
	case ColorSpaceBigGamutYCbCr:
		return bigGamutYCbCrToRGB(cm.Maps[0][idx], cm.Maps[1][idx], cm.Maps[2][idx])
	case ColorSpaceCMYK, ColorSpaceYCCK:
		return cmykLikeToRGB(cm.Maps[0][idx], cm.Maps[1][idx], cm.Maps[2][idx], cm.Maps[3][idx], cs)
	default:
		return 0, 0, 0
	}
}

func cmykLikeToRGB(c, m, y, k byte, cs ColorSpace) (byte, byte, byte) {
	if cs == ColorSpaceYCCK {
		c, m, y = yccToCMY(c, m, y)
	}
	red, green, blue, _ := color.CMYK{C: c, M: m, Y: y, K: k}.RGBA()
	return byte(red >> 8), byte(green >> 8), byte(blue >> 8)
}

func yccToCMY(y, cb, cr byte) (byte, byte, byte) {
	yy := int(y)
	cbb := int(cb)
	crr := int(cr)
	r := clamp8(yy + ((91881 * (crr - 128)) >> 16))
	g := clamp8(yy - ((22554*(cbb-128) + 46802*(crr-128)) >> 16))
	b := clamp8(yy + ((116130 * (cbb - 128)) >> 16))
	return byte(255 - r), byte(255 - g), byte(255 - b)
}

func bigGamutYCbCrToRGB(y, cb, cr byte) (byte, byte, byte) {
	yy := int(y)
	cbb := int(cb) - 128
	crr := int(cr) - 128
	r := yy + ((91882*crr + 32768) >> 15)
	g := yy - ((46720*crr + 22553*cbb + 16384) >> 15)
	b := yy + ((116130*cbb + 32768) >> 15)
	return byte(clamp8(r)), byte(clamp8(g)), byte(clamp8(b))
}
