package core

import (
	"errors"
	"sync"
	"sync/atomic"
)

var ErrInvalidPlayerState = errors.New("invalid player state transition")

// playerConfig holds configuration for the player
type playerConfig struct {
	// Minimal for now, will be extended in future tasks
}

// playerCallbacks holds callbacks for player events
type playerCallbacks struct {
	OnStateChange func(PlayerState)
}

// player is the internal player shell with state machine
type player struct {
	config    playerConfig
	callbacks playerCallbacks
	state     atomic.Int32
	terminal  atomic.Bool
	mu        sync.Mutex
}

// newPlayer creates a new player instance
func newPlayer(config playerConfig, callbacks playerCallbacks) *player {
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
		return next == PlayerStateStarting
	case PlayerStateStarting:
		return next == PlayerStatePlaying || next == PlayerStateError
	case PlayerStatePlaying:
		return next == PlayerStatePaused || next == PlayerStateStopped || next == PlayerStateClosing
	case PlayerStatePaused:
		return next == PlayerStatePlaying || next == PlayerStateStopped || next == PlayerStateClosing
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
