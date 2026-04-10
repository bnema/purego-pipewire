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
			{Name: "pw_stream_events", Signature: "struct{version uint32; destroy *func(stream unsafe.Pointer); state_changed *func(stream unsafe.Pointer, old uint32, state uint32, error *byte); control_info *func(stream unsafe.Pointer, id uint32, control unsafe.Pointer); io_changed *func(stream unsafe.Pointer, id uint32, area unsafe.Pointer, size uint32); param_changed *func(stream unsafe.Pointer, id uint32, param unsafe.Pointer); add_buffer *func(stream unsafe.Pointer, buffer unsafe.Pointer); remove_buffer *func(stream unsafe.Pointer, buffer unsafe.Pointer); process *func(stream unsafe.Pointer); drained *func(stream unsafe.Pointer); command *func(stream unsafe.Pointer, cmd unsafe.Pointer); trigger_done *func(stream unsafe.Pointer)}", Group: "stream_playback"},
		},
		EventStructs: []model.EventStruct{
			{Name: "pw_stream_events", Callbacks: []string{"destroy", "state_changed", "control_info", "io_changed", "param_changed", "add_buffer", "remove_buffer", "process", "drained", "command", "trigger_done"}, Group: "stream_playback"},
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

// TestPWStreamEventsABILayout verifies the generated pw_stream_events struct
// has fields in the exact order that matches the PipeWire C ABI.
// This catches layout drift that would silently corrupt callback dispatch.
func TestPWStreamEventsABILayout(t *testing.T) {
	paths, err := Emit(testPlayerModel(), t.TempDir())
	if err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}
	capiContent := string(paths["internal/capi/stream_playback_gen.go"])

	// The correct C ABI field order from <pipewire/stream.h>
	expectedFields := []string{
		"version uint32",
		"destroy *func(",
		"state_changed *func(",
		"control_info *func(",
		"io_changed *func(",
		"param_changed *func(",
		"add_buffer *func(",
		"remove_buffer *func(",
		"process *func(",
		"drained *func(",
		"command *func(",
		"trigger_done *func(",
	}

	// Verify all expected fields are present
	for _, field := range expectedFields {
		if !strings.Contains(capiContent, field) {
			t.Errorf("pw_stream_events missing expected field %q", field)
		}
	}

	// Verify the spurious "flush" field is NOT present
	if strings.Contains(capiContent, "flush *func(") {
		t.Error("pw_stream_events contains spurious 'flush' field — this does not exist in the PipeWire C ABI")
	}

	// Verify field order: each field must appear after the previous one
	lastIdx := 0
	for _, field := range expectedFields {
		idx := strings.Index(capiContent, field)
		if idx < 0 {
			continue // already reported above
		}
		if idx < lastIdx {
			t.Errorf("pw_stream_events field %q appears at wrong position (before previous field)", field)
		}
		lastIdx = idx
	}
}

