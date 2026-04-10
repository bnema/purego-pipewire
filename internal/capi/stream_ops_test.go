package capi

import (
	"errors"
	"testing"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

// TestDestroyStreamCallsPwStreamDestroy verifies that DestroyStream delegates
// to the generated pw_stream_destroy binding (not pw_stream_disconnect).
func TestDestroyStreamCallsPwStreamDestroy(t *testing.T) {
	// Save and restore the package-level function var.
	origDestroy := pw_stream_destroy
	t.Cleanup(func() { pw_stream_destroy = origDestroy })

	var called bool
	var receivedPtr unsafe.Pointer
	pw_stream_destroy = func(stream unsafe.Pointer) {
		called = true
		receivedPtr = stream
	}

	ops := &streamOpsImpl{
		pinned: make(map[unsafe.Pointer]*streamCallbackStorage),
	}

	fakePtr := unsafe.Pointer(uintptr(0x1234))
	ops.pinned[fakePtr] = &streamCallbackStorage{}

	ops.DestroyStream(fakePtr)

	if !called {
		t.Fatal("expected pw_stream_destroy to be called")
	}
	if receivedPtr != fakePtr {
		t.Fatalf("pw_stream_destroy received %v, want %v", receivedPtr, fakePtr)
	}

	// Verify internal bookkeeping: pinned entry should be removed.
	if _, exists := ops.pinned[fakePtr]; exists {
		t.Error("expected callback storage to be unpinned after destroy")
	}
}

// TestDestroyStreamDoubleDestroySkipsSecondCall verifies idempotency:
// a second DestroyStream on the same pointer does NOT call pw_stream_destroy again.
func TestDestroyStreamDoubleDestroySkipsSecondCall(t *testing.T) {
	origDestroy := pw_stream_destroy
	t.Cleanup(func() { pw_stream_destroy = origDestroy })

	callCount := 0
	pw_stream_destroy = func(_ unsafe.Pointer) {
		callCount++
	}

	ops := &streamOpsImpl{
		pinned: make(map[unsafe.Pointer]*streamCallbackStorage),
	}

	fakePtr := unsafe.Pointer(uintptr(0xABCD))
	ops.pinned[fakePtr] = &streamCallbackStorage{}

	ops.DestroyStream(fakePtr)
	ops.DestroyStream(fakePtr)

	if callCount != 1 {
		t.Fatalf("pw_stream_destroy called %d times, want 1", callCount)
	}
}

// TestDestroyStreamBookkeepingDoesNotLeak verifies that internal tracking
// does not retain entries after cleanup across many create/destroy cycles.
func TestDestroyStreamBookkeepingDoesNotLeak(t *testing.T) {
	origDestroy := pw_stream_destroy
	t.Cleanup(func() { pw_stream_destroy = origDestroy })

	pw_stream_destroy = func(_ unsafe.Pointer) {}

	ops := &streamOpsImpl{
		pinned: make(map[unsafe.Pointer]*streamCallbackStorage),
	}

	const cycles = 100
	for i := uintptr(1); i <= cycles; i++ {
		ptr := unsafe.Pointer(i)
		ops.pinned[ptr] = &streamCallbackStorage{}
		ops.DestroyStream(ptr)
	}

	// After destroying all streams, pinned map should be empty.
	if len(ops.pinned) != 0 {
		t.Fatalf("pinned map has %d entries after %d cycles; expected 0", len(ops.pinned), cycles)
	}
}

// TestQuitMainLoopCallsPwMainLoopQuit verifies that QuitMainLoop delegates
// to the generated pw_main_loop_quit binding.
func TestQuitMainLoopCallsPwMainLoopQuit(t *testing.T) {
	origQuit := pw_main_loop_quit
	t.Cleanup(func() { pw_main_loop_quit = origQuit })

	var called bool
	var receivedPtr unsafe.Pointer
	pw_main_loop_quit = func(loop unsafe.Pointer) int32 {
		called = true
		receivedPtr = loop
		return 0
	}

	ops := &streamOpsImpl{}
	fakeLoop := unsafe.Pointer(uintptr(0x5678))

	ops.QuitMainLoop(fakeLoop)

	if !called {
		t.Fatal("expected pw_main_loop_quit to be called")
	}
	if receivedPtr != fakeLoop {
		t.Fatalf("pw_main_loop_quit received %v, want %v", receivedPtr, fakeLoop)
	}
}

// TestConnectPlaybackStreamReturnsTypedErrorOnNegativeReturn verifies that
// ConnectPlaybackStream returns a PWError with function name and return code.
func TestConnectPlaybackStreamReturnsTypedErrorOnNegativeReturn(t *testing.T) {
	origConnect := pw_stream_connect
	t.Cleanup(func() { pw_stream_connect = origConnect })

	pw_stream_connect = func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
		return -22 // EINVAL
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x1234))

	validFmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}
	err := ops.ConnectPlaybackStream(fakePtr, validFmt)
	if err == nil {
		t.Fatal("expected error for negative return code, got nil")
	}

	var pwErr *PWError
	if !errors.As(err, &pwErr) {
		t.Fatalf("expected *PWError, got %T", err)
	}
	if pwErr.Func != "pw_stream_connect" {
		t.Errorf("expected Func='pw_stream_connect', got '%s'", pwErr.Func)
	}
	if pwErr.Code != -22 {
		t.Errorf("expected Code=-22, got %d", pwErr.Code)
	}
}

