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

	// Build set of valid group names
	groupNames := make(map[string]bool)
	for i, group := range m.Groups {
		if group.Name == "" {
			return fmt.Errorf("group at index %d has empty name", i)
		}
		if group.Interface == "" {
			return fmt.Errorf("group %q has empty interface", group.Name)
		}
		groupNames[group.Name] = true
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
		if sym.Group == "" {
			return fmt.Errorf("symbol %q has empty group", sym.Name)
		}
		if !groupNames[sym.Group] {
			return fmt.Errorf("symbol %q references unknown group %q", sym.Name, sym.Group)
		}
	}

	// Validate callbacks
	for i, cb := range m.Callbacks {
		if cb.Name == "" {
			return fmt.Errorf("callback at index %d has empty name", i)
		}
		if cb.Signature == "" {
			return fmt.Errorf("callback %q has empty signature", cb.Name)
		}
		if cb.Group == "" {
			return fmt.Errorf("callback %q has empty group", cb.Name)
		}
		if !groupNames[cb.Group] {
			return fmt.Errorf("callback %q references unknown group %q", cb.Name, cb.Group)
		}
	}

	// Validate event structs
	for i, es := range m.EventStructs {
		if es.Name == "" {
			return fmt.Errorf("event struct at index %d has empty name", i)
		}
		if es.Group == "" {
			return fmt.Errorf("event struct %q has empty group", es.Name)
		}
		if !groupNames[es.Group] {
			return fmt.Errorf("event struct %q references unknown group %q", es.Name, es.Group)
		}
	}

	return nil
}
