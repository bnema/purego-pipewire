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
	capi     portout.VideoStreamAPI
	shm      portout.SharedMemAPI
	registry *Registry
}

// Disconnect disconnects from the daemon. Streams created from the core must
// have been destroyed first. Disconnect is idempotent.
func (c *Core) Disconnect() {
	if c.ptr == nil || c.capi == nil {
		return
	}
	c.capi.PWCoreDisconnect(c.ptr)
	c.ptr = nil
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
