package parser

import (
	"encoding/json"
	"os"

	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/model"
)

// Load reads and parses a binding model JSON file.
func Load(path string) (*model.Model, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m model.Model
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
