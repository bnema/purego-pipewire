package pipewire

import (
	"github.com/bnema/purego-pipewire/internal/core"
)

// Context is the public interface for PipeWire context operations.
// Contexts manage resources and connections to the PipeWire server.
type Context interface {
	Destroy()
	Connect() (Core, error)
}

// contextWrapper wraps the internal core.Context to implement the public Context interface.
type contextWrapper struct {
	inner *core.Context
}

func (cw contextWrapper) Destroy() {
	cw.inner.Destroy()
}

func (cw contextWrapper) Connect() (Core, error) {
	c, err := cw.inner.Connect()
	if err != nil {
		return nil, err
	}
	return coreWrapper{inner: c}, nil
}
