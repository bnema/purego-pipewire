package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
)

type Handles struct {
	PipeWire uintptr
	SPA      uintptr
}

func Open() (Handles, error) {
	pw, err := openOne(resolveLib("PIPEWIRE_LIB_PATH", "libpipewire-0.3.so.0"))
	if err != nil {
		return Handles{}, err
	}
	spa, err := openOne(resolveLib("SPA_LIB_PATH", "libspa-0.2.so.0"))
	if err != nil {
		return Handles{}, err
	}
	return Handles{PipeWire: pw, SPA: spa}, nil
}

func openOne(path string) (uintptr, error) {
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("dlopen %s: %w", path, err)
	}
	return h, nil
}

func resolveLib(envName, fallback string) string {
	if v := os.Getenv(envName); v != "" {
		return v
	}
	return fallback
}

func libFileName(base, soversion string) string {
	return filepath.Base(base) + ".so." + soversion
}
