package core

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

var (
	ErrInvalidPlayerState = errors.New("invalid player state transition")
	ErrPlayerClosed       = errors.New("player is closed")
)

// player is the internal player shell with state machine
type player struct {
	config    playerConfig
	callbacks playerCallbacks
	state     atomic.Int32
	paused    atomic.Bool
	mu        sync.Mutex

	// Stream cleanup fields — nil when no stream is active.
	streamOps portout.StreamOps
	streamPtr unsafe.Pointer
	loopPtr   unsafe.Pointer
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
		return next == PlayerStateStarting || next == PlayerStateClosing
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

// Stop stops playback but allows restart.
// Returns an error if stream deactivation fails, leaving the state unchanged.
func (p *player) Stop() error {
	current := p.State()
	if current == PlayerStateClosed || current == PlayerStateClosing {
		return ErrPlayerClosed
	}

	// Valid from Playing or Paused.
	if current != PlayerStatePlaying && current != PlayerStatePaused {
		return ErrInvalidPlayerState
	}

	// Deactivate stream - if this fails, return the error and don't transition
	if err := p.deactivateStream(); err != nil {
		return err
	}

	// Clear paused flag only after successful deactivation
	p.setPaused(false)

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

// deactivateStream deactivates the stream via StreamOps.
// Returns an error if deactivation fails. No-op when streamOps is nil.
func (p *player) deactivateStream() error {
	p.mu.Lock()
	streamOps := p.streamOps
	streamPtr := p.streamPtr
	p.mu.Unlock()

	if streamOps == nil || streamPtr == nil {
		return nil
	}
	return streamOps.SetStreamActive(streamPtr, false)
}

// teardown releases player-owned stream and main-loop resources through
// StreamOps. Safe to call when fields are nil or on repeated invocation.
func (p *player) teardown() {
	p.mu.Lock()
	streamOps := p.streamOps
	streamPtr := p.streamPtr
	loopPtr := p.loopPtr
	p.streamPtr = nil
	p.loopPtr = nil
	p.mu.Unlock()

	if streamOps == nil {
		return
	}

	// Destroy stream first (it depends on the loop).
	if streamPtr != nil {
		streamOps.DestroyStream(streamPtr)
	}

	// Quit then destroy the main loop.
	if loopPtr != nil {
		streamOps.QuitMainLoop(loopPtr)
		streamOps.DestroyMainLoop(loopPtr)
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
