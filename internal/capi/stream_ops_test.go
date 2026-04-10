package capi

import (
	"testing"
	"unsafe"
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
		pinned:    make(map[unsafe.Pointer]*streamCallbackStorage),
		destroyed: make(map[unsafe.Pointer]bool),
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

	// Verify internal bookkeeping.
	if !ops.destroyed[fakePtr] {
		t.Error("expected stream to be marked as destroyed")
	}
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
		pinned:    make(map[unsafe.Pointer]*streamCallbackStorage),
		destroyed: make(map[unsafe.Pointer]bool),
	}

	fakePtr := unsafe.Pointer(uintptr(0xABCD))
	ops.pinned[fakePtr] = &streamCallbackStorage{}

	ops.DestroyStream(fakePtr)
	ops.DestroyStream(fakePtr)

	if callCount != 1 {
		t.Fatalf("pw_stream_destroy called %d times, want 1", callCount)
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
