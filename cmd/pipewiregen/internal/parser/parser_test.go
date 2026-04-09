package parser

import (
	"strings"
	"testing"
)

func TestLoadParsesLibraryAndSymbolGroups(t *testing.T) {
	model, err := Load("testdata/minimal_pipewire.json")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got, want := len(model.Libraries), 1; got != want {
		t.Fatalf("libraries=%d want %d", got, want)
	}
	if got, want := model.Groups[0].Name, "init"; got != want {
		t.Fatalf("group=%q want %q", got, want)
	}
}

func TestLoadMissingFileReturnsError(t *testing.T) {
	_, err := Load("testdata/nonexistent_file.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Fatalf("error message should contain 'failed to read file', got: %v", err)
	}
}

func TestLoadInvalidJSONReturnsError(t *testing.T) {
	_, err := Load("testdata/invalid.json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal JSON") {
		t.Fatalf("error message should contain 'failed to unmarshal JSON', got: %v", err)
	}
}

func TestLoadSymbolWithUnknownLibraryReturnsError(t *testing.T) {
	_, err := Load("testdata/unknown_library.json")
	if err == nil {
		t.Fatal("expected error for symbol with unknown library, got nil")
	}
	if !strings.Contains(err.Error(), "unknown library") {
		t.Fatalf("error message should contain 'unknown library', got: %v", err)
	}
}
