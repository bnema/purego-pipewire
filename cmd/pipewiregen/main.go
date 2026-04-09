package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/parser"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipewiregen: %v\n", err)
		os.Exit(1)
	}
	_, err = parser.Load(filepath.Join(root, "gen", "pipewire.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipewiregen: %v\n", err)
		os.Exit(1)
	}
	// Emit step will be added in Task 2.
}
