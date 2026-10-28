package core

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
	outmocks "github.com/bnema/purego-pipewire/internal/ports/out/mocks"
	"github.com/bnema/purego-pipewire/spa"
)

func TestCloseWaitsForControls(t *testing.T) {
	for _, get := range []bool{false, true} {
		t.Run(map[bool]string{false: "set", true: "get"}[get], func(t *testing.T) {
			ops := outmocks.NewMockStreamOps(t)
			entered, release, closing, callDone := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var returned atomic.Bool
			p := newPlayer(PlayerConfig{}, PlayerCallbacks{OnStateChange: func(s PlayerState) {
				if s == PlayerStateClosing {
					close(closing)
				}
			}})
			ptr := unsafe.Pointer(new(byte))
			p.streamOps = ops
			p.streamPtr = ptr
			block := func() { close(entered); <-release; returned.Store(true) }
			if get {
				ops.EXPECT().GetStreamControl(ptr, spa.Prop(1)).RunAndReturn(func(unsafe.Pointer, spa.Prop) (*spa.ControlInfo, error) { block(); return &spa.ControlInfo{}, nil }).Once()
			} else {
				ops.EXPECT().SetStreamControl(ptr, spa.PropVolume.Value(1)).RunAndReturn(func(unsafe.Pointer, ...spa.PropValue) error { block(); return nil }).Once()
			}
			ops.EXPECT().DisconnectStream(ptr).RunAndReturn(func(unsafe.Pointer) error {
				if !returned.Load() {
					t.Error("disconnect before control returned")
				}
				return nil
			}).Once()
			ops.EXPECT().DestroyStream(ptr).Once()
			closed := make(chan error, 1)
			closeDone := make(chan struct{})
			go func() {
				defer close(callDone)
				if get {
					_, _ = p.GetControl(1)
				} else {
					_ = p.SetControl(spa.PropVolume.Value(1))
				}
			}()
			<-entered
			go func() { defer close(closeDone); closed <- p.Close() }()
			t.Cleanup(func() { unblock(); <-callDone; <-closeDone })
			<-closing
			// The in-flight call still owns its lifetime reference, without timing inference.
			if p.controlMu.TryLock() {
				p.controlMu.Unlock()
				t.Error("control released lifetime before returning")
			}
			if err := p.SetControl(); !errors.Is(err, ErrPlayerClosed) {
				t.Errorf("closing set: %v", err)
			}
			if _, err := p.GetControl(1); !errors.Is(err, ErrPlayerClosed) {
				t.Errorf("closing get: %v", err)
			}
			unblock()
			<-callDone
			if err := <-closed; err != nil {
				t.Fatal(err)
			}
			if err := p.SetControl(); !errors.Is(err, ErrPlayerClosed) {
				t.Errorf("closed set: %v", err)
			}
			if _, err := p.GetControl(1); !errors.Is(err, ErrPlayerClosed) {
				t.Errorf("closed get: %v", err)
			}
		})
	}
}

func TestControlCallbackDuringClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ops := outmocks.NewMockStreamOps(t)
		p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
		stream, loop := unsafe.Pointer(new(byte)), unsafe.Pointer(new(byte))
		loopDone := make(chan struct{})
		callback := make(chan struct{})
		p.streamOps = ops
		p.streamPtr = stream
		p.loopPtr = loop
		p.loopDone = loopDone
		ops.EXPECT().DisconnectStream(stream).RunAndReturn(func(unsafe.Pointer) error {
			// Simulate a loop callback needed by the native disconnect.
			go func() {
				defer close(callback)
				if err := p.SetControl(spa.PropVolume.Value(1)); !errors.Is(err, ErrPlayerClosed) {
					t.Errorf("callback set: %v", err)
				}
				if _, err := p.GetControl(1); !errors.Is(err, ErrPlayerClosed) {
					t.Errorf("callback get: %v", err)
				}
			}()
			<-callback
			return nil
		}).Once()
		ops.EXPECT().QuitMainLoop(loop).Run(func(unsafe.Pointer) { close(loopDone) }).Once()
		ops.EXPECT().DestroyStream(stream).Once()
		ops.EXPECT().DestroyMainLoop(loop).Once()
		done := make(chan error, 1)
		go func() { done <- p.Close() }()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestControlDoesNotWaitForLifetimeWriter(t *testing.T) {
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	p.controlMu.Lock()
	defer p.controlMu.Unlock()
	if err := p.SetControl(); !errors.Is(err, ErrPlayerClosed) {
		t.Fatal(err)
	}
	if _, err := p.GetControl(1); !errors.Is(err, ErrPlayerClosed) {
		t.Fatal(err)
	}
}

func TestFirstStartConcurrentControls(t *testing.T) {
	oldRegister, oldDefault, oldOps, oldInit := registerPipeWire, defaultPipeWireCAPI, defaultPipeWireOps, initializePipeWire
	t.Cleanup(func() {
		registerPipeWire, defaultPipeWireCAPI, defaultPipeWireOps, initializePipeWire = oldRegister, oldDefault, oldOps, oldInit
	})
	ops := outmocks.NewMockStreamOps(t)
	api := outmocks.NewMockCAPI(t)
	registerPipeWire = func() error { return nil }
	defaultPipeWireCAPI = func() portout.CAPI { return api }
	initializePipeWire = func(portout.CAPI) error { return nil }
	publish := make(chan struct{})
	release := make(chan struct{})
	defaultPipeWireOps = func() portout.StreamOps { close(publish); <-release; return ops }
	// Fail resource creation after exercising the initial runtime publication.
	ops.EXPECT().CreateMainLoop().Return(nil, errors.New("test creation failure")).Once()
	p := newPlayer(PlayerConfig{}, PlayerCallbacks{})
	done := make(chan error, 1)
	go func() { done <- p.Start() }()
	<-publish
	controlsDone := make(chan struct{})
	go func() {
		defer close(controlsDone)
		for range 1000 {
			_ = p.SetControl(spa.PropVolume.Value(1))
			_, _ = p.GetControl(1)
		}
	}()
	close(release)
	<-controlsDone
	if err := <-done; err == nil {
		t.Fatal("expected resource failure")
	}
}
