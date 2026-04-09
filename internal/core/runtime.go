package core

import (
	"sync"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

type Runtime struct {
	capi    portout.CAPI
	once    sync.Once
	initErr error
}

func NewRuntime(capi portout.CAPI) *Runtime {
	return &Runtime{capi: capi}
}

func (r *Runtime) Init() error {
	r.once.Do(func() {
		r.capi.PWInit(nil, nil)
	})
	return r.initErr
}

func (r *Runtime) Deinit() {
	r.capi.PWDeinit()
}
