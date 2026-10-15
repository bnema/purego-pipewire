package pipewire

// Loop is a PipeWire loop that the caller iterates itself, typically from a
// poll loop that waits on Fd. Create it with Runtime.NewLoop.
//
// A Loop and everything created on it (Context, Core, VideoSource) must only
// be used from one goroutine that stays on one OS thread: call
// runtime.LockOSThread before creating the loop and keep it until Destroy.
type Loop interface {
	// Fd returns a pollable file descriptor that becomes readable when
	// Iterate has work to do.
	Fd() int
	// Iterate dispatches pending events without blocking. Stream callbacks
	// (VideoHandler methods) run inside it. A transient failure such as
	// EINTR is ignored; the next Iterate retries.
	Iterate()
	// Destroy destroys the loop. Destroy contexts created on it first.
	Destroy()
}
