package core

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var (
	ErrInvalidPlayerState = errors.New("invalid player state transition")
	ErrPlayerClosed       = errors.New("player is closed")
)

// UnderrunPolicy determines how to handle underruns
type UnderrunPolicy int

const (
	UnderrunFillSilence UnderrunPolicy = iota
	UnderrunFailFast
)

// PCMBuffer represents a PCM audio buffer with deinterleaved samples
// Samples[channel][frame] format for cache efficiency
type PCMBuffer struct {
	Frames   int
	Channels int
	Stride   int
	Samples  [][]float32
}

// allocate allocates the Samples slice structure
func (b *PCMBuffer) allocate() {
	if b.Samples == nil {
		b.Samples = make([][]float32, b.Channels)
		for ch := 0; ch < b.Channels; ch++ {
			b.Samples[ch] = make([]float32, b.Frames)
		}
	}
}

// PlayerConfig holds configuration for the player
type PlayerConfig struct {
	FramesPerBuffer int
	Channels        int
	UnderrunPolicy  UnderrunPolicy
}

// PlayerCallbacks holds callbacks for player events
type PlayerCallbacks struct {
	OnStateChange func(PlayerState)
	Fill          func(*PCMBuffer) (int, error)
	OnUnderrun    func(int)
	OnDrain       func()
	OnError       func(error)
}

// playerConfig is the internal alias for PlayerConfig
type playerConfig = PlayerConfig

// playerCallbacks is the internal alias for PlayerCallbacks
type playerCallbacks = PlayerCallbacks

// player is the internal player shell with state machine
type player struct {
	config    playerConfig
	callbacks playerCallbacks
	state     atomic.Int32
	terminal  atomic.Bool
	paused    atomic.Bool
	mu        sync.Mutex
}

// newPlayer creates a new player instance
func newPlayer(config PlayerConfig, callbacks PlayerCallbacks) *player {
	p := &player{
		config:    config,
		callbacks: callbacks,
	}
	p.state.Store(int32(PlayerStateIdle))
	return p
}

// State returns the current player state
func (p *player) State() PlayerState {
	return PlayerState(p.state.Load())
}

// setState atomically sets the player state
func (p *player) setState(next PlayerState) {
	p.state.Store(int32(next))
}

// isPaused returns true if the player is paused
func (p *player) isPaused() bool {
	return p.paused.Load()
}

// transition attempts to transition to the next state
func (p *player) transition(next PlayerState) error {
	p.mu.Lock()

	current := p.State()

	// Validate transition
	if !isValidTransition(current, next) {
		p.mu.Unlock()
		return ErrInvalidPlayerState
	}

	// Perform transition
	p.setState(next)

	// Capture callback under lock
	cb := p.callbacks.OnStateChange

	p.mu.Unlock()

	// Invoke callback after releasing lock to avoid deadlock on reentry
	if cb != nil {
		cb(next)
	}

	return nil
}

// isValidTransition checks if a state transition is valid
func isValidTransition(current, next PlayerState) bool {
	switch current {
	case PlayerStateIdle:
		return next == PlayerStateStarting
	case PlayerStateStarting:
		return next == PlayerStatePlaying || next == PlayerStateError
	case PlayerStatePlaying:
		return next == PlayerStatePaused || next == PlayerStateStopped || next == PlayerStateClosing || next == PlayerStateError
	case PlayerStatePaused:
		return next == PlayerStatePlaying || next == PlayerStateStopped || next == PlayerStateClosing || next == PlayerStateError
	case PlayerStateStopped:
		return next == PlayerStateStarting || next == PlayerStateClosing
	case PlayerStateClosing:
		return next == PlayerStateClosed
	case PlayerStateClosed:
		return false // Terminal state
	case PlayerStateError:
		return next == PlayerStateClosing
	default:
		return false
	}
}

// Start begins playback, transitioning from Idle or Stopped to Playing
func (p *player) Start() error {
	// Check if player is in terminal state
	current := p.State()
	if current == PlayerStateClosed || current == PlayerStateClosing {
		return ErrPlayerClosed
	}

	// Only valid from Idle or Stopped
	if current != PlayerStateIdle && current != PlayerStateStopped {
		return ErrInvalidPlayerState
	}

	// Ensure runtime is available (placeholder)
	if err := p.ensureRuntime(); err != nil {
		return err
	}

	// Transition to Starting then Playing
	if err := p.transition(PlayerStateStarting); err != nil {
		return err
	}

	// Clear paused flag
	p.setPaused(false)

	// Transition to Playing
	return p.transition(PlayerStatePlaying)
}

// Pause temporarily pauses playback
func (p *player) Pause() error {
	current := p.State()
	if current == PlayerStateClosed || current == PlayerStateClosing {
		return ErrPlayerClosed
	}

	// Only valid from Playing
	if current != PlayerStatePlaying {
		return ErrInvalidPlayerState
	}

	p.setPaused(true)
	return p.transition(PlayerStatePaused)
}

