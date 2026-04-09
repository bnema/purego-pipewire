package capi

import "github.com/bnema/purego-pipewire/internal/loader"

func registerAll(h loader.Handles) error {
	registerInit(h.PipeWire)
	registerLoop(h.PipeWire)
	return nil
}
