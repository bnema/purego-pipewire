package pipewire

// Context is the public interface for PipeWire context operations.
// Contexts manage resources and connections to the PipeWire server. Create
// one with Runtime.NewContext on a Loop; destroy it before the Loop.
type Context interface {
	Destroy()
	Connect() (Core, error)
}
