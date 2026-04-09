package emitter

import (
	"strings"
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
