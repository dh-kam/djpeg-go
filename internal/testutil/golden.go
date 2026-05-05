package testutil

import (
	"encoding/json"
	"os"
)

// GoldenTestData represents the expected output for a golden test JPEG file.
type GoldenTestData struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Components   int    `json:"components"`
	PixelsBase64 string `json:"pixels_base64"`
	Description  string `json:"description"`
}

// LoadGoldenReference loads a golden reference JSON file and returns the
// expected pixel data along with image dimensions and component count.
func LoadGoldenReference(jsonPath string) (pixels []byte, w, h, nc int, err error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	var gd GoldenTestData
	if err := json.Unmarshal(data, &gd); err != nil {
		return nil, 0, 0, 0, err
	}

	pixels, err = DecodeBase64(gd.PixelsBase64)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	return pixels, gd.Width, gd.Height, gd.Components, nil
}

// writeGoldenJSONE writes a GoldenTestData to a JSON file. Used by golden file generation.
func writeGoldenJSONE(path string, gd GoldenTestData) error {
	data, err := json.MarshalIndent(gd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
