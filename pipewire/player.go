package pipewire

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by stub functions that are not yet implemented.
var ErrNotImplemented = errors.New("not implemented")

// AudioFormat describes the audio format for a playback stream.
type AudioFormat struct {
	SampleRate      int
	Channels        int
	FramesPerBuffer int
}

// PlaybackStream is the public interface for writing audio samples to PipeWire.
type PlaybackStream interface {
	Write(samples [][]float32) error
	Close() error
}

// Player is the public interface for creating audio playback streams.
type Player interface {
	NewStream(ctx context.Context, format AudioFormat) (PlaybackStream, error)
	Close() error
}

// NewPlayer creates a new Player for audio playback.
// This is currently a stub and returns ErrNotImplemented.
func NewPlayer() (Player, error) {
	return nil, ErrNotImplemented
}
