package in

// Runtime is the inbound interface for runtime operations.
// This package defines interfaces that internal/core implements
// and that are exposed to higher-level packages.
type Runtime interface {
	Init() error
	Deinit()
}