// TestSetStreamActiveReturnsTypedErrorOnNegativeReturn verifies that
// SetStreamActive returns a PWError with function name and return code.
func TestSetStreamActiveReturnsTypedErrorOnNegativeReturn(t *testing.T) {
	origSetActive := pw_stream_set_active
	t.Cleanup(func() { pw_stream_set_active = origSetActive })

	pw_stream_set_active = func(stream unsafe.Pointer, active bool) int32 {
		return -9 // EBADF
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x5678))

	err := ops.SetStreamActive(fakePtr, true)
	if err == nil {
		t.Fatal("expected error for negative return code, got nil")
	}

	var pwErr *PWError
	if !errors.As(err, &pwErr) {
		t.Fatalf("expected *PWError, got %T", err)
	}
	if pwErr.Func != "pw_stream_set_active" {
		t.Errorf("expected Func='pw_stream_set_active', got '%s'", pwErr.Func)
	}
	if pwErr.Code != -9 {
		t.Errorf("expected Code=-9, got %d", pwErr.Code)
	}
}

// TestQueueBufferReturnsTypedErrorOnNegativeReturn verifies that
// QueueBuffer returns a PWError with function name and return code.
func TestQueueBufferReturnsTypedErrorOnNegativeReturn(t *testing.T) {
	origQueue := pw_stream_queue_buffer
	t.Cleanup(func() { pw_stream_queue_buffer = origQueue })

	pw_stream_queue_buffer = func(stream unsafe.Pointer, buffer unsafe.Pointer) int32 {
		return -5 // EIO
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x9ABC))
	fakeBuf := unsafe.Pointer(uintptr(0xDEF0))

	err := ops.QueueBuffer(fakePtr, fakeBuf)
	if err == nil {
		t.Fatal("expected error for negative return code, got nil")
	}

	var pwErr *PWError
	if !errors.As(err, &pwErr) {
		t.Fatalf("expected *PWError, got %T", err)
	}
	if pwErr.Func != "pw_stream_queue_buffer" {
		t.Errorf("expected Func='pw_stream_queue_buffer', got '%s'", pwErr.Func)
	}
	if pwErr.Code != -5 {
		t.Errorf("expected Code=-5, got %d", pwErr.Code)
	}
}

// TestDisconnectStreamReturnsTypedErrorOnNegativeReturn verifies that
// DisconnectStream returns a PWError with function name and return code.
func TestDisconnectStreamReturnsTypedErrorOnNegativeReturn(t *testing.T) {
	origDisconnect := pw_stream_disconnect
	t.Cleanup(func() { pw_stream_disconnect = origDisconnect })

	pw_stream_disconnect = func(stream unsafe.Pointer) int32 {
		return -19 // ENODEV
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x1111))

	err := ops.DisconnectStream(fakePtr)
	if err == nil {
		t.Fatal("expected error for negative return code, got nil")
	}

	var pwErr *PWError
	if !errors.As(err, &pwErr) {
		t.Fatalf("expected *PWError, got %T", err)
	}
	if pwErr.Func != "pw_stream_disconnect" {
		t.Errorf("expected Func='pw_stream_disconnect', got '%s'", pwErr.Func)
	}
	if pwErr.Code != -19 {
		t.Errorf("expected Code=-19, got %d", pwErr.Code)
	}
}

