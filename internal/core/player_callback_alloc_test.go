package core

import (
	"errors"
	"testing"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

// callbackOps is intentionally allocation-free so callback allocation tests
// measure player code rather than a mocking framework.
type callbackOps struct {
	buffers   []unsafe.Pointer
	dequeueAt int
	queueErr  error
	queued    int
}

func (o *callbackOps) CreatePlaybackStream(unsafe.Pointer, string, func()) (unsafe.Pointer, error) {
	return nil, errors.New("not implemented")
}
func (o *callbackOps) ConnectPlaybackStream(unsafe.Pointer, portout.PlaybackFormat) error {
	return errors.New("not implemented")
}
func (o *callbackOps) SetStreamActive(unsafe.Pointer, bool) error {
	return errors.New("not implemented")
}
func (o *callbackOps) DequeueBuffer(unsafe.Pointer) unsafe.Pointer {
	if o.dequeueAt >= len(o.buffers) {
		return nil
	}
	buf := o.buffers[o.dequeueAt]
	o.dequeueAt++
	return buf
}
func (o *callbackOps) QueueBuffer(unsafe.Pointer, unsafe.Pointer) error {
	o.queued++
	return o.queueErr
}
func (o *callbackOps) DisconnectStream(unsafe.Pointer) error { return nil }
func (o *callbackOps) DestroyStream(unsafe.Pointer)          {}
func (o *callbackOps) CreateMainLoop() (unsafe.Pointer, error) {
	return nil, errors.New("not implemented")
}
func (o *callbackOps) RunMainLoop(unsafe.Pointer) error { return nil }
func (o *callbackOps) QuitMainLoop(unsafe.Pointer)      {}
func (o *callbackOps) DestroyMainLoop(unsafe.Pointer)   {}

func callbackBuffer(channels, frames, capacity int) (unsafe.Pointer, [][]float32, []spaData) {
	samples := make([][]float32, channels)
	data := make([]spaData, channels)
	chunks := make([]spaChunk, channels)
	for ch := range samples {
		samples[ch] = make([]float32, capacity)
		data[ch] = spaData{Maxsize: uint32(capacity * 4), Data: unsafe.Pointer(&samples[ch][0]), Chunk: &chunks[ch]}
	}
	spa := &spaBuffer{NDatas: uint32(channels), Datas: unsafe.Pointer(&data[0])}
	return unsafe.Pointer(&pwBuffer{Buffer: spa}), samples, data
}

func newCallbackPlayer(ops *callbackOps, fill func(*PCMBuffer) (int, error)) *player {
	p := newPlayer(PlayerConfig{Channels: 2, FramesPerBuffer: 64}, PlayerCallbacks{Fill: fill})
	p.streamOps = ops
	p.streamPtr = opaqueTestPtr()
	p.setState(PlayerStatePlaying)
	return p
}

// TestProcessCallbackSteadyStateAllocs is deliberately a RED-first regression
// test: after its warm-up callback, processing must not allocate.
func TestProcessCallbackSteadyStateAllocs(t *testing.T) {
	buffer, _, _ := callbackBuffer(2, 64, 64)
	ops := &callbackOps{buffers: make([]unsafe.Pointer, 1002)}
	for i := range ops.buffers {
		ops.buffers[i] = buffer
	}
	p := newCallbackPlayer(ops, func(buf *PCMBuffer) (int, error) { return buf.Frames, nil })

	p.onProcess() // Allocate reusable headers before measuring steady state.
	allocs := testing.AllocsPerRun(1000, p.onProcess)
	if allocs != 0 {
		t.Fatalf("steady-state process callback allocations = %v, want 0", allocs)
	}
}

func BenchmarkPlayerOnProcessStereo(b *testing.B) {
	buffer, _, _ := callbackBuffer(2, 64, 64)
	ops := &callbackOps{buffers: make([]unsafe.Pointer, b.N+1)}
	for i := range ops.buffers {
		ops.buffers[i] = buffer
	}
	p := newCallbackPlayer(ops, func(buf *PCMBuffer) (int, error) { return buf.Frames, nil })

	p.onProcess() // Warm reusable callback metadata before reporting allocations.
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.onProcess()
	}
}

