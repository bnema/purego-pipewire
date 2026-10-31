package core

import (
	"errors"
	"unsafe"

	portout "bnema.dev/purego-pipewire/internal/ports/out"
)

var ErrContextCreate = errors.New("failed to create context")

// Context wraps a pw_context. Create it with Runtime.NewContext and destroy
// it before the Loop it was created on.
type Context struct {
	ptr  unsafe.Pointer
	capi portout.VideoStreamAPI
	shm  portout.SharedMemAPI
}

// NewContext creates a pw_context that runs on loop. The loop must outlive
// the context.
func (r *Runtime) NewContext(loop *Loop) (*Context, error) {
	if loop == nil || loop.ptr == nil {
		return nil, ErrContextCreate
	}
	ptr := r.capi.PWContextNew(loop.ptr, nil, 0)
	if ptr == nil {
		return nil, ErrContextCreate
	}
	return &Context{ptr: ptr, capi: r.capi, shm: memfdShm{}}, nil
}

// Destroy destroys the context. All cores connected from it must have been
// disconnected first. Destroy is idempotent.
func (c *Context) Destroy() {
	if c.ptr == nil {
		return
	}
	c.capi.PWContextDestroy(c.ptr)
	c.ptr = nil
}

// Connect connects to the PipeWire daemon.
func (c *Context) Connect() (*Core, error) {
	if c.ptr == nil {
		return nil, ErrCoreConnect
	}
	ptr := c.capi.PWContextConnect(c.ptr, nil, 0)
	if ptr == nil {
		return nil, ErrCoreConnect
	}
	return &Core{ptr: ptr, capi: c.capi, shm: c.shm}, nil
}

func (c *Context) Ptr() unsafe.Pointer {
	return c.ptr
}
