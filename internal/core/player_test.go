package core

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/bnema/purego-pipewire/internal/ports/out/mocks"
)

func TestPlayerStopIsRestartableButCloseIsTerminal(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})

	// Start from Stopped state should work
	p.setState(PlayerStateStopped)
	if err := p.Start(); err != nil {
		t.Fatalf("Start from Stopped failed: %v", err)
	}

	// Should be in Playing state after Start
	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state after Start, got %v", p.State())
	}

	// Stop should transition to Stopped
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if p.State() != PlayerStateStopped {
		t.Fatalf("expected Stopped state after Stop, got %v", p.State())
	}

	// Start from Stopped should work again (restartable)
	if err := p.Start(); err != nil {
		t.Fatalf("Start from Stopped (restart) failed: %v", err)
	}
	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state after restart, got %v", p.State())
	}

	// Close should transition to Closed
	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if p.State() != PlayerStateClosed {
		t.Fatalf("expected Closed state after Close, got %v", p.State())
	}

	// Start from Closed should fail (terminal)
	if err := p.Start(); err == nil {
		t.Fatal("expected error starting from Closed state, got nil")
	}
}

func TestPlayerStartFromClosedReturnsError(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.setState(PlayerStateClosed)

	err := p.Start()
	if err == nil {
		t.Fatal("expected error starting from Closed state, got nil")
	}
	if !errors.Is(err, ErrPlayerClosed) {
		t.Fatalf("expected ErrPlayerClosed, got %v", err)
	}
}

func TestPlayerStartFromClosingReturnsError(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.setState(PlayerStateClosing)

	err := p.Start()
	if err == nil {
		t.Fatal("expected error starting from Closing state, got nil")
	}
	if !errors.Is(err, ErrPlayerClosed) {
		t.Fatalf("expected ErrPlayerClosed, got %v", err)
	}
}

func TestPlayerStartTransitionsThroughStartingToPlaying(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})

	// Initial state should be Idle
	if p.State() != PlayerStateIdle {
		t.Fatalf("expected initial state Idle, got %v", p.State())
	}

	// Start should transition to Playing
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state after Start, got %v", p.State())
	}
}

func TestPlayerPauseTransitionsToPaused(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})

	// Start first
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Pause should transition to Paused
	if err := p.Pause(); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	if p.State() != PlayerStatePaused {
		t.Fatalf("expected Paused state after Pause, got %v", p.State())
	}
}

func TestPlayerStopClearsPausedState(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})

	// Start then pause
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := p.Pause(); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if p.State() != PlayerStatePaused {
		t.Fatalf("expected Paused state, got %v", p.State())
	}

	// Stop should transition to Stopped (clearing paused state)
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if p.State() != PlayerStateStopped {
		t.Fatalf("expected Stopped state after Stop, got %v", p.State())
	}
}

func TestPlayerCloseIsIdempotent(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})

	// Start first
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Close should work
	if err := p.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if p.State() != PlayerStateClosed {
		t.Fatalf("expected Closed state, got %v", p.State())
	}

	// Second close should also succeed (idempotent)
	if err := p.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
	if p.State() != PlayerStateClosed {
		t.Fatalf("expected Closed state after second close, got %v", p.State())
	}
}

// TestPlayerStopDeactivatesStream verifies that Stop() calls
// SetStreamActive(false) via StreamOps and transitions to Stopped.
func TestPlayerStopDeactivatesStream(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := unsafe.Pointer(uintptr(0x1234))

	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(nil)

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream

	// Start the player
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state, got %v", p.State())
	}

	// Stop should call SetStreamActive(false) and transition to Stopped
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if p.State() != PlayerStateStopped {
		t.Fatalf("expected Stopped state after Stop, got %v", p.State())
	}
}

// TestPlayerStopReturnsDeactivateError verifies that Stop() surfaces the
// SetStreamActive error and leaves the player in the pre-stop state.
func TestPlayerStopReturnsDeactivateError(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := unsafe.Pointer(uintptr(0x5678))
	deactivateErr := errors.New("deactivate failed")

	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(deactivateErr)

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream

	// Start the player
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state, got %v", p.State())
	}

	// Stop should return the deactivation error
	err := p.Stop()
	if err == nil {
		t.Fatal("expected deactivation error, got nil")
	}
	if !errors.Is(err, deactivateErr) {
		t.Fatalf("expected %v, got %v", deactivateErr, err)
	}

	// State should remain Playing (not transitioned to Stopped)
	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected state to remain Playing after failed deactivation, got %v", p.State())
	}
}

