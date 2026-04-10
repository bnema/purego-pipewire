package capi

import (
	"fmt"
	"sync"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

// PWError represents a PipeWire C API error with the function name and return code.
type PWError struct {
	Func string
	Code int32
}

func (e *PWError) Error() string {
	return fmt.Sprintf("%s failed: %d", e.Func, e.Code)
}

var (
	errStreamCreate   = &PWError{Func: "pw_stream_new_simple", Code: 0}
	errMainLoopCreate = &PWError{Func: "pw_main_loop_new", Code: 0}
)

const pwVersionStreamEvents = 2

const (
	pwDirectionOutput                 = 1
	pwIDAny                           = 0xffffffff
	pwStreamFlagAutoConnectMapBuffers = 0x05
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
	mu     sync.Mutex
	pinned map[unsafe.Pointer]*streamCallbackStorage // keyed by stream ptr
}

// Verify interface compliance at compile time.
var _ portout.StreamOps = (*streamOpsImpl)(nil)

func (s *streamOpsImpl) CreatePlaybackStream(loopPtr unsafe.Pointer, name string, onProcess func()) (unsafe.Pointer, error) {
	nameBytes := append([]byte(name), 0)

	// Build persistent callback storage that outlives this function.
	storage := &streamCallbackStorage{}
	storage.processCb = func(_ unsafe.Pointer) {
		onProcess()
	}
	storage.events = pw_stream_events{
		version: pwVersionStreamEvents,
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
	ret := pw_stream_connect(streamPtr, pwDirectionOutput, pwIDAny, pwStreamFlagAutoConnectMapBuffers, nil, 0)
	if ret < 0 {
		return &PWError{Func: "pw_stream_connect", Code: ret}
	}
	return nil
}

func (s *streamOpsImpl) SetStreamActive(streamPtr unsafe.Pointer, active bool) error {
	ret := pw_stream_set_active(streamPtr, active)
	if ret < 0 {
		return &PWError{Func: "pw_stream_set_active", Code: ret}
	}
	return nil
}

func (s *streamOpsImpl) DequeueBuffer(streamPtr unsafe.Pointer) unsafe.Pointer {
	return pw_stream_dequeue_buffer(streamPtr)
}

func (s *streamOpsImpl) QueueBuffer(streamPtr unsafe.Pointer, bufPtr unsafe.Pointer) error {
	ret := pw_stream_queue_buffer(streamPtr, bufPtr)
	if ret < 0 {
		return &PWError{Func: "pw_stream_queue_buffer", Code: ret}
	}
	return nil
}

// DisconnectStream disconnects the stream from its port.
// Returns a PWError if the operation fails.
func (s *streamOpsImpl) DisconnectStream(streamPtr unsafe.Pointer) error {
	ret := pw_stream_disconnect(streamPtr)
	if ret < 0 {
		return &PWError{Func: "pw_stream_disconnect", Code: ret}
	}
	return nil
}

func (s *streamOpsImpl) DestroyStream(streamPtr unsafe.Pointer) {
	s.mu.Lock()
	if _, alive := s.pinned[streamPtr]; !alive {
		// Already destroyed or never tracked — skip.
		s.mu.Unlock()
		return
	}
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
