package pipewire

// Runtime is the public interface for PipeWire runtime lifecycle management.
type Runtime interface {
	Init() error
	Deinit()
	NewMainLoop() (MainLoop, error)
	// NewLoop creates a loop the caller iterates itself. See Loop for the
	// threading rules.
	NewLoop() (Loop, error)
	// NewContext creates a context that runs on loop.
	NewContext(loop Loop) (Context, error)
}

// MainLoop is the public interface for PipeWire main loop operations.
type MainLoop interface {
	Run() int32
	Destroy()
}
