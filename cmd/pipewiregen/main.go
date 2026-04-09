package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/emitter"
	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/parser"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipewiregen: %v\n", err)
		os.Exit(1)
	}
	pipewireModel, err := parser.Load(filepath.Join(root, "gen", "pipewire.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipewiregen: %v\n", err)
		os.Exit(1)
	}
	if _, err := emitter.Emit(pipewireModel, root); err != nil {
		fmt.Fprintf(os.Stderr, "pipewiregen: %v\n", err)
		os.Exit(1)
	}
}
