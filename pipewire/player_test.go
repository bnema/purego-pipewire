package pipewire

import (
	"errors"
	"testing"
)

func TestNewPlayerReturnsError(t *testing.T) {
	p, err := NewPlayer()
	if err == nil {
		t.Fatal("expected NewPlayer to return an error (not yet implemented)")
	}
	if p != nil {
		t.Fatal("expected NewPlayer to return nil player when erroring")
	}
	// Verify the error message indicates not-implemented.
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got: %v", err)
	}
}
