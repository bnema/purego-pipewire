package core

import (
	"testing"
	"testing/synctest"
)

func TestPlayerStateCallbackFanout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callbackStates []PlayerState
		callbacks := playerCallbacks{
			OnStateChange: func(state PlayerState) {
				callbackStates = append(callbackStates, state)
			},
		}

		p := newPlayer(playerConfig{}, callbacks)

		// Trigger a transition
		if err := p.transition(PlayerStateStarting); err != nil {
			t.Errorf("transition to Starting failed: %v", err)
		}

		synctest.Wait()
		// Verify callback was invoked with the new state
		if len(callbackStates) != 1 {
			t.Errorf("callback invoked %d times, want 1", len(callbackStates))
		}
		if len(callbackStates) > 0 && callbackStates[0] != PlayerStateStarting {
			t.Errorf("callback received state %v, want %v", callbackStates[0], PlayerStateStarting)
		}

		// Trigger another transition
		if err := p.transition(PlayerStatePlaying); err != nil {
			t.Errorf("transition to Playing failed: %v", err)
		}

		synctest.Wait()
		// Verify callback was invoked again
		if len(callbackStates) != 2 {
			t.Errorf("callback invoked %d times, want 2", len(callbackStates))
		}
		if len(callbackStates) > 1 && callbackStates[1] != PlayerStatePlaying {
			t.Errorf("callback received state %v, want %v", callbackStates[1], PlayerStatePlaying)
		}
	})
}

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

func TestStateCallbackTransitionOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var p *player
		states := make(chan PlayerState, 5)
		p = newPlayer(playerConfig{}, playerCallbacks{OnStateChange: func(s PlayerState) {
			states <- s
			if s == PlayerStateStarting {
				if err := p.transition(PlayerStatePlaying); err != nil {
					t.Error(err)
				}
			}
		}})
		if p.State() != PlayerStateIdle {
			t.Fatal("not Idle")
		}
		if err := p.transition(PlayerStateStarting); err != nil {
			t.Fatal(err)
		}
		for _, want := range []PlayerState{PlayerStateStarting, PlayerStatePlaying} {
			if got := <-states; got != want {
				t.Fatalf("got %v want %v", got, want)
			}
		}
		if err := p.transition(PlayerStateStopped); err != nil {
			t.Fatal(err)
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		for _, want := range []PlayerState{PlayerStateStopped, PlayerStateClosing, PlayerStateClosed} {
			if got := <-states; got != want {
				t.Fatalf("got %v want %v", got, want)
			}
		}
		synctest.Wait()
		p.mu.Lock()
		dispatching := p.dispatching
		p.mu.Unlock()
		if dispatching {
			t.Fatal("dispatcher did not exit")
		}
	})
}

func TestStateDispatcherRestartsAfterDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		states := make(chan PlayerState, 1)
		p := newPlayer(playerConfig{}, playerCallbacks{OnStateChange: func(s PlayerState) { states <- s }})
		for _, want := range []PlayerState{PlayerStateStarting, PlayerStatePlaying, PlayerStateStopped} {
			if err := p.transition(want); err != nil {
				t.Fatal(err)
			}
			if got := <-states; got != want {
				t.Fatalf("got %v want %v", got, want)
			}
			synctest.Wait()
			p.mu.Lock()
			dispatching, queued := p.dispatching, len(p.stateQueue)
			p.mu.Unlock()
			if dispatching || queued != 0 {
				t.Fatalf("dispatcher not drained: running=%v queued=%d", dispatching, queued)
			}
		}
	})
}
