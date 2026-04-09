//go:build integration

package integration

import (
	"testing"

	"github.com/bnema/purego-pipewire/pipewire"
)

func TestInitSmoke(t *testing.T) {
	r, err := pipewire.Init()
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	r.Deinit()
}
