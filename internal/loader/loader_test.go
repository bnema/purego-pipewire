package loader

import (
	"os"
	"testing"
)

func TestLibFileNameLinuxDefault(t *testing.T) {
	if got, want := libFileName("libpipewire-0.3", "0"), "libpipewire-0.3.so.0"; got != want {
		t.Fatalf("libFileName=%q want %q", got, want)
	}
}

func TestResolveLibEnvOverride(t *testing.T) {
	// Save and restore env
	orig := os.Getenv("TEST_LIB_PATH")
	defer os.Setenv("TEST_LIB_PATH", orig)

	// Test fallback when env is not set
	os.Unsetenv("TEST_LIB_PATH")
	if got := resolveLib("TEST_LIB_PATH", "fallback.so"); got != "fallback.so" {
		t.Errorf("resolveLib fallback=%q want %q", got, "fallback.so")
	}

	// Test env override
	os.Setenv("TEST_LIB_PATH", "/custom/path/lib.so")
	if got := resolveLib("TEST_LIB_PATH", "fallback.so"); got != "/custom/path/lib.so" {
		t.Errorf("resolveLib env=%q want %q", got, "/custom/path/lib.so")
	}
}
