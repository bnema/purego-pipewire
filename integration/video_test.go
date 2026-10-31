//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"bnema.dev/purego-pipewire/internal/ports/in/mocks"
	"bnema.dev/purego-pipewire/pipewire"
	"bnema.dev/purego-pipewire/spa"
	"github.com/stretchr/testify/mock"
)

const (
	vidW = 320
	vidH = 240
)

// videoEvents records what the source reports. Callbacks run on the loop
// thread; the fields the test reads are atomics.
type videoEvents struct {
	state    atomic.Int32
	nodeID   atomic.Uint32
	format   atomic.Uint32
	width    atomic.Int32
	height   atomic.Int32
	stride   atomic.Int32
	buffers  atomic.Int32
	statesCh chan int32
}

// newVideoHandler returns a generated VideoHandler mock that records every
// callback into the returned events.
func newVideoHandler(t *testing.T) (*mocks.MockVideoHandler, *videoEvents) {
	ev := &videoEvents{statesCh: make(chan int32, 64)}
	h := mocks.NewMockVideoHandler(t)
	h.EXPECT().StateChanged(mock.Anything, mock.Anything, mock.Anything).Run(func(state int32, nodeID uint32, _ string) {
		ev.state.Store(state)
		if nodeID != 0 {
			ev.nodeID.Store(nodeID)
		}
		select {
		case ev.statesCh <- state:
		default:
		}
	}).Maybe()
	h.EXPECT().FormatChanged(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(f spa.VideoFormat, w, hh, stride int) {
		ev.format.Store(uint32(f))
		ev.width.Store(int32(w))
		ev.height.Store(int32(hh))
		ev.stride.Store(int32(stride))
	}).Maybe()
	h.EXPECT().BuffersChanged().Run(func() { ev.buffers.Add(1) }).Maybe()
	return h, ev
}

type videoEnv struct {
	rt      pipewire.Runtime
	loop    pipewire.Loop
	ctx     pipewire.Context
	core    pipewire.Core
	src     *pipewire.VideoSource
	handler *videoEvents
}

func (e *videoEnv) close() {
	if e.src != nil {
		e.src.Destroy()
	}
	if e.core != nil {
		e.core.Disconnect()
	}
	if e.ctx != nil {
		e.ctx.Destroy()
	}
	if e.loop != nil {
		e.loop.Destroy()
	}
	e.rt.Deinit()
}

// pollIterate waits up to timeout for the loop fd and iterates once.
func pollIterate(loop pipewire.Loop, timeout time.Duration) {
	waitReadable(loop.Fd(), timeout)
	loop.Iterate()
}

func skipUnlessPipeWire(t *testing.T) {
	t.Helper()
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		t.Skip("XDG_RUNTIME_DIR not set")
	}
	if _, err := os.Stat(filepath.Join(dir, "pipewire-0")); err != nil {
		t.Skipf("no PipeWire socket: %v", err)
	}
}

// newVideoEnv must be called from a goroutine locked to its OS thread.
func newVideoEnv(t *testing.T) *videoEnv {
	t.Helper()
	skipUnlessPipeWire(t)
	rt, err := pipewire.Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	h, ev := newVideoHandler(t)
	e := &videoEnv{rt: rt, handler: ev}
	if e.loop, err = rt.NewLoop(); err != nil {
		e.close()
		t.Fatalf("NewLoop: %v", err)
	}
	if e.ctx, err = rt.NewContext(e.loop); err != nil {
		e.close()
		t.Fatalf("NewContext: %v", err)
	}
	if e.core, err = e.ctx.Connect(); err != nil {
		e.close()
		t.Skipf("cannot connect to PipeWire: %v", err)
	}
	e.src, err = e.core.NewVideoSource(pipewire.VideoConfig{
		Name: "purego-pipewire-test", Width: vidW, Height: vidH, MaxFPS: 60,
		Formats: []pipewire.VideoFormat{pipewire.VideoFormatBGRx},
	}, h)
	if err != nil {
		e.close()
		t.Fatalf("NewVideoSource: %v", err)
	}
	return e
}

