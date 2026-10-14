package pipewire

import (
	"unsafe"
)

// Core is the public interface for PipeWire core connection operations.
// A Core represents a connection to the PipeWire server.
type Core interface {
	// Disconnect disconnects from the server. Destroy streams first.
	Disconnect()
	Registry() (Registry, error)
	NewStream(name string, props unsafe.Pointer) (Stream, error)
	// NewVideoSource creates a Video/Source node fed by the caller. Call it
	// on the loop thread. See VideoSource.
	NewVideoSource(cfg VideoConfig, h VideoHandler) (*VideoSource, error)
}
