package core

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
	"github.com/bnema/purego-pipewire/internal/ports/out/mocks"
	"github.com/stretchr/testify/mock"
)

// defaultTestConfig returns a PlayerConfig suitable for tests that need
// valid PipeWire format parameters.
func defaultTestConfig() PlayerConfig {
	return PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 256,
	}
}

// expectStartWithSync sets up mock expectations for a first-time Start()
// and returns a pointer to a WaitGroup that is signaled when RunMainLoop completes.
// The caller must call waitOnMainLoop(t, wg) after Start() returns.
func expectStartWithSync(mockOps *mocks.MockStreamOps, fakeLoop, fakeStream unsafe.Pointer, cfg PlayerConfig) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	mockOps.EXPECT().CreateMainLoop().Return(fakeLoop, nil)
	mockOps.EXPECT().CreatePlaybackStream(fakeLoop, "purego-pipewire-player", mock.AnythingOfType("func()")).Return(fakeStream, nil)
	mockOps.EXPECT().ConnectPlaybackStream(fakeStream, portout.PlaybackFormat{
		SampleRate:      cfg.SampleRate,
		Channels:        cfg.Channels,
		FramesPerBuffer: cfg.FramesPerBuffer,
	}).Return(nil)
	mockOps.EXPECT().SetStreamActive(fakeStream, true).Return(nil).Once()
	mockOps.EXPECT().RunMainLoop(fakeLoop).RunAndReturn(func(unsafe.Pointer) error {
		wg.Done()
		return nil
	})

	return wg
}

// waitOnMainLoop waits for the RunMainLoop goroutine to finish, with a timeout.
func waitOnMainLoop(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RunMainLoop goroutine")
	}
}

func TestPlayerStopIsRestartableButCloseIsTerminal(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()

	// First start creates resources
	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)
	// Stop deactivates the stream
	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(nil)
	// Restart reactivates existing stream
	mockOps.EXPECT().SetStreamActive(fakeStream, true).Return(nil)
	// Close tears down resources
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start from Stopped state should work
	p.setState(PlayerStateStopped)
	if err := p.Start(); err != nil {
		t.Fatalf("Start from Stopped failed: %v", err)
	}
	waitOnMainLoop(t, wg)

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

func TestPlayerStartCreateResourcesAndActivateKeepsCreateAndTransitionErrors(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	createErr := errors.New("create main loop failed")

	mockOps.EXPECT().CreateMainLoop().Return(nil, createErr)

	p := newPlayer(defaultTestConfig(), PlayerCallbacks{})
	p.streamOps = mockOps

	err := p.startCreateResourcesAndActivate()
	if err == nil {
		t.Fatal("expected startCreateResourcesAndActivate to fail, got nil")
	}
	if !errors.Is(err, createErr) {
		t.Fatalf("expected createErr, got %v", err)
	}
	if !errors.Is(err, ErrInvalidPlayerState) {
		t.Fatalf("expected ErrInvalidPlayerState, got %v", err)
	}
}

func TestPlayerEnsureRuntimeWrapsInitPipeWireError(t *testing.T) {
	origRegister := registerPipeWire
	origDefault := defaultPipeWireCAPI
	origOps := defaultPipeWireOps
	origInit := initializePipeWire
	t.Cleanup(func() {
		registerPipeWire = origRegister
		defaultPipeWireCAPI = origDefault
		defaultPipeWireOps = origOps
		initializePipeWire = origInit
	})

	initErr := errors.New("init pipewire failed")
	registerPipeWire = func() error { return nil }
	defaultPipeWireCAPI = func() portout.CAPI { return mocks.NewMockCAPI(t) }
	defaultPipeWireOps = func() portout.StreamOps {
		t.Fatal("defaultPipeWireOps should not be called when init fails")
		return nil
	}
	initializePipeWire = func(portout.CAPI) error { return initErr }

	p := newPlayer(defaultTestConfig(), PlayerCallbacks{})
	err := p.ensureRuntime()
	if err == nil {
		t.Fatal("expected ensureRuntime to fail, got nil")
	}
	if !errors.Is(err, initErr) {
		t.Fatalf("expected initErr, got %v", err)
	}
	if !strings.Contains(err.Error(), "initialize pipewire runtime") {
		t.Fatalf("expected context in error, got %v", err)
	}
}

func TestPlayerStartTransitionsThroughStartingToPlaying(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Initial state should be Idle
	if p.State() != PlayerStateIdle {
		t.Fatalf("expected initial state Idle, got %v", p.State())
	}

	// Start should transition to Playing
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state after Start, got %v", p.State())
	}
}

func TestPlayerPauseTransitionsToPaused(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start first
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	// Pause should transition to Paused
	if err := p.Pause(); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	if p.State() != PlayerStatePaused {
		t.Fatalf("expected Paused state after Pause, got %v", p.State())
	}
}

func TestPlayerStopClearsPausedState(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)
	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(nil)

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start then pause
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

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
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start first
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

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
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)
	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(nil)

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start the player
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)
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
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	deactivateErr := errors.New("deactivate failed")
	cfg := defaultTestConfig()

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)
	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(deactivateErr)

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start the player
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)
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

