package pipewire

import (
	"unsafe"

	"github.com/bnema/purego-pipewire/internal/core"
)

// Stream is the public interface for PipeWire stream operations.
// Streams are used to transfer audio/video data to and from PipeWire.
type Stream interface {
	Destroy()
	Connect(direction int, id uint32, flags uint32, params unsafe.Pointer) error
	Disconnect()
}

// streamWrapper wraps the internal core.Stream to implement the public Stream interface.
type streamWrapper struct {
	inner *core.Stream
}

func (sw streamWrapper) Destroy() {
	sw.inner.Destroy()
}

func (sw streamWrapper) Connect(direction int, id uint32, flags uint32, params unsafe.Pointer) error {
	return sw.inner.Connect(direction, id, flags, params)
}

func (sw streamWrapper) Disconnect() {
	sw.inner.Disconnect()
}
