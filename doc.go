// Package djpeg exposes a Pure Go JPEG decompressor derived from the IJG
// libjpeg djpeg decode path.
//
// The package focuses on baseline, non-progressive JPEG files. Decode returns
// an image.Image for idiomatic Go callers, while DecodeRaster exposes the raw
// Gray8 or RGB24 byte buffer used by parity and renderer integrations.
package djpeg
