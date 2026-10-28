package capi

import (
	"github.com/bnema/purego-pipewire/spa"
	"testing"
	"unsafe"
)

func TestControlValidation(t *testing.T) {
	ops := &streamOpsImpl{}
	for _, values := range [][]spa.PropValue{
		nil, {spa.PropVolume.Value(1), spa.PropVolume.Value(1), spa.PropVolume.Value(1), spa.PropVolume.Value(1), spa.PropVolume.Value(1)},
		{{ID: spa.Prop(spa.PropVolume)}},
		{{ID: 0x1000e, Values: []float32{1}}},
		{{ID: 0x10012, Values: []float32{1}}},
		{{ID: 0x10013, Values: []float32{1}}},
		{{ID: 0x10014, Values: []float32{1}}},
		{{ID: 0x10015, Values: []float32{1}}},
	} {
		if err := ops.SetStreamControl(nil, values...); err == nil {
			t.Fatalf("accepted %v", values)
		}
	}
}

func TestControlGetterCopiesUnderLock(t *testing.T) {
	oldGet, oldLock := pw_stream_get_control, withLoopLock
	t.Cleanup(func() { pw_stream_get_control, withLoopLock = oldGet, oldLock })
	stream, loop := opaquePtr(), opaquePtr()
	ops := &streamOpsImpl{streamLoops: map[unsafe.Pointer]streamLoopContext{stream: {mainLoop: loop, pwLoop: loop}}, runningLoops: map[unsafe.Pointer]bool{loop: true}}
	name, values := []byte("volume\x00"), []float32{0.5, 1}
	native := pwStreamControl{name: &name[0], values: &values[0], nValues: 2, def: 1, max: 2}
	locked := false
	pw_stream_get_control = func(unsafe.Pointer, uint32) unsafe.Pointer {
		if !locked {
			t.Fatal("getter outside lock")
		}
		return unsafe.Pointer(&native)
	}
	withLoopLock = func(_ unsafe.Pointer, fn func() int32) int32 {
		locked = true
		ret := fn()
		name[0] = 'X'
		values[0] = 9
		locked = false
		return ret
	}
	info, err := ops.GetStreamControl(stream, spa.Prop(spa.PropVolume))
	if err != nil || info.Name != "volume" || info.Values[0] != 0.5 || info.Max != 2 {
		t.Fatalf("copy: %+v, %v", info, err)
	}
	pw_stream_get_control = func(unsafe.Pointer, uint32) unsafe.Pointer { return nil }
	if _, err = ops.GetStreamControl(stream, 0); err == nil {
		t.Fatal("unknown control accepted")
	}
}

func TestFourControlsFitNativeCall(t *testing.T) {
	old := pw_stream_set_control
	t.Cleanup(func() { pw_stream_set_control = old })
	called := false
	pw_stream_set_control = func(_ unsafe.Pointer, id uint32, n uint32, value *float32, args ...any) int32 {
		called = true
		if id != uint32(spa.PropVolume) || n != 1 || *value != 1 || len(args) != 10 || args[9] != uint32(0) {
			t.Fatalf("bad native arguments: %v %v %v %v", id, n, *value, args)
		}
		return 0
	}
	v := spa.PropVolume.Value(1)
	if err := (&streamOpsImpl{}).SetStreamControl(nil, v, v, v, v); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("native setter not called")
	}
}
