package pipewire

import (
	"unsafe"

	"github.com/bnema/purego-pipewire/internal/core"
)

// Core is the public interface for PipeWire core connection operations.
// A Core represents a connection to the PipeWire server.
type Core interface {
	Disconnect()
	Registry() (Registry, error)
	NewStream(name string, props unsafe.Pointer) (Stream, error)
}

// coreWrapper wraps the internal core.Core to implement the public Core interface.
type coreWrapper struct {
	inner *core.Core
}

func (cw coreWrapper) Disconnect() {
	cw.inner.Disconnect()
}

func (cw coreWrapper) Registry() (Registry, error) {
	r, err := cw.inner.Registry()
	if err != nil {
		return nil, err
	}
	return registryWrapper{inner: r}, nil
}

func (cw coreWrapper) NewStream(name string, props unsafe.Pointer) (Stream, error) {
	s, err := cw.inner.NewStream(name, props)
	if err != nil {
		return nil, err
	}
	return streamWrapper{inner: s}, nil
}
