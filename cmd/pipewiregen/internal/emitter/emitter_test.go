package emitter

import (
	"testing"

	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/model"
)

func testModel() *model.Model {
	return &model.Model{
		Libraries: []model.Library{
			{Name: "pipewire", SOName: "libpipewire-0.3.so.0"},
		},
		Groups: []model.Group{
			{Name: "init", Interface: "InitAPI", Package: "out", Symbols: []string{"pw_init", "pw_deinit"}},
		},
		Symbols: []model.Symbol{
			{Name: "pw_init", Library: "pipewire", Group: "init", Signature: "func(argc *int32, argv ***byte)"},
			{Name: "pw_deinit", Library: "pipewire", Group: "init", Signature: "func()"},
		},
	}
}

func TestEmitWritesCAPIAndPortFiles(t *testing.T) {
	paths, err := Emit(testModel(), t.TempDir())
	if err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}
	if _, ok := paths["internal/capi/init_gen.go"]; !ok {
		t.Fatalf("missing capi output")
	}
	if _, ok := paths["internal/ports/out/init_gen.go"]; !ok {
		t.Fatalf("missing port output")
	}
}
