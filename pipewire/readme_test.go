package pipewire_test

import (
	"os"
	"strings"
	"testing"
)

func TestReadmeMentionsGenerateAndMockery(t *testing.T) {
	b, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	text := string(b)
	for _, needle := range []string{"go generate ./...", "mockery", "gen/pipewire.json"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("README missing %q", needle)
		}
	}
}