func TestEmitWritesAdaptersAndCompositeFiles(t *testing.T) {
	paths, err := Emit(testPlayerModel(), t.TempDir())
	if err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}

	// Verify adapters file exists
	adapterContent, ok := paths["internal/capi/adapters_gen.go"]
	if !ok {
		t.Fatal("missing adapters_gen.go output")
	}

	// Verify adapter contains streamPlaybackCAPIAdapter with PWStreamConnect method
	adapterStr := string(adapterContent)
	if !strings.Contains(adapterStr, "StreamPlaybackCAPIAdapter") {
		t.Fatal("adapters output missing StreamPlaybackCAPIAdapter struct")
	}
	if !strings.Contains(adapterStr, "PWStreamConnect") {
		t.Fatal("adapters output missing PWStreamConnect method")
	}

	// Verify adapter forwarding calls use argument names only (no types).
	// For pw_stream_connect with signature "func(stream unsafe.Pointer, direction int32, ...)",
	// the forwarding call must be "pw_stream_connect(stream, direction, id, flags, ports, n_ports)"
	// NOT "pw_stream_connect(stream unsafe.Pointer, direction int32, ...)"
	if strings.Contains(adapterStr, "pw_stream_connect(stream unsafe.Pointer") {
		t.Fatal("adapter forwarding call contains type annotations — call site should use only argument names")
	}
	// Verify correct call syntax: forwarding with just names
	if !strings.Contains(adapterStr, "pw_stream_connect(stream, direction, id, flags, ports, n_ports)") {
		t.Fatal("adapter forwarding call missing correct call-site syntax pw_stream_connect(stream, direction, id, flags, ports, n_ports)")
	}

	// Verify composite port file exists
	compositeContent, ok := paths["internal/ports/out/capi_gen.go"]
	if !ok {
		t.Fatal("missing capi_gen.go composite port output")
	}

	// Verify composite interface embeds StreamPlaybackAPI
	compositeStr := string(compositeContent)
	if !strings.Contains(compositeStr, "CAPI") {
		t.Fatal("composite output missing CAPI type")
	}
	if !strings.Contains(compositeStr, "StreamPlaybackAPI") {
		t.Fatal("composite output missing StreamPlaybackAPI embed")
	}
	// Verify composite does not contain unused unsafe import
	if strings.Contains(compositeStr, `import "unsafe"`) {
		t.Fatal("composite output must not contain unused unsafe import — composite only embeds interfaces")
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
					"pw_main_loop_get_loop",
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
			{Name: "pw_main_loop_get_loop", Library: "pipewire", Group: "loop", Signature: "func(loop unsafe.Pointer) unsafe.Pointer"},
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

		// Verify composite port embeds StreamPlaybackAPI
		compositeContent := string(paths["internal/ports/out/capi_gen.go"])
		if !strings.Contains(compositeContent, "StreamPlaybackAPI") {
			t.Fatal("checked-in model: composite output missing StreamPlaybackAPI embed")
		}

		// Verify loop port contains PWMainLoopGetLoop
		loopPortContent := string(paths["internal/ports/out/loop_gen.go"])
		if !strings.Contains(loopPortContent, "PWMainLoopGetLoop(loop unsafe.Pointer) unsafe.Pointer") {
			t.Fatal("checked-in model: loop port output missing PWMainLoopGetLoop method")
		}
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

	// Verify loop_gen.go contains pw_main_loop_get_loop
	if !strings.Contains(loopCapiContent, "pw_main_loop_get_loop") {
		t.Fatalf("capi loop output missing pw_main_loop_get_loop binding")
	}
	if !strings.Contains(loopCapiContent, "pw_main_loop_get_loopFunc") {
		t.Fatalf("capi loop output missing pw_main_loop_get_loopFunc type")
	}

	// Verify loop port interface contains PWMainLoopQuit
	loopPortContent := string(paths["internal/ports/out/loop_gen.go"])
	if !strings.Contains(loopPortContent, "PWMainLoopQuit(loop unsafe.Pointer) int32") {
		t.Fatalf("port loop output missing PWMainLoopQuit method")
	}

	// Verify loop port interface contains PWMainLoopGetLoop
	if !strings.Contains(loopPortContent, "PWMainLoopGetLoop(loop unsafe.Pointer) unsafe.Pointer") {
		t.Fatalf("port loop output missing PWMainLoopGetLoop method")
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

func TestExtractParamNames(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty parameter list",
			input:  "()",
			expect: "()",
		},
		{
			name:   "single parameter",
			input:  "(stream unsafe.Pointer)",
			expect: "(stream)",
		},
		{
			name:   "multiple parameters",
			input:  "(stream unsafe.Pointer, direction int32, id uint32)",
			expect: "(stream, direction, id)",
		},
		{
			name:   "nested function-type parameter with inner commas",
			input:  "(callback func(int, int), count int32)",
			expect: "(callback, count)",
		},
		{
			name:   "nested function-type with multiple inner params",
			input:  "(handler func(a int32, b int32, c int32), ctx unsafe.Pointer)",
			expect: "(handler, ctx)",
		},
		{
			name:   "pointer type with multiple stars",
			input:  "(argv ***byte)",
			expect: "(argv)",
		},
		{
			name:   "no parameters bare parens trimmed to empty",
			input:  "()",
			expect: "()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractParamNames(tt.input)
			if got != tt.expect {
				t.Errorf("extractParamNames(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
