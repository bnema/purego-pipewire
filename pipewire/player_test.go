package pipewire

import (
	"testing"
)

func TestNewPlayerRejectsInvalidConfig(t *testing.T) {
	_, err := NewPlayer(PlayerConfig{}, PlayerCallbacks{})
	if err == nil {
		t.Fatal("NewPlayer returned nil error for invalid config")
	}
}
