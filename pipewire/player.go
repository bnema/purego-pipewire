package pipewire

import (
	"errors"
)

// SampleFormat represents the audio sample format type.
type SampleFormat int

const (
	// SampleFormatF32 is the 32-bit floating point sample format.
	SampleFormatF32 SampleFormat = iota
)

// UnderrunPolicy defines the behavior when a buffer underrun occurs.
type UnderrunPolicy int

const (
	// UnderrunFillSilence fills the buffer with silence on underrun.
	UnderrunFillSilence UnderrunPolicy = iota
	// UnderrunFail fails playback on underrun.
	UnderrunFail
)

// PlayerState represents the current state of the audio player.
type PlayerState int

const (
	// Idle is the initial state before the player is started.
	Idle PlayerState = iota
	// Starting indicates the player is initializing.
	Starting
	// Playing indicates the player is actively outputting audio.
	Playing
	// Paused indicates the player is temporarily stopped.
	Paused
	// Stopped indicates the player has been stopped.
	Stopped
	// Closing indicates the player is shutting down.
	Closing
	// Closed indicates the player has been fully closed.
	Closed
	// Error indicates the player encountered an error.
	Error
)

// PlayerConfig holds the configuration for a new audio player.
type PlayerConfig struct {
	SampleRate      int
	Channels        int
	FramesPerBuffer int
	SampleFormat    SampleFormat
	UnderrunPolicy  UnderrunPolicy
}

// PCMBuffer represents a buffer of PCM audio data.
type PCMBuffer struct {
	Frames   int
	Channels int
	Stride   int
	Samples  [][]float32
}

// PlayerCallbacks contains all callback functions for player events.
type PlayerCallbacks struct {
	Fill          func(*PCMBuffer) (int, error)
	OnUnderrun    func(int)
	OnDrain       func()
	OnError       func(error)
	OnStateChange func(PlayerState)
}

// Player is the public interface for audio playback.
type Player interface {
	Start() error
	Pause() error
	Stop() error
	Close() error
	State() PlayerState
}

// ErrInvalidPlayerConfig is returned when the player configuration is invalid.
var ErrInvalidPlayerConfig = errors.New("invalid player config")

// playerImpl is a placeholder implementation of the Player interface.
// Full implementation will be added in subsequent tasks.
type playerImpl struct {
	config    PlayerConfig
	callbacks PlayerCallbacks
	state     PlayerState
}

func (p *playerImpl) Start() error {
	return nil
}

func (p *playerImpl) Pause() error {
	return nil
}

func (p *playerImpl) Stop() error {
	return nil
}

func (p *playerImpl) Close() error {
	return nil
}

func (p *playerImpl) State() PlayerState {
	return p.state
}

// NewPlayer creates a new audio player with the given configuration and callbacks.
// Returns ErrInvalidPlayerConfig if the configuration is invalid.
func NewPlayer(config PlayerConfig, callbacks PlayerCallbacks) (Player, error) {
	if config.SampleRate <= 0 {
		return nil, ErrInvalidPlayerConfig
	}
	if config.Channels <= 0 {
		return nil, ErrInvalidPlayerConfig
	}
	if config.FramesPerBuffer <= 0 {
		return nil, ErrInvalidPlayerConfig
	}
	if config.SampleFormat != SampleFormatF32 {
		return nil, ErrInvalidPlayerConfig
	}

	return &playerImpl{
		config:    config,
		callbacks: callbacks,
		state:     Idle,
	}, nil
}
