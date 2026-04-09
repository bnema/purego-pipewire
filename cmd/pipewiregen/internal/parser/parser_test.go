package parser

import "testing"

func TestLoadParsesLibraryAndSymbolGroups(t *testing.T) {
	model, err := Load("testdata/minimal_pipewire.json")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got, want := len(model.Libraries), 1; got != want {
		t.Fatalf("libraries=%d want %d", got, want)
	}
	if got, want := model.Groups[0].Name, "init"; got != want {
		t.Fatalf("group=%q want %q", got, want)
	}
}
