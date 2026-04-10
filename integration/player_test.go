//go:build integration

package integration

import (
	"testing"

	"github.com/bnema/purego-pipewire/pipewire"
)

// TestPlayerStartStopClose verifies the player lifecycle against a real PipeWire
// daemon. This test requires a running PipeWire runtime and only validates
// externally visible state transitions (Idle -> Playing -> Stopped -> Closed).
// Deeper cleanup internals are covered by unit tests.
func TestPlayerStartStopClose(t *testing.T) {
	// Initialize PipeWire
	initResult, err := pipewire.Init()
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	defer initResult.Deinit()

	// Create player with specified configuration
	config := pipewire.PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 256,
		SampleFormat:    pipewire.SampleFormatF32,
		UnderrunPolicy:  pipewire.UnderrunFillSilence,
	}

	callbacks := pipewire.PlayerCallbacks{
		Fill: func(buf *pipewire.PCMBuffer) (int, error) {
			return buf.Frames, nil
		},
	}

	player, err := pipewire.NewPlayer(config, callbacks)
	if err != nil {
		t.Fatalf("NewPlayer returned error: %v", err)
	}

	// Verify initial state is Idle
	if state := player.State(); state != pipewire.PlayerStateIdle {
		t.Errorf("Expected initial state Idle, got %v", state)
	}

	// Test Start - should transition to Playing state
	if err := player.Start(); err != nil {
		t.Errorf("Start returned error: %v", err)
	}
	if state := player.State(); state != pipewire.PlayerStatePlaying {
		t.Errorf("After Start: expected state Playing, got %v", state)
	}

	// Test Stop - should transition to Stopped state
	if err := player.Stop(); err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
	if state := player.State(); state != pipewire.PlayerStateStopped {
		t.Errorf("After Stop: expected state Stopped, got %v", state)
	}

	// Test Close - should transition to Closed state
	if err := player.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
	if state := player.State(); state != pipewire.PlayerStateClosed {
		t.Errorf("After Close: expected state Closed, got %v", state)
	}
}
