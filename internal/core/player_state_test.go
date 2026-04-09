package core

import (
	"testing"
)

func TestPlayerStateTransitionsIdleStartStopClose(t *testing.T) {
	p := newPlayer(playerConfig{}, playerCallbacks{})

	if got := p.State(); got != PlayerStateIdle {
		t.Errorf("initial state = %v, want %v", got, PlayerStateIdle)
	}

	if err := p.transition(PlayerStateStarting); err != nil {
		t.Errorf("transition to Starting failed: %v", err)
	}
	if got := p.State(); got != PlayerStateStarting {
		t.Errorf("after transition to Starting, state = %v, want %v", got, PlayerStateStarting)
	}

	if err := p.transition(PlayerStatePlaying); err != nil {
		t.Errorf("transition to Playing failed: %v", err)
	}
	if got := p.State(); got != PlayerStatePlaying {
		t.Errorf("after transition to Playing, state = %v, want %v", got, PlayerStatePlaying)
	}

	if err := p.transition(PlayerStateStopped); err != nil {
		t.Errorf("transition to Stopped failed: %v", err)
	}
	if got := p.State(); got != PlayerStateStopped {
		t.Errorf("after transition to Stopped, state = %v, want %v", got, PlayerStateStopped)
	}

	if err := p.transition(PlayerStateClosing); err != nil {
		t.Errorf("transition to Closing failed: %v", err)
	}
	if got := p.State(); got != PlayerStateClosing {
		t.Errorf("after transition to Closing, state = %v, want %v", got, PlayerStateClosing)
	}

	if err := p.transition(PlayerStateClosed); err != nil {
		t.Errorf("transition to Closed failed: %v", err)
	}
	if got := p.State(); got != PlayerStateClosed {
		t.Errorf("after transition to Closed, state = %v, want %v", got, PlayerStateClosed)
	}
}
