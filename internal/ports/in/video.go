package in

import "bnema.dev/purego-pipewire/spa"

// VideoHandler receives video source stream events. All methods are called on
// the thread that drives the PipeWire loop (the goroutine that calls
// Loop.Iterate), from inside Iterate. Implementations must not block and
// should not allocate.
type VideoHandler interface {
	// StateChanged reports a stream state change. state is a pw_stream_state
	// value (-1 error, 0 unconnected, 1 connecting, 2 paused, 3 streaming).
	// nodeID is the PipeWire node id once known (0 before). err is the error
	// message when state is -1.
	StateChanged(state int32, nodeID uint32, err string)
	// FormatChanged reports the negotiated format.
	FormatChanged(f spa.VideoFormat, w, h, stride int)
	// BuffersChanged is called after a buffer was added or removed. Dequeue
	// may now return a buffer.
	BuffersChanged()
}