// waitNode iterates until the node id is known.
func (e *videoEnv) waitNode(t *testing.T) uint32 {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for e.handler.nodeID.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the node id")
		}
		pollIterate(e.loop, 20*time.Millisecond)
	}
	return e.handler.nodeID.Load()
}

// produce runs one Dequeue -> fill -> Queue -> Trigger cycle. It reports whether a
// buffer was available.
func (e *videoEnv) produce(seq uint64, fill byte) bool {
	b := e.src.Dequeue()
	if b == nil {
		return false
	}
	// Touch the first and last byte of the frame; filling everything would
	// dominate the allocation-free path being measured.
	b.Data[0] = fill
	b.Data[len(b.Data)-1] = fill
	e.src.Queue(b, seq, time.Duration(seq)*time.Second/60, false)
	e.src.Trigger()
	return true
}

// gstMediaType makes pipewiresrc declare media.type=Video. Without it the
// stream registers as Stream/Input/Unknown and WirePlumber refuses to link it
// to a Video node ("target not found").
const gstMediaType = "stream-properties=props,media.type=Video"

func startGst(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	path, err := exec.LookPath("gst-launch-1.0")
	if err != nil {
		t.Skip("gst-launch-1.0 not found")
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start gst-launch-1.0: %v", err)
	}
	return cmd
}

func TestVideoSource(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	skipUnlessPipeWire(t)
	if _, err := exec.LookPath("gst-launch-1.0"); err != nil {
		t.Skip("gst-launch-1.0 not found")
	}
	e := newVideoEnv(t)
	defer e.close()
	node := e.waitNode(t)

	cmd := startGst(t, "-q", "pipewiresrc", fmt.Sprintf("path=%d", node), gstMediaType, "num-buffers=120", "!",
		fmt.Sprintf("video/x-raw,format=BGRx,width=%d,height=%d", vidW, vidH), "!", "fakesink")
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() { _ = cmd.Process.Kill() }()

	var seq, produced uint64
	next := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("gst-launch-1.0 failed: %v", err)
			}
			if produced < 120 {
				t.Fatalf("gst finished after only %d produced frames", produced)
			}
			if got := e.handler.width.Load(); got != vidW || e.handler.height.Load() != vidH || e.handler.stride.Load() != vidW*4 {
				t.Fatalf("negotiated %dx%d stride %d", got, e.handler.height.Load(), e.handler.stride.Load())
			}
			if e.handler.format.Load() != uint32(pipewire.VideoFormatBGRx) {
				t.Fatalf("negotiated format %d", e.handler.format.Load())
			}
			t.Logf("gst received 120 frames; produced %d", produced)
			return
		case <-ctx.Done():
			t.Fatalf("timed out; produced %d frames", produced)
		default:
		}
		// 60 Hz tick from the loop's poll timeout.
		wait := time.Until(next)
		if wait < 0 {
			wait = 0
		}
		pollIterate(e.loop, wait)
		if !time.Now().Before(next) {
			next = next.Add(time.Second / 60)
			if e.handler.state.Load() == pipewire.StreamStateStreaming && e.produce(seq, byte(seq)) {
				seq++
				produced++
			}
		}
	}
}

