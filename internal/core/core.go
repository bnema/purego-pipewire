package core

import (
	"errors"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

var ErrCoreConnect = errors.New("failed to connect core")
var ErrRegistryUnavailable = errors.New("registry unavailable")

type Core struct {
	ptr      unsafe.Pointer
	capi     portout.CAPI
	context  *Context
	registry *Registry
}

func (c *Context) newCore(ptr unsafe.Pointer) *Core {
	return &Core{
		ptr:     ptr,
		capi:    c.capi,
		context: c,
	}
}

func (c *Core) Disconnect() {
	// Placeholder for core disconnection
}

func (c *Core) Registry() (*Registry, error) {
	if c.registry == nil {
		return nil, ErrRegistryUnavailable
	}
	return c.registry, nil
}

func (c *Core) Ptr() unsafe.Pointer {
	return c.ptr
}
