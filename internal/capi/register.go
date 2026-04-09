package capi

import "github.com/bnema/purego-pipewire/internal/loader"

func Register() error {
	h, err := loader.Open()
	if err != nil {
		return err
	}
	return registerAll(h)
}
