package pipewire

import (
	"errors"
	"unsafe"

	"github.com/bnema/purego-pipewire/internal/capi"
	"github.com/bnema/purego-pipewire/internal/core"
)

// runtimeWrapper wraps the internal core.Runtime to implement the public Runtime interface.
type runtimeWrapper struct {
	inner *core.Runtime
}

func (rw runtimeWrapper) Init() error {
	return rw.inner.Init()
}

func (rw runtimeWrapper) Deinit() {
	rw.inner.Deinit()
}

func (rw runtimeWrapper) NewMainLoop() (MainLoop, error) {
	ml, err := rw.inner.NewMainLoop()
	if err != nil {
		return nil, err
	}
	return mainLoopWrapper{inner: ml}, nil
}

func (rw runtimeWrapper) NewLoop() (Loop, error) {
	l, err := rw.inner.NewLoop()
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (rw runtimeWrapper) NewContext(loop Loop) (Context, error) {
	l, ok := loop.(*core.Loop)
	if !ok {
		return nil, errors.New("pipewire: loop was not created by Runtime.NewLoop")
	}
	c, err := rw.inner.NewContext(l)
	if err != nil {
		return nil, err
	}
	return contextWrapper{inner: c}, nil
}

// contextWrapper wraps the internal core.Context to implement Context.
type contextWrapper struct {
	inner *core.Context
}

func (cw contextWrapper) Destroy() { cw.inner.Destroy() }

func (cw contextWrapper) Connect() (Core, error) {
	c, err := cw.inner.Connect()
	if err != nil {
		return nil, err
	}
	return coreWrapper{inner: c}, nil
}

// coreWrapper wraps the internal core.Core to implement Core.
type coreWrapper struct {
	inner *core.Core
}

func (cw coreWrapper) Disconnect() { cw.inner.Disconnect() }

func (cw coreWrapper) Registry() (Registry, error) {
	r, err := cw.inner.Registry()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (cw coreWrapper) NewStream(name string, props unsafe.Pointer) (Stream, error) {
	st, err := cw.inner.NewStream(name, props)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (cw coreWrapper) NewVideoSource(cfg VideoConfig, h VideoHandler) (*VideoSource, error) {
	if h == nil {
		return nil, core.ErrVideoConfig
	}
	return cw.inner.NewVideoSource(cfg, h)
}

// mainLoopWrapper wraps the internal core.MainLoop to implement the public MainLoop interface.
type mainLoopWrapper struct {
	inner *core.MainLoop
}

func (mlw mainLoopWrapper) Run() int32 {
	return mlw.inner.Run()
}

func (mlw mainLoopWrapper) Destroy() {
	mlw.inner.Destroy()
}

// Init initializes the PipeWire runtime and returns a Runtime instance.
// This function must be called before using any other PipeWire functionality.
// Returns an error if the PipeWire libraries cannot be loaded.
func Init() (Runtime, error) {
	if err := capi.Register(); err != nil {
		return nil, err
	}
	capiInstance := capi.Default()
	if capiInstance == nil {
		return nil, errors.New("failed to get default CAPI instance")
	}
	r := core.NewRuntime(capiInstance)
	if err := r.Init(); err != nil {
		return nil, err
	}
	return runtimeWrapper{inner: r}, nil
}
