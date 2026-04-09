package pipewire

import (
	"unsafe"

	"github.com/bnema/purego-pipewire/internal/core"
)

// Registry is the public interface for PipeWire registry operations.
// The registry is used to discover and bind to global objects.
type Registry interface {
	Destroy()
	Bind(name uint32, t string, version uint32) (unsafe.Pointer, error)
}

// registryWrapper wraps the internal core.Registry to implement the public Registry interface.
type registryWrapper struct {
	inner *core.Registry
}

func (rw registryWrapper) Destroy() {
	rw.inner.Destroy()
}

func (rw registryWrapper) Bind(name uint32, t string, version uint32) (unsafe.Pointer, error) {
	return rw.inner.Bind(name, t, version)
}
