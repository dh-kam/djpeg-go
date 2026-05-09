// Package djpeg exposes a Pure Go, libjpeg-compatible JPEG decompressor API.
//
// The package focuses on baseline, non-progressive JPEG files. Decode returns
// an image.Image for idiomatic Go callers, DecodeRaster exposes the raw Gray8
// or RGB24 byte buffer used by renderers, and Decoder provides a scanline API
// shaped after libjpeg's decompressor flow. The cmd/djpeg command is a CLI
// compatibility and debug tool built on top of this library API.
package djpeg
