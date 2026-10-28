package core

import (
	"errors"
	"github.com/bnema/purego-pipewire/spa"
	"testing"
	"time"
	"unsafe"
)

type controlBlockingOps struct {
	callbackOps
	entered   chan struct{}
	release   chan struct{}
	destroyed chan struct{}
}

func (o *controlBlockingOps) SetStreamControl(unsafe.Pointer, ...spa.PropValue) error {
	close(o.entered)
	<-o.release
	return nil
}
func (o *controlBlockingOps) GetStreamControl(unsafe.Pointer, spa.Prop) (*spa.ControlInfo, error) {
	close(o.entered)
	<-o.release
	return &spa.ControlInfo{}, nil
}
func (o *controlBlockingOps) DisconnectStream(unsafe.Pointer) error { return nil }
func (o *controlBlockingOps) DestroyStream(unsafe.Pointer)          { close(o.destroyed) }

func TestCloseWaitsForControls(t *testing.T) {
	for _, get := range []bool{false, true} {
		t.Run(map[bool]string{false: "set", true: "get"}[get], func(t *testing.T) {
			ops := &controlBlockingOps{entered: make(chan struct{}), release: make(chan struct{}), destroyed: make(chan struct{})}
			closing := make(chan struct{})
			p := newPlayer(PlayerConfig{}, PlayerCallbacks{OnStateChange: func(state PlayerState) {
				if state == PlayerStateClosing {
					close(closing)
				}
			}})
			p.streamOps = ops
			p.streamPtr = unsafe.Pointer(new(byte))
			callDone := make(chan struct{})
			go func() {
				defer close(callDone)
				if get {
					_, _ = p.GetControl(0)
				} else {
					_ = p.SetControl(spa.PropVolume.Value(1))
				}
			}()
			<-ops.entered
			closed := make(chan error, 1)
			go func() { closed <- p.Close() }()
			select {
			case <-closing:
			case <-time.After(time.Second):
				t.Fatal("Close did not enter closing state")
			}
			select {
			case <-ops.destroyed:
				t.Fatal("destroyed during control call")
			case <-time.After(20 * time.Millisecond):
			}
			close(ops.release)
			<-callDone
			if err := <-closed; err != nil {
				t.Fatal(err)
			}
			select {
			case <-ops.destroyed:
			default:
				t.Fatal("stream not destroyed")
			}
			if err := p.SetControl(); !errors.Is(err, ErrPlayerClosed) {
				t.Fatalf("set after close: %v", err)
			}
			if _, err := p.GetControl(0); !errors.Is(err, ErrPlayerClosed) {
				t.Fatalf("get after close: %v", err)
			}
		})
	}
}
