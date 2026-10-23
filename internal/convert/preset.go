package convert

import (
	"encoding/json"
	"fmt"
	"os"
)

// Preset stores a mapping plus options for reuse across sessions.
type Preset struct {
	Name    string  `json:"name"`
	Mapping Mapping `json:"mapping"`
	Options Options `json:"options"`
}

// SavePreset writes the preset as JSON.
func SavePreset(path string, p Preset) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadPreset reads a preset JSON file.
func LoadPreset(path string) (Preset, error) {
	var p Preset
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("プリセットファイルが読み込めません: %w", err)
	}
	return p, nil
}