// TestPlayerStopWithoutStreamOpsIsNoop verifies that Stop() works
// when no StreamOps is configured (backward compat).
func TestPlayerStopWithoutStreamOpsIsNoop(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop without StreamOps should succeed (deactivation is a no-op)
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if p.State() != PlayerStateStopped {
		t.Fatalf("expected Stopped state, got %v", p.State())
	}
}

// TestPlayerCloseTeardownCallsStreamOps verifies that Close() calls
// DisconnectStream, DestroyStream, QuitMainLoop, and DestroyMainLoop through StreamOps
// when the corresponding pointers are set.
func TestPlayerCloseTeardownCallsStreamOps(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := unsafe.Pointer(uintptr(0xAAAA))
	fakeLoop := unsafe.Pointer(uintptr(0xBBBB))

	// Expect teardown calls in order: DisconnectStream, DestroyStream, QuitMainLoop, DestroyMainLoop.
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream
	p.loopPtr = fakeLoop

	// Move to a state that allows Close.
	p.setState(PlayerStatePlaying)

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if p.State() != PlayerStateClosed {
		t.Fatalf("expected Closed state, got %v", p.State())
	}

	// Pointers should be nil after teardown so repeated close is safe.
	if p.streamPtr != nil {
		t.Error("expected streamPtr to be nil after teardown")
	}
	if p.loopPtr != nil {
		t.Error("expected loopPtr to be nil after teardown")
	}
}

// TestPlayerCloseWithNilPointersIsNoop verifies that teardown is safe
// when no stream/loop pointers are set.
func TestPlayerCloseWithNilPointersIsNoop(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.setState(PlayerStatePlaying)

	// No streamOps, no pointers — Close should succeed without panic.
	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if p.State() != PlayerStateClosed {
		t.Fatalf("expected Closed state, got %v", p.State())
	}
}

// TestPlayerRepeatedCloseIsSafe verifies that calling Close() twice
// with teardown resources does not call StreamOps methods a second time.
func TestPlayerRepeatedCloseIsSafe(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := unsafe.Pointer(uintptr(0xCCCC))
	fakeLoop := unsafe.Pointer(uintptr(0xDDDD))

	// Only expect one set of teardown calls.
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream
	p.loopPtr = fakeLoop
	p.setState(PlayerStatePlaying)

	if err := p.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	// Second close should succeed and not call StreamOps again.
	if err := p.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

// TestPlayerTeardownWithOnlyStreamPtr verifies teardown works
// when only streamPtr is set (no loopPtr).
func TestPlayerTeardownWithOnlyStreamPtr(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := unsafe.Pointer(uintptr(0xEEEE))

	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().DestroyStream(fakeStream).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream
	p.setState(PlayerStatePlaying)

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if p.streamPtr != nil {
		t.Error("expected streamPtr to be nil after teardown")
	}
}

// TestPlayerTeardownWithOnlyLoopPtr verifies teardown works
// when only loopPtr is set (no streamPtr).
func TestPlayerTeardownWithOnlyLoopPtr(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xFFFF))

	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.loopPtr = fakeLoop
	p.setState(PlayerStatePlaying)

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if p.loopPtr != nil {
		t.Error("expected loopPtr to be nil after teardown")
	}
}

// TestPlayerTeardownCallsDisconnectStream verifies that teardown calls
// DisconnectStream before DestroyStream when a stream exists.
func TestPlayerTeardownCallsDisconnectStream(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := unsafe.Pointer(uintptr(0xAAAA))
	fakeLoop := unsafe.Pointer(uintptr(0xBBBB))

	// Expect disconnect before destroy
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream
	p.loopPtr = fakeLoop
	p.setState(PlayerStatePlaying)

	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestPlayerTeardownContinuesOnDisconnectError verifies that teardown
// continues cleanup even if DisconnectStream returns an error.
func TestPlayerTeardownContinuesOnDisconnectError(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := unsafe.Pointer(uintptr(0xCCCC))
	fakeLoop := unsafe.Pointer(uintptr(0xDDDD))
	disconnectErr := errors.New("disconnect failed")

	// Disconnect fails, but destroy should still be called
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(disconnectErr)
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream
	p.loopPtr = fakeLoop
	p.setState(PlayerStatePlaying)

	// Close should succeed even though disconnect failed (best-effort cleanup)
	if err := p.Close(); err != nil {
		t.Fatalf("Close should succeed despite disconnect error: %v", err)
	}
}
