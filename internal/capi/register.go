package capi

import (
	"unsafe"

	"github.com/bnema/purego-pipewire/internal/loader"
	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

// defaultImpl implements portout.CAPI by wrapping the registered functions.
type defaultImpl struct{}

func (d defaultImpl) PWInit(argc *int32, argv ***byte) {
	pw_init(argc, argv)
}

func (d defaultImpl) PWDeinit() {
	pw_deinit()
}

func (d defaultImpl) PWMainLoopNew(props unsafe.Pointer) unsafe.Pointer {
	return pw_main_loop_new(props)
}

func (d defaultImpl) PWMainLoopDestroy(loop unsafe.Pointer) {
	pw_main_loop_destroy(loop)
}

func (d defaultImpl) PWMainLoopRun(loop unsafe.Pointer) int32 {
	return pw_main_loop_run(loop)
}

var (
	defaultAPI portout.CAPI
	registered bool
)

// Register loads the PipeWire libraries and registers all function bindings.
func Register() error {
	if registered {
		return nil
	}
	h, err := loader.Open()
	if err != nil {
		return err
	}
	if err := registerAll(h); err != nil {
		return err
	}
	defaultAPI = defaultImpl{}
	registered = true
	return nil
}

// Default returns the default CAPI instance after Register() has been called.
// Returns nil if Register() has not been called successfully.
func Default() portout.CAPI {
	return defaultAPI
}
