package model

// Model represents the checked-in binding model for PipeWire/SPA code generation.
type Model struct {
	Libraries []Library `json:"libraries"`
	Groups    []Group   `json:"groups"`
	Symbols   []Symbol  `json:"symbols"`
}

// Library represents a shared library to load at runtime.
type Library struct {
	Name   string `json:"name"`
	SOName string `json:"soname"`
}

// Group represents a logical grouping of symbols.
type Group struct {
	Name    string   `json:"name"`
	Symbols []string `json:"symbols"`
}

// Symbol represents a single C function to bind.
type Symbol struct {
	Name      string `json:"name"`
	Library   string `json:"library"`
	Signature string `json:"signature"`
}
