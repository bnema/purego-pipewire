package capi

import (
	"sync"
	"unsafe"

	"github.com/bnema/purego-pipewire/internal/loader"
	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

// defaultImpl implements portout.CAPI by wrapping the registered functions.
type defaultImpl struct{}

func (d defaultImpl) PWInit(argc *int32, argv ***byte) {
	pw_init(argc, argv)
}

func (d defaultImpl) PWDeinit() {
	pw_deinit()
}

func (d defaultImpl) PWMainLoopNew(props unsafe.Pointer) unsafe.Pointer {
	return pw_main_loop_new(props)
}

func (d defaultImpl) PWMainLoopDestroy(loop unsafe.Pointer) {
	pw_main_loop_destroy(loop)
}

func (d defaultImpl) PWMainLoopRun(loop unsafe.Pointer) int32 {
	return pw_main_loop_run(loop)
}

func (d defaultImpl) PWMainLoopQuit(loop unsafe.Pointer) int32 {
	return pw_main_loop_quit(loop)
}

func (d defaultImpl) PWStreamNewSimple(context unsafe.Pointer, name *byte, props unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer {
	return pw_stream_new_simple(context, name, props, events, data)
}

func (d defaultImpl) PWStreamConnect(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32 {
	return pw_stream_connect(stream, direction, id, flags, ports, n_ports)
}

func (d defaultImpl) PWStreamSetActive(stream unsafe.Pointer, active bool) int32 {
	return pw_stream_set_active(stream, active)
}

func (d defaultImpl) PWStreamDisconnect(stream unsafe.Pointer) int32 {
	return pw_stream_disconnect(stream)
}

func (d defaultImpl) PWStreamDequeueBuffer(stream unsafe.Pointer) unsafe.Pointer {
	return pw_stream_dequeue_buffer(stream)
}

func (d defaultImpl) PWStreamQueueBuffer(stream unsafe.Pointer, buffer unsafe.Pointer) int32 {
	return pw_stream_queue_buffer(stream, buffer)
}

func (d defaultImpl) PWStreamAddListener(stream unsafe.Pointer, listener unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) int32 {
	return pw_stream_add_listener(stream, listener, events, data)
}

func (d defaultImpl) PWStreamDestroy(stream unsafe.Pointer) {
	pw_stream_destroy(stream)
}

var (
	defaultAPI   portout.CAPI
	registerOnce sync.Once
	registerErr  error
)

// Register loads the PipeWire libraries and registers all function bindings.
// It is safe to call from multiple goroutines; the initialization will only happen once.
func Register() error {
	registerOnce.Do(func() {
		h, err := loader.Open()
		if err != nil {
			registerErr = err
			return
		}
		if err := registerAll(h); err != nil {
			registerErr = err
			return
		}
		defaultAPI = defaultImpl{}
	})
	return registerErr
}

// Default returns the default CAPI instance after Register() has been called.
// Returns nil if Register() has not been called successfully.
func Default() portout.CAPI {
	return defaultAPI
}
