package loader

import (
	"testing"
)

func TestLibFileNameLinuxDefault(t *testing.T) {
	if got, want := libFileName("libpipewire-0.3", "0"), "libpipewire-0.3.so.0"; got != want {
		t.Fatalf("libFileName=%q want %q", got, want)
	}
}

func TestResolveLibEnvOverride(t *testing.T) {
	t.Run("fallback", func(t *testing.T) {
		t.Setenv("TEST_LIB_PATH", "")
		if got := resolveLib("TEST_LIB_PATH", "fallback.so"); got != "fallback.so" {
			t.Errorf("resolveLib fallback=%q want %q", got, "fallback.so")
		}
	})

	t.Run("override", func(t *testing.T) {
		t.Setenv("TEST_LIB_PATH", "/custom/path/lib.so")
		if got := resolveLib("TEST_LIB_PATH", "fallback.so"); got != "/custom/path/lib.so" {
			t.Errorf("resolveLib env=%q want %q", got, "/custom/path/lib.so")
		}
	})
}
