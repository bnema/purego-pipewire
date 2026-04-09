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

func TestLoadEmptyLibraryNameReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_lib_name.json")
	if err == nil {
		t.Fatal("expected error for empty library name, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("error message should contain 'empty name', got: %v", err)
	}
}

func TestLoadEmptyLibrarySONameReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_lib_soname.json")
	if err == nil {
		t.Fatal("expected error for empty library soname, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "empty soname") {
		t.Fatalf("error message should contain 'empty soname', got: %v", err)
	}
}

func TestLoadEmptyGroupNameReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_group_name.json")
	if err == nil {
		t.Fatal("expected error for empty group name, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("error message should contain 'empty name', got: %v", err)
	}
}

func TestLoadEmptySymbolNameReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_symbol_name.json")
	if err == nil {
		t.Fatal("expected error for empty symbol name, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("error message should contain 'empty name', got: %v", err)
	}
}

func TestLoadEmptySymbolSignatureReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_symbol_sig.json")
	if err == nil {
		t.Fatal("expected error for empty symbol signature, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "empty signature") {
		t.Fatalf("error message should contain 'empty signature', got: %v", err)
	}
}

func TestLoadEmptyCallbackNameReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_callback_name.json")
	if err == nil {
		t.Fatal("expected error for empty callback name, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "callback") && !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("error message should mention callback empty name, got: %v", err)
	}
}

func TestLoadEmptyCallbackSignatureReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_callback_signature.json")
	if err == nil {
		t.Fatal("expected error for empty callback signature, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "callback") && !strings.Contains(err.Error(), "empty signature") {
		t.Fatalf("error message should mention callback empty signature, got: %v", err)
	}
}

func TestLoadEmptyEventStructNameReturnsError(t *testing.T) {
	_, err := Load("testdata/empty_event_struct_name.json")
	if err == nil {
		t.Fatal("expected error for empty event struct name, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("error message should contain 'validation failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "event struct") && !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("error message should mention event struct empty name, got: %v", err)
	}
}