// TestDisconnectStreamReturnsNilOnSuccess verifies that DisconnectStream
// returns nil when the return code is non-negative.
func TestDisconnectStreamReturnsNilOnSuccess(t *testing.T) {
	origDisconnect := pw_stream_disconnect
	t.Cleanup(func() { pw_stream_disconnect = origDisconnect })

	pw_stream_disconnect = func(stream unsafe.Pointer) int32 {
		return 0 // Success
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x2222))

	err := ops.DisconnectStream(fakePtr)
	if err != nil {
		t.Fatalf("expected nil on success, got %v", err)
	}
}

// TestConnectPlaybackStreamRejectsMissingFormat verifies that ConnectPlaybackStream
// rejects a zero-valued PlaybackFormat at the StreamOps layer by returning
// ErrInvalidPlaybackFormat. This is a temporary guard; once SPA params are
// wired, the rejection will also cover missing/invalid SPA param construction.
func TestConnectPlaybackStreamRejectsMissingFormat(t *testing.T) {
	origConnect := pw_stream_connect
	t.Cleanup(func() { pw_stream_connect = origConnect })

	// Even if pw_stream_connect succeeds, a zero PlaybackFormat should be rejected
	// at the StreamOps layer before the C call is made.
	pw_stream_connect = func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
		return 0
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x1234))

	// Zero-valued PlaybackFormat must be rejected.
	zeroFmt := portout.PlaybackFormat{}
	err := ops.ConnectPlaybackStream(fakePtr, zeroFmt)
	if err == nil {
		t.Fatal("expected error for zero-valued PlaybackFormat, got nil")
	}
	if !errors.Is(err, ErrInvalidPlaybackFormat) {
		t.Fatalf("expected error wrapping ErrInvalidPlaybackFormat, got: %v", err)
	}
}

// TestConnectPlaybackStreamAcceptsValidFormat verifies that ConnectPlaybackStream
// accepts a valid PlaybackFormat and calls pw_stream_connect successfully.
// NOTE: The format is not yet forwarded to PipeWire as SPA params; this test
// only confirms that the format is accepted at the validation seam. A subsequent
// task will add tests verifying that the format values reach the C call.
func TestConnectPlaybackStreamAcceptsValidFormat(t *testing.T) {
	origConnect := pw_stream_connect
	t.Cleanup(func() { pw_stream_connect = origConnect })

	var called bool
	pw_stream_connect = func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
		called = true
		return 0
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x5678))

	validFmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}
	err := ops.ConnectPlaybackStream(fakePtr, validFmt)
	if err != nil {
		t.Fatalf("expected nil error for valid format, got %v", err)
	}
	if !called {
		t.Fatal("expected pw_stream_connect to be called")
	}
}

// TestRunMainLoopReturnsPWErrorOnNegativeReturn verifies that RunMainLoop
// returns a typed PWError when pw_main_loop_run returns a negative value.
func TestRunMainLoopReturnsPWErrorOnNegativeResult(t *testing.T) {
	origRun := pw_main_loop_run
	t.Cleanup(func() { pw_main_loop_run = origRun })

	pw_main_loop_run = func(loop unsafe.Pointer) int32 {
		return -22 // EINVAL
	}

	ops := &streamOpsImpl{}
	fakeLoop := unsafe.Pointer(uintptr(0xAAAA))

	err := ops.RunMainLoop(fakeLoop)
	if err == nil {
		t.Fatal("expected error for negative return code, got nil")
	}

	var pwErr *PWError
	if !errors.As(err, &pwErr) {
		t.Fatalf("expected *PWError, got %T", err)
	}
	if pwErr.Func != "pw_main_loop_run" {
		t.Errorf("expected Func='pw_main_loop_run', got '%s'", pwErr.Func)
	}
	if pwErr.Code != -22 {
		t.Errorf("expected Code=-22, got %d", pwErr.Code)
	}
}

