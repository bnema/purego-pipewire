//go:build integration

package integration

import (
	"testing"
	"time"

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

// TestPlayerPlaybackInvokesFill proves that starting a Player under a live
// PipeWire daemon causes the process callback to invoke the user-supplied Fill
// callback. A buffered channel synchronizes the test instead of sleeping.
func TestPlayerPlaybackInvokesFill(t *testing.T) {
	initResult, err := pipewire.Init()
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	defer initResult.Deinit()

	filled := make(chan struct{}, 1) // buffered so Fill never blocks

	config := pipewire.PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 512,
		SampleFormat:    pipewire.SampleFormatF32,
		UnderrunPolicy:  pipewire.UnderrunFillSilence,
	}

	callbacks := pipewire.PlayerCallbacks{
		Fill: func(buf *pipewire.PCMBuffer) (int, error) {
			// Write a small constant so the buffer isn't silent.
			const val = 0.001
			for ch := range buf.Samples {
				for i := range buf.Samples[ch] {
					buf.Samples[ch][i] = val
				}
			}
			// Signal exactly once.
			select {
			case filled <- struct{}{}:
			default:
			}
			return buf.Frames, nil
		},
	}

	player, err := pipewire.NewPlayer(config, callbacks)
	if err != nil {
		t.Fatalf("NewPlayer returned error: %v", err)
	}

	if err := player.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	// Wait for the Fill callback to fire at least once.
	select {
	case <-filled:
		// success — the real PipeWire process callback invoked Fill
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for Fill callback; PipeWire process callback did not fire")
	}

	if err := player.Stop(); err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
	if err := player.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}
