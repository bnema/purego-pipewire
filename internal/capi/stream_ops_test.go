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
// rejects a zero-valued PlaybackFormat at the StreamOps layer. This is a temporary
// guard; once SPA params are wired, the rejection will also cover missing/invalid
// SPA param construction.
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
}

// TestConnectPlaybackStreamBuildsParamsFromFormat verifies that ConnectPlaybackStream
// accepts a valid PlaybackFormat and calls pw_stream_connect successfully.
// NOTE: The format is not yet forwarded to PipeWire as SPA params; this test
// only confirms that the format is accepted at the seam. A subsequent task
// will add tests verifying that the format values reach the C call.
func TestConnectPlaybackStreamBuildsParamsFromFormat(t *testing.T) {
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

// TestRunMainLoopReturnsErrorOnNegativeReturn verifies that RunMainLoop
// returns a PWError when pw_main_loop_run returns a negative value.
func TestRunMainLoopReturnsErrorOnNegativeReturn(t *testing.T) {
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