func TestVideoSourceConsumerLeaves(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	skipUnlessPipeWire(t)
	if _, err := exec.LookPath("gst-launch-1.0"); err != nil {
		t.Skip("gst-launch-1.0 not found")
	}
	e := newVideoEnv(t)
	defer e.close()
	node := e.waitNode(t)

	cmd := startGst(t, "-q", "pipewiresrc", fmt.Sprintf("path=%d", node), gstMediaType, "!", "fakesink")
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	// Stream until the consumer has started streaming.
	deadline := time.Now().Add(10 * time.Second)
	var seq uint64
	for e.handler.state.Load() != pipewire.StreamStateStreaming {
		if time.Now().After(deadline) {
			t.Fatal("stream never reached STREAMING")
		}
		pollIterate(e.loop, 16*time.Millisecond)
	}
	var borrowed *pipewire.VideoBuffer
	for borrowed == nil {
		if time.Now().After(deadline) {
			t.Fatal("no buffer to borrow")
		}
		pollIterate(e.loop, 16*time.Millisecond)
		e.produce(seq, 1)
		seq++
		borrowed = e.src.Dequeue()
	}
	data := borrowed.Data
	buffersBefore := e.handler.buffers.Load()

	// Kill the consumer while holding a buffer.
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	deadline = time.Now().Add(10 * time.Second)
	for st := e.handler.state.Load(); st == pipewire.StreamStateStreaming; st = e.handler.state.Load() {
		if time.Now().After(deadline) {
			t.Fatal("stream stayed STREAMING after the consumer left")
		}
		pollIterate(e.loop, 20*time.Millisecond)
		e.src.Trigger()
	}
	if st := e.handler.state.Load(); st != pipewire.StreamStatePaused && st != pipewire.StreamStateUnconnected {
		t.Fatalf("state after consumer left = %d, want PAUSED or UNCONNECTED", st)
	}

	// The borrowed buffer must still be writable (no SIGBUS/SIGSEGV), even if
	// PipeWire removed it meanwhile.
	data[0] = 0xEE
	data[len(data)-1] = 0xEE
	if !isMapped(&data[0]) {
		t.Fatal("borrowed buffer was unmapped before Release")
	}

	// Give the buffer back. If PipeWire removed it while borrowed, Release must
	// unmap it now; otherwise it goes back to PipeWire and stays mapped.
	removed := e.handler.buffers.Load() != buffersBefore
	e.src.Release(borrowed)
	if removed && isMapped(&data[0]) {
		t.Fatal("removed buffer still mapped after Release")
	}
	t.Logf("consumer left: removed while borrowed = %v", removed)
	e.src.Release(borrowed) // a second Release is ignored
}

func TestVideoSourceFrameAllocations(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	skipUnlessPipeWire(t)
	if _, err := exec.LookPath("gst-launch-1.0"); err != nil {
		t.Skip("gst-launch-1.0 not found")
	}
	if raceEnabled {
		t.Skip("allocation counts are not meaningful under -race")
	}
	e := newVideoEnv(t)
	defer e.close()
	node := e.waitNode(t)

	cmd := startGst(t, "-q", "pipewiresrc", fmt.Sprintf("path=%d", node), gstMediaType, "!", "fakesink", "sync=false")
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	deadline := time.Now().Add(15 * time.Second)
	for e.handler.state.Load() != pipewire.StreamStateStreaming {
		if time.Now().After(deadline) {
			t.Fatal("stream never reached STREAMING")
		}
		pollIterate(e.loop, 16*time.Millisecond)
	}
	// Warm up so every buffer and every lazily initialised path has been used.
	var seq uint64
	for i := 0; i < 60; i++ {
		pollIterate(e.loop, 16*time.Millisecond)
		if e.produce(seq, 1) {
			seq++
		}
	}
	if seq == 0 {
		t.Fatal("no frame could be produced during warm-up")
	}

	// Each run waits (without allocating) for the loop fd, so that the consumer
	// has returned a buffer and most runs queue and trigger a real frame.
	loopFd := e.loop.Fd() // read once, like a real poll loop does
	var produced int
	allocs := testing.AllocsPerRun(200, func() {
		waitReadable(loopFd, 17*time.Millisecond)
		e.loop.Iterate()
		if e.produce(seq, byte(seq)) {
			seq++
			produced++
		}
	})
	t.Logf("allocs per Dequeue/fill/Queue/Trigger/Iterate cycle: %v (%d frames queued)", allocs, produced)
	if allocs != 0 {
		t.Fatalf("allocs per cycle = %v, want 0", allocs)
	}
	if produced < 50 {
		t.Fatalf("only %d frames were produced inside the measured closure, want >= 50", produced)
	}
}
