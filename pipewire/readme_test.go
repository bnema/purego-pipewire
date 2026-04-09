package pipewire_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadmeMentionsGenerateAndMockery(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "README.md"))
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