// TestPlayerStopWithNilStreamPtrIsNoop verifies that Stop() works
// when streamOps is set but streamPtr is nil (backward compat).
func TestPlayerStopWithNilStreamPtrIsNoop(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	cfg := defaultTestConfig()

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Set state to Playing manually (without going through Start)
	// to test Stop without an active stream pointer
	p.setState(PlayerStatePlaying)

	// Stop without a streamPtr should succeed (deactivation is a no-op)
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
	fakeStream := opaqueTestPtr()
	fakeLoop := opaqueTestPtr()

	// Expect teardown calls in order: DisconnectStream, QuitMainLoop, DestroyStream, DestroyMainLoop.
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
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
	fakeStream := opaqueTestPtr()
	fakeLoop := opaqueTestPtr()

	// Only expect one set of teardown calls.
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
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
	fakeStream := opaqueTestPtr()

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
	fakeLoop := opaqueTestPtr()

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

func TestPlayerCloseWaitsForMainLoopExitBeforeDestroy(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	loopDone := make(chan struct{})
	quitCalled := make(chan struct{})
	destroyCalled := make(chan struct{})
	closeReturned := make(chan error, 1)

	mockOps.EXPECT().QuitMainLoop(fakeLoop).Run(func(unsafe.Pointer) {
		close(quitCalled)
	}).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Run(func(unsafe.Pointer) {
		close(destroyCalled)
	}).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.loopPtr = fakeLoop
	p.loopDone = loopDone
	p.setState(PlayerStatePlaying)

	go func() {
		closeReturned <- p.Close()
	}()

	select {
	case <-quitCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QuitMainLoop")
	}

	select {
	case <-destroyCalled:
		t.Fatal("DestroyMainLoop called before loopDone was closed")
	case <-time.After(100 * time.Millisecond):
	}

	close(loopDone)

	select {
	case err := <-closeReturned:
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Close to return")
	}

	select {
	case <-destroyCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DestroyMainLoop after loopDone")
	}
}

func TestPlayerCloseWaitsForMainLoopExitBeforeDestroyingStream(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	loopDone := make(chan struct{})
	quitCalled := make(chan struct{})
	streamDestroyed := make(chan struct{})
	closeReturned := make(chan error, 1)

	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Run(func(unsafe.Pointer) {
		close(quitCalled)
	}).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Run(func(unsafe.Pointer) {
		close(streamDestroyed)
	}).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.streamOps = mockOps
	p.streamPtr = fakeStream
	p.loopPtr = fakeLoop
	p.loopDone = loopDone
	p.setState(PlayerStatePlaying)

	go func() {
		closeReturned <- p.Close()
	}()

	select {
	case <-quitCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QuitMainLoop")
	}

	select {
	case <-streamDestroyed:
		t.Fatal("DestroyStream called before loopDone was closed")
	case <-time.After(100 * time.Millisecond):
	}

	close(loopDone)

	select {
	case err := <-closeReturned:
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Close to return")
	}

	select {
	case <-streamDestroyed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DestroyStream after loopDone")
	}
}

// TestPlayerTeardownCallsDisconnectStream verifies that teardown calls
// DisconnectStream before DestroyStream when a stream exists.
func TestPlayerTeardownCallsDisconnectStream(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeStream := opaqueTestPtr()
	fakeLoop := opaqueTestPtr()

	// Expect disconnect before destroy, and stream destroy only after loop shutdown begins.
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
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
	fakeStream := opaqueTestPtr()
	fakeLoop := opaqueTestPtr()
	disconnectErr := errors.New("disconnect failed")

	// Disconnect fails, but teardown should still quit the loop and destroy the stream.
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(disconnectErr)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
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

// TestPlayerRestartActivationFailureCleansUpResources verifies that when
// SetStreamActive(true) fails during a restart (after Stop), owned stream
// and loop resources are cleaned up and no stale pointers remain.
func TestPlayerRestartActivationFailureCleansUpResources(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()
	activateErr := errors.New("activate failed on restart")

	// First start: create resources
	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)
	// Stop: deactivate
	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(nil)
	// Restart: activation fails → teardown cleans up
	mockOps.EXPECT().SetStreamActive(fakeStream, true).Return(activateErr)
	// teardown should call DisconnectStream, QuitMainLoop, DestroyStream, DestroyMainLoop
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().QuitMainLoop(fakeLoop).Return()
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	// Stop
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Restart should fail because SetStreamActive fails
	err := p.Start()
	if err == nil {
		t.Fatal("expected Start to fail on restart activation failure")
	}
	if !errors.Is(err, activateErr) {
		t.Fatalf("expected activateErr, got %v", err)
	}

	// Player should be in Error state
	if p.State() != PlayerStateError {
		t.Errorf("expected Error state after failed restart activation, got %v", p.State())
	}

	// Stale pointers must be cleared
	p.mu.Lock()
	loopPtr := p.loopPtr
	streamPtr := p.streamPtr
	p.mu.Unlock()

	if loopPtr != nil {
		t.Error("expected loopPtr to be nil after failed restart activation")
	}
	if streamPtr != nil {
		t.Error("expected streamPtr to be nil after failed restart activation")
	}
}

// TestPlayerRestartDoesNotStartSecondMainLoop verifies that when restarting
// after Stop (reusing existing resources), no second RunMainLoop call is
// made — only SetStreamActive(true) is called.
func TestPlayerRestartDoesNotStartSecondMainLoop(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := opaqueTestPtr()
	fakeStream := opaqueTestPtr()
	cfg := defaultTestConfig()

	// First start: full resource creation + RunMainLoop
	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)
	// Stop: deactivate
	mockOps.EXPECT().SetStreamActive(fakeStream, false).Return(nil)
	// Restart: only SetStreamActive(true) — NO CreateMainLoop, NO RunMainLoop
	mockOps.EXPECT().SetStreamActive(fakeStream, true).Return(nil)

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	// Start
	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	// Stop
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Restart — mock will fail if any extra calls are made
	if err := p.Start(); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}

	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state after restart, got %v", p.State())
	}
}