// TestRunMainLoopReturnsNilOnSuccess verifies that RunMainLoop
// returns nil when pw_main_loop_run returns zero.
func TestRunMainLoopReturnsNilOnSuccess(t *testing.T) {
	origRun := pw_main_loop_run
	t.Cleanup(func() { pw_main_loop_run = origRun })

	pw_main_loop_run = func(loop unsafe.Pointer) int32 {
		return 0
	}

	ops := &streamOpsImpl{}
	fakeLoop := unsafe.Pointer(uintptr(0xBBBB))

	err := ops.RunMainLoop(fakeLoop)
	if err != nil {
		t.Fatalf("expected nil on success, got %v", err)
	}
}

// TestCreatePlaybackStreamUsesPWLoopFromMainLoop verifies that CreatePlaybackStream
// calls pw_main_loop_get_loop to obtain the pw_loop pointer and passes it to
// pw_stream_new_simple (not the raw main loop pointer).
func TestCreatePlaybackStreamUsesPWLoopFromMainLoop(t *testing.T) {
	origGetLoop := pw_main_loop_get_loop
	origNewSimple := pw_stream_new_simple
	t.Cleanup(func() {
		pw_main_loop_get_loop = origGetLoop
		pw_stream_new_simple = origNewSimple
	})

	fakeMainLoop := unsafe.Pointer(uintptr(0xAAAA))
	fakePWLoop := unsafe.Pointer(uintptr(0xBBBB))
	var gotContext unsafe.Pointer

	pw_main_loop_get_loop = func(loop unsafe.Pointer) unsafe.Pointer {
		if loop != fakeMainLoop {
			t.Errorf("pw_main_loop_get_loop received %v, want %v", loop, fakeMainLoop)
		}
		return fakePWLoop
	}

	pw_stream_new_simple = func(context unsafe.Pointer, name *byte, props unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer {
		gotContext = context
		// Return a non-nil fake stream pointer.
		return unsafe.Pointer(uintptr(0xCCCC))
	}

	ops := &streamOpsImpl{
		pinned: make(map[unsafe.Pointer]*streamCallbackStorage),
	}

	stream, err := ops.CreatePlaybackStream(fakeMainLoop, "test", func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream == nil {
		t.Fatal("expected non-nil stream pointer")
	}
	if gotContext != fakePWLoop {
		t.Errorf("pw_stream_new_simple received context %v, want %v (the pw_loop from pw_main_loop_get_loop)", gotContext, fakePWLoop)
	}
}

// TestCreatePlaybackStreamReturnsPWErrorWhenPWLoopIsNull verifies that
// CreatePlaybackStream returns a typed PWError when pw_main_loop_get_loop
// returns nil.
func TestCreatePlaybackStreamReturnsPWErrorWhenPWLoopIsNull(t *testing.T) {
	origGetLoop := pw_main_loop_get_loop
	origNewSimple := pw_stream_new_simple
	t.Cleanup(func() {
		pw_main_loop_get_loop = origGetLoop
		pw_stream_new_simple = origNewSimple
	})

	pw_main_loop_get_loop = func(loop unsafe.Pointer) unsafe.Pointer {
		return nil
	}

	// pw_stream_new_simple should NOT be called.
	pw_stream_new_simple = func(context unsafe.Pointer, name *byte, props unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer {
		t.Fatal("pw_stream_new_simple should not be called when pw_main_loop_get_loop returns nil")
		return nil
	}

	ops := &streamOpsImpl{
		pinned: make(map[unsafe.Pointer]*streamCallbackStorage),
	}

	fakeMainLoop := unsafe.Pointer(uintptr(0xAAAA))
	_, err := ops.CreatePlaybackStream(fakeMainLoop, "test", func() {})
	if err == nil {
		t.Fatal("expected error when pw_main_loop_get_loop returns nil")
	}

	var pwErr *PWError
	if !errors.As(err, &pwErr) {
		t.Fatalf("expected *PWError, got %T: %v", err, err)
	}
	if pwErr.Func != "pw_main_loop_get_loop" {
		t.Errorf("expected Func='pw_main_loop_get_loop', got '%s'", pwErr.Func)
	}
}

// TestConnectPlaybackStreamPassesParamsFromHelper verifies that ConnectPlaybackStream
// calls buildRawAudioParams and passes the resulting non-nil params and nParams > 0
// into pw_stream_connect.
func TestConnectPlaybackStreamPassesParamsFromHelper(t *testing.T) {
	origConnect := pw_stream_connect
	t.Cleanup(func() { pw_stream_connect = origConnect })

	var gotPorts unsafe.Pointer
	var gotNPorts uint32

	pw_stream_connect = func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
		gotPorts = ports
		gotNPorts = n_ports
		return 0
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x5678))

	validFmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}
	err := ops.ConnectPlaybackStream(fakePtr, validFmt)
	if err != nil {
		t.Fatalf("expected nil error for valid format, got %v", err)
	}

	if gotPorts == nil {
		t.Fatal("expected non-nil params pointer from buildRawAudioParams, got nil")
	}
	if gotNPorts == 0 {
		t.Fatal("expected nParams > 0 from buildRawAudioParams, got 0")
	}
}