// Stop stops playback but allows restart
func (p *player) Stop() error {
	current := p.State()
	if current == PlayerStateClosed || current == PlayerStateClosing {
		return ErrPlayerClosed
	}

	// Valid from Playing, Paused, or Idle
	if current != PlayerStatePlaying && current != PlayerStatePaused && current != PlayerStateIdle {
		return ErrInvalidPlayerState
	}

	// Clear paused flag
	p.setPaused(false)

	// Deactivate stream (placeholder)
	p.deactivateStream()

	return p.transition(PlayerStateStopped)
}

// Close permanently shuts down the player
func (p *player) Close() error {
	current := p.State()

	// Idempotent on already closed
	if current == PlayerStateClosed {
		return nil
	}

	// Transition to Closing if not already there
	if current != PlayerStateClosing {
		if err := p.transition(PlayerStateClosing); err != nil {
			return err
		}
	}

	// Teardown resources (placeholder)
	p.teardown()

	// Transition to Closed
	return p.transition(PlayerStateClosed)
}

// ensureRuntime ensures the runtime is available (placeholder)
func (p *player) ensureRuntime() error {
	// Minimal placeholder - will be implemented in future tasks
	return nil
}

// setPaused sets the paused flag
func (p *player) setPaused(paused bool) {
	p.paused.Store(paused)
}

// deactivateStream deactivates the stream (placeholder)
func (p *player) deactivateStream() {
	// Minimal placeholder - will be implemented in future tasks
}

// teardown performs cleanup (placeholder)
func (p *player) teardown() {
	// Minimal placeholder - will be implemented in future tasks
}

// processPCM processes PCM audio data for the given buffer
// Returns the number of frames processed, or an error
func (p *player) processPCM(buf *PCMBuffer) (int, error) {
	// Ensure buffer is allocated
	buf.allocate()

	state := p.State()

	// If paused or stopped, fill with silence and return
	if state == PlayerStatePaused || state == PlayerStateStopped || state == PlayerStateIdle {
		p.fillSilence(buf, 0, buf.Frames)
		return buf.Frames, nil
	}

	// If not playing, fill with silence
	if state != PlayerStatePlaying {
		p.fillSilence(buf, 0, buf.Frames)
		return buf.Frames, nil
	}

	// Call Fill callback
	if p.callbacks.Fill == nil {
		p.fillSilence(buf, 0, buf.Frames)
		return buf.Frames, nil
	}

	frames, err := p.callbacks.Fill(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			// Drain condition - emit drain callback and return frames
			p.emitDrain()
			return frames, nil
		}
		// Other error - fail the player
		p.fail(err)
		return 0, err
	}

	// Handle underrun if Fill returned fewer frames than requested
	if frames < buf.Frames {
		return p.handleUnderrun(buf, frames)
	}

	return frames, nil
}

// fillSilence fills the buffer with silence (0.0) from startFrame to endFrame
func (p *player) fillSilence(buf *PCMBuffer, startFrame, endFrame int) {
	for ch := 0; ch < buf.Channels; ch++ {
		for frame := startFrame; frame < endFrame && frame < len(buf.Samples[ch]); frame++ {
			buf.Samples[ch][frame] = 0.0
		}
	}
}

// handleUnderrun applies the underrun policy when Fill returns fewer frames than requested
func (p *player) handleUnderrun(buf *PCMBuffer, filledFrames int) (int, error) {
	// Emit underrun callback
	if p.callbacks.OnUnderrun != nil {
		p.callbacks.OnUnderrun(buf.Frames - filledFrames)
	}

	switch p.config.UnderrunPolicy {
	case UnderrunFailFast:
		err := errors.New("underrun occurred")
		p.fail(err)
		return 0, err
	case UnderrunFillSilence:
		fallthrough
	default:
		p.fillSilence(buf, filledFrames, buf.Frames)
		return buf.Frames, nil
	}
}

// emitDrain invokes the OnDrain callback
func (p *player) emitDrain() {
	if p.callbacks.OnDrain != nil {
		p.callbacks.OnDrain()
	}
}

// fail transitions the player to error state and invokes OnError
func (p *player) fail(err error) {
	p.transition(PlayerStateError)
	if p.callbacks.OnError != nil {
		p.callbacks.OnError(err)
	}
}

// Player is the exported wrapper for the internal player
type Player struct {
	p *player
}

// NewPlayer creates a new exported Player instance
func NewPlayer(config playerConfig, callbacks playerCallbacks) *Player {
	return &Player{p: newPlayer(config, callbacks)}
}

// State returns the current player state
func (p *Player) State() PlayerState {
	if p.p == nil {
		return PlayerStateIdle
	}
	return p.p.State()
}

// Start begins playback
func (p *Player) Start() error {
	if p.p == nil {
		return ErrPlayerClosed
	}
	return p.p.Start()
}

// Pause temporarily pauses playback
func (p *Player) Pause() error {
	if p.p == nil {
		return ErrPlayerClosed
	}
	return p.p.Pause()
}

// Stop stops playback but allows restart
func (p *Player) Stop() error {
	if p.p == nil {
		return ErrPlayerClosed
	}
	return p.p.Stop()
}

// Close permanently shuts down the player
func (p *Player) Close() error {
	if p.p == nil {
		return nil
	}
	return p.p.Close()
}
