package emitter

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/model"
	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/parser"
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

func testPlayerModel() *model.Model {
	return &model.Model{
		Libraries: []model.Library{
			{Name: "pipewire", SOName: "libpipewire-0.3.so.0"},
		},
		Groups: []model.Group{
			{
				Name:      "stream_playback",
				Interface: "StreamPlaybackAPI",
				Package:   "out",
				Symbols: []string{
					"pw_stream_new_simple",
					"pw_stream_connect",
					"pw_stream_set_active",
					"pw_stream_disconnect",
					"pw_stream_dequeue_buffer",
					"pw_stream_queue_buffer",
					"pw_stream_add_listener",
				},
			},
		},
		Symbols: []model.Symbol{
			{Name: "pw_stream_new_simple", Library: "pipewire", Group: "stream_playback", Signature: "func(context unsafe.Pointer, name *byte, props unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer"},
			{Name: "pw_stream_connect", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32"},
			{Name: "pw_stream_set_active", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, active bool) int32"},
			{Name: "pw_stream_disconnect", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer) int32"},
			{Name: "pw_stream_dequeue_buffer", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer) unsafe.Pointer"},
			{Name: "pw_stream_queue_buffer", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, buffer unsafe.Pointer) int32"},
			{Name: "pw_stream_add_listener", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, listener unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) int32"},
		},
		Callbacks: []model.Callback{
			{Name: "pw_stream_events", Signature: "struct{version uint32; process *func(stream unsafe.Pointer); param_changed *func(stream unsafe.Pointer, id uint32, param unsafe.Pointer); add_buffer *func(stream unsafe.Pointer, buffer unsafe.Pointer); remove_buffer *func(stream unsafe.Pointer, buffer unsafe.Pointer); flush *func(stream unsafe.Pointer, drain bool); drained *func(stream unsafe.Pointer); io_changed *func(stream unsafe.Pointer, id uint32, area unsafe.Pointer, size uint32)}"},
		},
		EventStructs: []model.EventStruct{
			{Name: "pw_stream_events", Callbacks: []string{"process", "param_changed", "add_buffer", "remove_buffer", "flush", "drained", "io_changed"}},
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

func TestEmitWritesPlayerStreamFiles(t *testing.T) {
	paths, err := Emit(testPlayerModel(), t.TempDir())
	if err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}
	if _, ok := paths["internal/capi/stream_playback_gen.go"]; !ok {
		t.Fatalf("missing stream_playback capi output")
	}
	if _, ok := paths["internal/ports/out/stream_playback_gen.go"]; !ok {
		t.Fatalf("missing stream_playback port output")
	}

	// Verify callback typedefs are in capi output
	capiContent := string(paths["internal/capi/stream_playback_gen.go"])
	if !strings.Contains(capiContent, "pw_stream_events") {
		t.Fatalf("capi output missing pw_stream_events callback typedef")
	}

	// Verify event struct callbacks are in port output
	portContent := string(paths["internal/ports/out/stream_playback_gen.go"])
	if !strings.Contains(portContent, "StreamPlaybackAPI") {
		t.Fatalf("port output missing StreamPlaybackAPI interface")
	}
}

// testCleanupModel returns a model with cleanup symbols that should be generated.
func testCleanupModel() *model.Model {
	return &model.Model{
		Libraries: []model.Library{
			{Name: "pipewire", SOName: "libpipewire-0.3.so.0"},
		},
		Groups: []model.Group{
			{
				Name:      "loop",
				Interface: "LoopAPI",
				Package:   "out",
				Symbols: []string{
					"pw_main_loop_new",
					"pw_main_loop_destroy",
					"pw_main_loop_run",
					"pw_main_loop_quit",
				},
			},
			{
				Name:      "stream_playback",
				Interface: "StreamPlaybackAPI",
				Package:   "out",
				Symbols: []string{
					"pw_stream_new_simple",
					"pw_stream_connect",
					"pw_stream_set_active",
					"pw_stream_disconnect",
					"pw_stream_dequeue_buffer",
					"pw_stream_queue_buffer",
					"pw_stream_add_listener",
					"pw_stream_destroy",
				},
			},
		},
		Symbols: []model.Symbol{
			// Loop symbols
			{Name: "pw_main_loop_new", Library: "pipewire", Group: "loop", Signature: "func(props unsafe.Pointer) unsafe.Pointer"},
			{Name: "pw_main_loop_destroy", Library: "pipewire", Group: "loop", Signature: "func(loop unsafe.Pointer)"},
			{Name: "pw_main_loop_run", Library: "pipewire", Group: "loop", Signature: "func(loop unsafe.Pointer) int32"},
			{Name: "pw_main_loop_quit", Library: "pipewire", Group: "loop", Signature: "func(loop unsafe.Pointer) int32"},
			// Stream playback symbols
			{Name: "pw_stream_new_simple", Library: "pipewire", Group: "stream_playback", Signature: "func(context unsafe.Pointer, name *byte, props unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer"},
			{Name: "pw_stream_connect", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, direction int32, id uint32, flags uint32, ports unsafe.Pointer, n_ports uint32) int32"},
			{Name: "pw_stream_set_active", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, active bool) int32"},
			{Name: "pw_stream_disconnect", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer) int32"},
			{Name: "pw_stream_dequeue_buffer", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer) unsafe.Pointer"},
			{Name: "pw_stream_queue_buffer", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, buffer unsafe.Pointer) int32"},
			{Name: "pw_stream_add_listener", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer, listener unsafe.Pointer, events unsafe.Pointer, data unsafe.Pointer) int32"},
			{Name: "pw_stream_destroy", Library: "pipewire", Group: "stream_playback", Signature: "func(stream unsafe.Pointer)"},
		},
	}
}

// TestEmitGeneratesCleanupBindings verifies that cleanup bindings are generated.
// This test ensures pw_main_loop_quit and pw_stream_destroy are present in generated files.
func TestEmitGeneratesCleanupBindings(t *testing.T) {
	t.Run("unit model", func(t *testing.T) {
		paths, err := Emit(testCleanupModel(), t.TempDir())
		if err != nil {
			t.Fatalf("Emit returned error: %v", err)
		}

		assertCleanupBindings(t, paths)
	})

	t.Run("checked in model", func(t *testing.T) {
		// Load the actual pipewire.json from the project root
		root, err := filepath.Abs("../../../..")
		if err != nil {
			t.Fatalf("failed to get project root: %v", err)
		}

		actualModel, err := parser.Load(filepath.Join(root, "gen", "pipewire.json"))
		if err != nil {
			t.Fatalf("failed to load pipewire.json: %v", err)
		}

		paths, err := Emit(actualModel, t.TempDir())
		if err != nil {
			t.Fatalf("Emit returned error: %v", err)
		}

		assertCleanupBindings(t, paths)
	})
}

func assertCleanupBindings(t *testing.T, paths map[string][]byte) {
	t.Helper()

	// Verify loop_gen.go contains pw_main_loop_quit
	loopCapiContent := string(paths["internal/capi/loop_gen.go"])
	if !strings.Contains(loopCapiContent, "pw_main_loop_quit") {
		t.Fatalf("capi loop output missing pw_main_loop_quit binding")
	}
	if !strings.Contains(loopCapiContent, "pw_main_loop_quitFunc") {
		t.Fatalf("capi loop output missing pw_main_loop_quitFunc type")
	}
	if !strings.Contains(loopCapiContent, "purego.RegisterLibFunc(&pw_main_loop_quit") {
		t.Fatalf("capi loop output missing pw_main_loop_quit registration")
	}

	// Verify loop port interface contains PWMainLoopQuit
	loopPortContent := string(paths["internal/ports/out/loop_gen.go"])
	if !strings.Contains(loopPortContent, "PWMainLoopQuit(loop unsafe.Pointer) int32") {
		t.Fatalf("port loop output missing PWMainLoopQuit method")
	}

	// Verify stream_playback_gen.go contains pw_stream_destroy
	streamCapiContent := string(paths["internal/capi/stream_playback_gen.go"])
	if !strings.Contains(streamCapiContent, "pw_stream_destroy") {
		t.Fatalf("capi stream_playback output missing pw_stream_destroy binding")
	}
	if !strings.Contains(streamCapiContent, "pw_stream_destroyFunc") {
		t.Fatalf("capi stream_playback output missing pw_stream_destroyFunc type")
	}
	if !strings.Contains(streamCapiContent, "purego.RegisterLibFunc(&pw_stream_destroy") {
		t.Fatalf("capi stream_playback output missing pw_stream_destroy registration")
	}

	// Verify stream_playback port interface contains PWStreamDestroy
	streamPortContent := string(paths["internal/ports/out/stream_playback_gen.go"])
	if !strings.Contains(streamPortContent, "PWStreamDestroy(stream unsafe.Pointer)") {
		t.Fatalf("port stream_playback output missing PWStreamDestroy method")
	}
}
