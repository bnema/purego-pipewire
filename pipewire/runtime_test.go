package pipewire

import "testing"

func TestInitReturnsRuntime(t *testing.T) {
	r, err := Init()
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if r == nil {
		t.Fatal("Init returned nil runtime")
	}
}