func TestProcessCallbackRefreshesChangedPointersAndCapacities(t *testing.T) {
	first, firstSamples, _ := callbackBuffer(2, 64, 64)
	second, secondSamples, _ := callbackBuffer(2, 64, 96)
	ops := &callbackOps{buffers: []unsafe.Pointer{first, second}}
	calls := 0
	p := newCallbackPlayer(ops, func(buf *PCMBuffer) (int, error) {
		calls++
		for ch := range buf.Samples {
			buf.Samples[ch][0] = float32(calls)
			if cap(buf.Samples[ch]) < buf.Frames {
				t.Fatalf("channel %d capacity = %d, want at least %d", ch, cap(buf.Samples[ch]), buf.Frames)
			}
		}
		return buf.Frames, nil
	})

	p.onProcess()
	p.onProcess()
	if calls != 2 || ops.queued != 2 {
		t.Fatalf("calls=%d queues=%d, want 2 each", calls, ops.queued)
	}
	for ch := range firstSamples {
		if firstSamples[ch][0] != 1 {
			t.Errorf("first buffer channel %d = %v, want 1", ch, firstSamples[ch][0])
		}
		if secondSamples[ch][0] != 2 {
			t.Errorf("second buffer channel %d = %v, want 2", ch, secondSamples[ch][0])
		}
	}
}

func TestProcessCallbackInvalidMetadataClearsReusableViewAndRequeuesOnce(t *testing.T) {
	valid, _, _ := callbackBuffer(2, 64, 64)
	invalid, _, invalidData := callbackBuffer(2, 64, 64)
	// Fail after channel zero has already been refreshed to prove partial
	// validation cannot retain a native pointer from the rejected buffer.
	invalidData[1].Data = nil
	ops := &callbackOps{buffers: []unsafe.Pointer{valid, invalid}}
	fillCalls := 0
	p := newCallbackPlayer(ops, func(buf *PCMBuffer) (int, error) {
		fillCalls++
		return buf.Frames, nil
	})

	p.onProcess()
	p.setState(PlayerStatePlaying) // inspect the invalid-buffer behavior independently.
	p.onProcess()

	if fillCalls != 1 {
		t.Fatalf("Fill calls = %d, want 1 after invalid metadata", fillCalls)
	}
	if ops.queued != 2 {
		t.Fatalf("QueueBuffer calls = %d, want exactly 2", ops.queued)
	}
	if p.pwView.buf != nil || p.pwView.data != nil {
		t.Fatal("invalid metadata retained native buffer references")
	}
	for ch, samples := range p.pwView.samples {
		if samples != nil {
			t.Fatalf("invalid metadata retained channel %d samples", ch)
		}
	}
}

func TestProcessCallbackClearsReusableViewAfterQueueAndQueueError(t *testing.T) {
	buffer, _, _ := callbackBuffer(2, 64, 64)
	ops := &callbackOps{buffers: []unsafe.Pointer{buffer, buffer}}
	clearedDuringError := false
	p := newCallbackPlayer(ops, func(buf *PCMBuffer) (int, error) { return buf.Frames, nil })

	p.onProcess()
	if p.pwView.buf != nil || p.pwView.data != nil {
		t.Fatal("successful callback retained reusable native buffer references")
	}
	for ch, samples := range p.pwView.samples {
		if samples != nil {
			t.Fatalf("successful callback retained channel %d samples", ch)
		}
	}

	ops.queueErr = errors.New("queue failed")
	p.callbacks.OnError = func(error) {
		clearedDuringError = p.pwView.buf == nil && p.pwView.data == nil
		for _, samples := range p.pwView.samples {
			clearedDuringError = clearedDuringError && samples == nil
		}
	}
	p.setState(PlayerStatePlaying)
	p.onProcess()
	if !clearedDuringError {
		t.Fatal("queue error retained reusable native buffer references during OnError")
	}
}

var _ portout.StreamOps = (*callbackOps)(nil)
