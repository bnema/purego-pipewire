package core

import (
	"errors"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

var ErrMainLoopCreate = errors.New("failed to create main loop")

type MainLoop struct {
	ptr  unsafe.Pointer
	capi portout.CAPI
}

func (r *Runtime) NewMainLoop() (*MainLoop, error) {
	ptr := r.capi.PWMainLoopNew(nil)
	if ptr == nil {
		return nil, ErrMainLoopCreate
	}
	return &MainLoop{ptr: ptr, capi: r.capi}, nil
}

func (ml *MainLoop) Run() int32 {
	return ml.capi.PWMainLoopRun(ml.ptr)
}

func (ml *MainLoop) Destroy() {
	ml.capi.PWMainLoopDestroy(ml.ptr)
}

func (ml *MainLoop) Ptr() unsafe.Pointer {
	return ml.ptr
}

var ErrLoopCreate = errors.New("failed to create loop")

// Loop wraps a pw_loop that the caller iterates itself, typically from a
// poll loop that waits on Fd. A Loop and everything created on it (Context,
// Core, streams) must only be used from one goroutine locked to one OS
// thread (runtime.LockOSThread); PipeWire loops are not thread safe.
type Loop struct {
	ptr  unsafe.Pointer
	capi portout.VideoStreamAPI
}

// NewLoop creates a pw_loop. The Runtime must be initialized.
func (r *Runtime) NewLoop() (*Loop, error) {
	ptr := r.capi.PWLoopNew(nil)
	if ptr == nil {
		return nil, ErrLoopCreate
	}
	return &Loop{ptr: ptr, capi: r.capi}, nil
}

// Fd returns the pollable file descriptor of the loop. It becomes readable
// when Iterate has work to do.
func (l *Loop) Fd() int {
	return int(l.capi.PWLoopGetFd(l.ptr))
}

// Iterate dispatches the pending events without blocking (pw_loop_enter,
// pw_loop_iterate with a zero timeout, pw_loop_leave). Stream callbacks run
// inside it. A negative result such as -EINTR is not fatal and is ignored:
// the next Iterate simply retries.
func (l *Loop) Iterate() {
	l.capi.PWLoopEnter(l.ptr)
	_ = l.capi.PWLoopIterate(l.ptr, 0)
	l.capi.PWLoopLeave(l.ptr)
}

// Destroy destroys the loop. Contexts created on it must be destroyed first.
// Destroy is idempotent.
func (l *Loop) Destroy() {
	if l.ptr == nil {
		return
	}
	l.capi.PWLoopDestroy(l.ptr)
	l.ptr = nil
}

func (l *Loop) Ptr() unsafe.Pointer {
	return l.ptr
}
