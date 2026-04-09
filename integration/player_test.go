//go:build integration

package integration

import (
	"testing"

	"github.com/bnema/purego-pipewire/pipewire"
)

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

	// Test Start
	if err := player.Start(); err != nil {
		t.Errorf("Start returned error: %v", err)
	}

	// Test Stop
	if err := player.Stop(); err != nil {
		t.Errorf("Stop returned error: %v", err)
	}

	// Test Close
	if err := player.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}
