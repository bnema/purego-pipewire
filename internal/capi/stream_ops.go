package capi

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

var (
	errStreamCreate   = errors.New("pw_stream_new_simple returned nil")
	errStreamConnect  = errors.New("pw_stream_connect failed")
	errMainLoopCreate = errors.New("pw_main_loop_new returned nil")
)

// streamCallbackStorage keeps the pw_stream_events struct and its
// associated Go callback alive for the lifetime of the stream.
// Without this, the stack-allocated values in CreatePlaybackStream
// would be collected while PipeWire still holds pointers to them.
type streamCallbackStorage struct {
	processCb func(unsafe.Pointer)
	events    pw_stream_events
}

// streamOpsImpl implements portout.StreamOps using the real PipeWire C API
// bindings registered via purego.
type streamOpsImpl struct {
	mu        sync.Mutex
	pinned    map[unsafe.Pointer]*streamCallbackStorage // keyed by stream ptr
	destroyed map[unsafe.Pointer]bool                   // guard double-destroy
}

// Verify interface compliance at compile time.
var _ portout.StreamOps = (*streamOpsImpl)(nil)

func (s *streamOpsImpl) CreatePlaybackStream(loopPtr unsafe.Pointer, name string, sampleRate int, channels int, onProcess func()) (unsafe.Pointer, error) {
	nameBytes := append([]byte(name), 0)

	// Build persistent callback storage that outlives this function.
	storage := &streamCallbackStorage{}
	storage.processCb = func(_ unsafe.Pointer) {
		onProcess()
	}
	storage.events = pw_stream_events{
		version: 0, // PW_VERSION_STREAM_EVENTS
		process: &storage.processCb,
	}

	ptr := pw_stream_new_simple(
		loopPtr,
		&nameBytes[0],
		nil,                             // props
		unsafe.Pointer(&storage.events), // events
		nil,                             // data
	)
	if ptr == nil {
		return nil, errStreamCreate
	}

	// Pin the storage so GC cannot collect it while PipeWire holds references.
	s.mu.Lock()
	if s.pinned == nil {
		s.pinned = make(map[unsafe.Pointer]*streamCallbackStorage)
	}
	s.pinned[ptr] = storage
	s.mu.Unlock()

	return ptr, nil
}

func (s *streamOpsImpl) ConnectPlaybackStream(streamPtr unsafe.Pointer) error {
	// PW_DIRECTION_OUTPUT = 1, PW_ID_ANY = 0xffffffff
	// PW_STREAM_FLAG_AUTOCONNECT | PW_STREAM_FLAG_MAP_BUFFERS = 0x01 | 0x04
	ret := pw_stream_connect(streamPtr, 1, 0xffffffff, 0x05, nil, 0)
	if ret < 0 {
		return errStreamConnect
	}
	return nil
}

func (s *streamOpsImpl) SetStreamActive(streamPtr unsafe.Pointer, active bool) error {
	ret := pw_stream_set_active(streamPtr, active)
	if ret < 0 {
		return fmt.Errorf("pw_stream_set_active failed: %d", ret)
	}
	return nil
}

func (s *streamOpsImpl) DequeueBuffer(streamPtr unsafe.Pointer) unsafe.Pointer {
	return pw_stream_dequeue_buffer(streamPtr)
}

func (s *streamOpsImpl) QueueBuffer(streamPtr unsafe.Pointer, bufPtr unsafe.Pointer) error {
	ret := pw_stream_queue_buffer(streamPtr, bufPtr)
	if ret < 0 {
		return fmt.Errorf("pw_stream_queue_buffer failed: %d", ret)
	}
	return nil
}

func (s *streamOpsImpl) DisconnectStream(streamPtr unsafe.Pointer) {
	pw_stream_disconnect(streamPtr)
}

func (s *streamOpsImpl) DestroyStream(streamPtr unsafe.Pointer) {
	s.mu.Lock()
	if s.destroyed == nil {
		s.destroyed = make(map[unsafe.Pointer]bool)
	}
	if s.destroyed[streamPtr] {
		s.mu.Unlock()
		return
	}
	s.destroyed[streamPtr] = true
	// Unpin callback storage — safe now that the stream is being destroyed.
	delete(s.pinned, streamPtr)
	s.mu.Unlock()

	// Use the generated pw_stream_destroy binding for proper cleanup.
	pw_stream_destroy(streamPtr)
}

func (s *streamOpsImpl) CreateMainLoop() (unsafe.Pointer, error) {
	ptr := pw_main_loop_new(nil)
	if ptr == nil {
		return nil, errMainLoopCreate
	}
	return ptr, nil
}

func (s *streamOpsImpl) RunMainLoop(loopPtr unsafe.Pointer) {
	pw_main_loop_run(loopPtr)
}

func (s *streamOpsImpl) QuitMainLoop(loopPtr unsafe.Pointer) {
	// Use the generated pw_main_loop_quit binding to signal loop exit.
	pw_main_loop_quit(loopPtr)
}

func (s *streamOpsImpl) DestroyMainLoop(loopPtr unsafe.Pointer) {
	pw_main_loop_destroy(loopPtr)
}

// DefaultStreamOps returns a StreamOps implementation backed by the real
// PipeWire C API. Must be called after Register().
func DefaultStreamOps() portout.StreamOps {
	return &streamOpsImpl{}
}
