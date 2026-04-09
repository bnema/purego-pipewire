package pipewire

import (
	"errors"
	"testing"
)

func TestNewPlayerRejectsInvalidConfig(t *testing.T) {
	_, err := NewPlayer(PlayerConfig{}, PlayerCallbacks{})
	if err == nil {
		t.Fatal("NewPlayer returned nil error for invalid config")
	}
	if !errors.Is(err, ErrInvalidPlayerConfig) {
		t.Fatalf("expected ErrInvalidPlayerConfig, got %v", err)
	}
}

func TestNewPlayerAcceptsValidConfig(t *testing.T) {
	config := PlayerConfig{
		SampleRate:      44100,
		Channels:        2,
		FramesPerBuffer: 256,
		SampleFormat:    SampleFormatF32,
	}
	player, err := NewPlayer(config, PlayerCallbacks{})
	if err != nil {
		t.Fatalf("NewPlayer returned error for valid config: %v", err)
	}
	if player == nil {
		t.Fatal("NewPlayer returned nil player for valid config")
	}
}

func TestPlayerStateReflectsInitialState(t *testing.T) {
	config := PlayerConfig{
		SampleRate:      44100,
		Channels:        2,
		FramesPerBuffer: 256,
		SampleFormat:    SampleFormatF32,
	}
	player, err := NewPlayer(config, PlayerCallbacks{})
	if err != nil {
		t.Fatalf("NewPlayer returned error: %v", err)
	}
	if player.State() != Idle {
		t.Fatalf("expected initial state Idle, got %v", player.State())
	}
}