// TestConnectPlaybackStreamPinsParamsAfterSuccess verifies that ConnectPlaybackStream
// stores the connectParams in pinnedParams so the SPA POD backing storage outlives
// the function call.
func TestConnectPlaybackStreamPinsParamsAfterSuccess(t *testing.T) {
	origConnect := pw_stream_connect
	t.Cleanup(func() { pw_stream_connect = origConnect })

	pw_stream_connect = func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
		return 0
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x5678))

	validFmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}
	err := ops.ConnectPlaybackStream(fakePtr, validFmt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp, ok := ops.pinnedParams[fakePtr]
	if !ok {
		t.Fatal("expected connectParams to be pinned for stream after ConnectPlaybackStream")
	}
	if cp == nil {
		t.Fatal("pinned connectParams should not be nil")
	}
	// Verify the pinned params point to real SPA POD data.
	if cp.Count() != 1 {
		t.Fatalf("expected pinned params Count()=1, got %d", cp.Count())
	}
	if cp.Pointer() == nil {
		t.Fatal("expected pinned params Pointer() to be non-nil")
	}
}

// TestConnectPlaybackStreamDoesNotPinParamsOnFailure verifies that when pw_stream_connect
// returns a negative result, connectParams are NOT pinned.
func TestConnectPlaybackStreamDoesNotPinParamsOnFailure(t *testing.T) {
	origConnect := pw_stream_connect
	t.Cleanup(func() { pw_stream_connect = origConnect })

	pw_stream_connect = func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
		return -22 // EINVAL
	}

	ops := &streamOpsImpl{}
	fakePtr := unsafe.Pointer(uintptr(0x5678))

	validFmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}
	err := ops.ConnectPlaybackStream(fakePtr, validFmt)
	if err == nil {
		t.Fatal("expected error for negative return code")
	}

	if _, ok := ops.pinnedParams[fakePtr]; ok {
		t.Fatal("connectParams should NOT be pinned when pw_stream_connect fails")
	}
}

// TestDestroyStreamReleasesPinnedParams verifies that DestroyStream removes both the
// callback storage and the connect params from the pinned maps.
func TestDestroyStreamReleasesPinnedParams(t *testing.T) {
	origConnect := pw_stream_connect
	origDestroy := pw_stream_destroy
	t.Cleanup(func() {
		pw_stream_connect = origConnect
		pw_stream_destroy = origDestroy
	})

	pw_stream_connect = func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
		return 0
	}
	pw_stream_destroy = func(_ unsafe.Pointer) {}

	ops := &streamOpsImpl{
		pinned: make(map[unsafe.Pointer]*streamCallbackStorage),
	}

	fakePtr := unsafe.Pointer(uintptr(0xDEAD))
	ops.pinned[fakePtr] = &streamCallbackStorage{}

	validFmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}
	err := ops.ConnectPlaybackStream(fakePtr, validFmt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify params are pinned.
	if _, ok := ops.pinnedParams[fakePtr]; !ok {
		t.Fatal("expected connectParams to be pinned before DestroyStream")
	}

	ops.DestroyStream(fakePtr)

	// Both maps should no longer contain the stream.
	if _, ok := ops.pinned[fakePtr]; ok {
		t.Error("expected callback storage to be removed after DestroyStream")
	}
	if _, ok := ops.pinnedParams[fakePtr]; ok {
		t.Error("expected connectParams to be removed after DestroyStream")
	}
}
