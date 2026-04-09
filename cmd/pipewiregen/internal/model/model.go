package model

// Model represents the checked-in binding model for PipeWire/SPA code generation.
type Model struct {
	Libraries    []Library     `json:"libraries"`
	Groups       []Group       `json:"groups"`
	Symbols      []Symbol      `json:"symbols"`
	Callbacks    []Callback    `json:"callbacks,omitempty"`
	EventStructs []EventStruct `json:"event_structs,omitempty"`
}

// Library represents a shared library to load at runtime.
type Library struct {
	Name   string `json:"name"`
	SOName string `json:"soname"`
}

// Group represents a logical grouping of symbols.
type Group struct {
	Name      string   `json:"name"`
	Symbols   []string `json:"symbols"`
	Interface string   `json:"interface"`
	Package   string   `json:"package"`
}

// Symbol represents a single C function to bind.
type Symbol struct {
	Name      string `json:"name"`
	Library   string `json:"library"`
	Signature string `json:"signature"`
	Optional  bool   `json:"optional,omitempty"`
	Group     string `json:"group"`
}

// Callback represents a C callback typedef to generate.
type Callback struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
}

// EventStruct represents an event struct with its associated callbacks.
type EventStruct struct {
	Name      string   `json:"name"`
	Callbacks []string `json:"callbacks"`
}
