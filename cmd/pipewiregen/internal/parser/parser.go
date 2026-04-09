package parser

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/model"
)

// Load reads and parses a binding model JSON file.
func Load(path string) (*model.Model, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	var m model.Model
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON from %s: %w", path, err)
	}
	if err := validate(&m); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return &m, nil
}

func validate(m *model.Model) error {
	// Build set of valid library names
	libNames := make(map[string]bool)
	for i, lib := range m.Libraries {
		if lib.Name == "" {
			return fmt.Errorf("library at index %d has empty name", i)
		}
		if lib.SOName == "" {
			return fmt.Errorf("library %q has empty soname", lib.Name)
		}
		libNames[lib.Name] = true
	}

	// Validate groups
	for i, group := range m.Groups {
		if group.Name == "" {
			return fmt.Errorf("group at index %d has empty name", i)
		}
	}

	// Validate symbols
	for i, sym := range m.Symbols {
		if sym.Name == "" {
			return fmt.Errorf("symbol at index %d has empty name", i)
		}
		if sym.Library == "" {
			return fmt.Errorf("symbol %q has empty library", sym.Name)
		}
		if sym.Signature == "" {
			return fmt.Errorf("symbol %q has empty signature", sym.Name)
		}
		if !libNames[sym.Library] {
			return fmt.Errorf("symbol %q references unknown library %q", sym.Name, sym.Library)
		}
	}

	return nil
}
