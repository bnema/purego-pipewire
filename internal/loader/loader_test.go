package loader

import "testing"

func TestLibFileNameLinuxDefault(t *testing.T) {
	if got, want := libFileName("libpipewire-0.3", "0"), "libpipewire-0.3.so.0"; got != want {
		t.Fatalf("libFileName=%q want %q", got, want)
	}
}
