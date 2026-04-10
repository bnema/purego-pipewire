package core

import (
	"errors"
	"io"
	"testing"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
	"github.com/bnema/purego-pipewire/internal/ports/out/mocks"
	"github.com/stretchr/testify/mock"
)

// TestProcessCallbackSilenceFillUnderrun tests that when Fill returns fewer frames
// than requested, the remaining frames are filled with silence
func TestProcessCallbackSilenceFillUnderrun(t *testing.T) {
	fillCallCount := 0
	var receivedBuf *PCMBuffer

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				fillCallCount++
				receivedBuf = buf
				// Return only 2 frames, leaving 2 frames as underrun
				return 2, nil
			},
		},
	)

	// Set player to playing state
	p.setState(PlayerStatePlaying)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	frames, err := p.processPCM(buf)
	if err != nil {
		t.Fatalf("processPCM returned error: %v", err)
	}

	if frames != 4 {
		t.Errorf("processPCM returned %d frames, want 4", frames)
	}

	if fillCallCount != 1 {
		t.Errorf("Fill callback called %d times, want 1", fillCallCount)
	}

	// Verify the buffer was silence-filled
	// Frames 0-1 should have data (we don't verify the actual values since Fill could write anything)
	// Frames 2-3 should be silence (0.0)
	for ch := 0; ch < 2; ch++ {
		for frame := 2; frame < 4; frame++ {
			if buf.Samples[ch][frame] != 0.0 {
				t.Errorf("Expected silence at channel %d frame %d, got %f", ch, frame, buf.Samples[ch][frame])
			}
		}
	}

	if receivedBuf != buf {
		t.Error("Fill callback received different buffer than passed to processPCM")
	}
}

// TestProcessCallbackFailFastUnderrun tests that when configured to fail on underrun,
// the player transitions to error state and invokes OnError callback
func TestProcessCallbackFailFastUnderrun(t *testing.T) {
	errCalled := false
	var errReceived error

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFailFast,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				// Return only 2 frames, triggering underrun
				return 2, nil
			},
			OnError: func(err error) {
				errCalled = true
				errReceived = err
			},
		},
	)

	// Set player to playing state
	p.setState(PlayerStatePlaying)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	_, err := p.processPCM(buf)
	if err == nil {
		t.Fatal("processPCM should have returned error for underrun with FailFast policy")
	}

	if p.State() != PlayerStateError {
		t.Errorf("expected state Error after underrun, got %v", p.State())
	}

	if !errCalled {
		t.Error("OnError callback was not called")
	}

	if errReceived == nil {
		t.Error("OnError callback received nil error")
	}
}

// TestProcessCallbackPausedSuppressesFillAndEmitsSilence tests that when paused,
// Fill is not called and the buffer is filled with silence
func TestProcessCallbackPausedSuppressesFillAndEmitsSilence(t *testing.T) {
	fillCallCount := 0

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				fillCallCount++
				return 4, nil
			},
		},
	)

	// Set player to paused state
	p.setState(PlayerStatePaused)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	frames, err := p.processPCM(buf)
	if err != nil {
		t.Fatalf("processPCM returned error: %v", err)
	}

	if frames != 4 {
		t.Errorf("processPCM returned %d frames, want 4", frames)
	}

	if fillCallCount != 0 {
		t.Errorf("Fill callback called %d times while paused, want 0", fillCallCount)
	}

	// Verify the entire buffer is silence
	for ch := 0; ch < 2; ch++ {
		for frame := 0; frame < 4; frame++ {
			if buf.Samples[ch][frame] != 0.0 {
				t.Errorf("Expected silence at channel %d frame %d while paused, got %f", ch, frame, buf.Samples[ch][frame])
			}
		}
	}
}

// TestProcessCallbackStoppedFillsSilence tests that when stopped,
// Fill is not called and the buffer is filled with silence
func TestProcessCallbackStoppedFillsSilence(t *testing.T) {
	fillCallCount := 0

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				fillCallCount++
				return 4, nil
			},
		},
	)

	// Set player to stopped state
	p.setState(PlayerStateStopped)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	frames, err := p.processPCM(buf)
	if err != nil {
		t.Fatalf("processPCM returned error: %v", err)
	}

	if frames != 4 {
		t.Errorf("processPCM returned %d frames, want 4", frames)
	}

	if fillCallCount != 0 {
		t.Errorf("Fill callback called %d times while stopped, want 0", fillCallCount)
	}

	// Verify the entire buffer is silence
	for ch := 0; ch < 2; ch++ {
		for frame := 0; frame < 4; frame++ {
			if buf.Samples[ch][frame] != 0.0 {
				t.Errorf("Expected silence at channel %d frame %d while stopped, got %f", ch, frame, buf.Samples[ch][frame])
			}
		}
	}
}

// TestProcessCallbackDrainOnEOF tests that io.EOF from Fill triggers drain callback
func TestProcessCallbackDrainOnEOF(t *testing.T) {
	drainCalled := false
	fillCallCount := 0

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				fillCallCount++
				// Return EOF to signal drain
				return 2, io.EOF
			},
			OnDrain: func() {
				drainCalled = true
			},
		},
	)

	// Set player to playing state
	p.setState(PlayerStatePlaying)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	frames, err := p.processPCM(buf)
	if err != nil {
		t.Fatalf("processPCM returned error: %v", err)
	}

	if frames != 2 {
		t.Errorf("processPCM returned %d frames, want 2", frames)
	}

	if !drainCalled {
		t.Error("OnDrain callback was not called on io.EOF")
	}

	if fillCallCount != 1 {
		t.Errorf("Fill callback called %d times, want 1", fillCallCount)
	}
}

// TestProcessCallbackErrorHandling tests that errors from Fill (other than EOF)
// fail the player and invoke OnError callback
func TestProcessCallbackErrorHandling(t *testing.T) {
	testErr := errors.New("fill error")
	errCalled := false
	var errReceived error

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				return 0, testErr
			},
			OnError: func(err error) {
				errCalled = true
				errReceived = err
			},
		},
	)

	// Set player to playing state
	p.setState(PlayerStatePlaying)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	_, err := p.processPCM(buf)
	if err == nil {
		t.Fatal("processPCM should have returned error")
	}

	if p.State() != PlayerStateError {
		t.Errorf("expected state Error after fill error, got %v", p.State())
	}

	if !errCalled {
		t.Error("OnError callback was not called")
	}

	if !errors.Is(errReceived, testErr) {
		t.Errorf("OnError received error %v, want %v", errReceived, testErr)
	}
}

// TestProcessCallbackIdleFillsSilence tests that when idle,
// Fill is not called and the buffer is filled with silence
func TestProcessCallbackIdleFillsSilence(t *testing.T) {
	fillCallCount := 0

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				fillCallCount++
				return 4, nil
			},
		},
	)

	// Player starts in Idle state
	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	frames, err := p.processPCM(buf)
	if err != nil {
		t.Fatalf("processPCM returned error: %v", err)
	}

	if frames != 4 {
		t.Errorf("processPCM returned %d frames, want 4", frames)
	}

	if fillCallCount != 0 {
		t.Errorf("Fill callback called %d times while idle, want 0", fillCallCount)
	}
}

// TestProcessCallbackUnderrunCallback tests that OnUnderrun is called when underrun occurs
func TestProcessCallbackUnderrunCallback(t *testing.T) {
	underrunCalled := false
	underrunFrames := 0

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				return 2, nil
			},
			OnUnderrun: func(frames int) {
				underrunCalled = true
				underrunFrames = frames
			},
		},
	)

	// Set player to playing state
	p.setState(PlayerStatePlaying)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	_, err := p.processPCM(buf)
	if err != nil {
		t.Fatalf("processPCM returned error: %v", err)
	}

	if !underrunCalled {
		t.Error("OnUnderrun callback was not called")
	}

	if underrunFrames != 2 {
		t.Errorf("OnUnderrun received %d frames, want 2", underrunFrames)
	}
}

// TestProcessCallbackFillReturnsFullFrames tests normal operation when Fill returns all requested frames
func TestProcessCallbackFillReturnsFullFrames(t *testing.T) {
	fillCallCount := 0

	p := newPlayer(
		PlayerConfig{
			FramesPerBuffer: 4,
			Channels:        2,
			UnderrunPolicy:  UnderrunFillSilence,
		},
		PlayerCallbacks{
			Fill: func(buf *PCMBuffer) (int, error) {
				fillCallCount++
				// Write some data to verify Fill was called
				for ch := 0; ch < buf.Channels; ch++ {
					for frame := 0; frame < buf.Frames; frame++ {
						buf.Samples[ch][frame] = float32(frame + ch*10)
					}
				}
				return 4, nil
			},
		},
	)

	// Set player to playing state
	p.setState(PlayerStatePlaying)

	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}
	buf.allocate()

	frames, err := p.processPCM(buf)
	if err != nil {
		t.Fatalf("processPCM returned error: %v", err)
	}

	if frames != 4 {
		t.Errorf("processPCM returned %d frames, want 4", frames)
	}

	if fillCallCount != 1 {
		t.Errorf("Fill callback called %d times, want 1", fillCallCount)
	}

	// Verify the data was written by Fill
	for ch := 0; ch < 2; ch++ {
		for frame := 0; frame < 4; frame++ {
			expected := float32(frame + ch*10)
			if buf.Samples[ch][frame] != expected {
				t.Errorf("Expected %f at channel %d frame %d, got %f", expected, ch, frame, buf.Samples[ch][frame])
			}
		}
	}
}

// TestPCMBufferAllocate tests the allocate method
func TestPCMBufferAllocate(t *testing.T) {
	buf := &PCMBuffer{
		Frames:   4,
		Channels: 2,
	}

	buf.allocate()

	if buf.Samples == nil {
		t.Fatal("Samples is nil after allocate")
	}

	if len(buf.Samples) != 2 {
		t.Errorf("len(Samples) = %d, want 2", len(buf.Samples))
	}

	for ch := 0; ch < 2; ch++ {
		if len(buf.Samples[ch]) != 4 {
			t.Errorf("len(Samples[%d]) = %d, want 4", ch, len(buf.Samples[ch]))
		}
	}
}

// --- Task 6: Start and process callback integration tests ---

// TestPlayerStartCreatesLoopConnectsStreamAndRunsMainLoop verifies that Start()
// creates the main loop, creates a playback stream, connects it with the right
// format, activates it, stores owned pointers, and starts the main loop in a
// goroutine — then transitions to Playing.
func TestPlayerStartCreatesLoopConnectsStreamAndRunsMainLoop(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 256,
	}

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state after Start, got %v", p.State())
	}

	// Verify owned pointers are set
	p.mu.Lock()
	loopPtr := p.loopPtr
	streamPtr := p.streamPtr
	p.mu.Unlock()

	if loopPtr != fakeLoop {
		t.Fatalf("expected loopPtr = %v, got %v", fakeLoop, loopPtr)
	}
	if streamPtr != fakeStream {
		t.Fatalf("expected streamPtr = %v, got %v", fakeStream, streamPtr)
	}

	// Wait for the RunMainLoop goroutine to finish
	waitOnMainLoop(t, wg)
}

// TestProcessCallbackDequeuesProcessesAndQueues verifies the full onProcess
// path: dequeue buffer, build pwBufferView, call processPCM, commit, and
// queue the buffer back.
func TestProcessCallbackDequeuesProcessesAndQueues(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 64,
	}

	// Build a real pwBuffer that onProcess will dequeue
	const channels = 2
	const frames = 64
	channelData := make([][]float32, channels)
	chunkSlice := make([]spaChunk, channels)
	dataSlice := make([]spaData, channels)
	floatPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, frames)
		floatPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
		dataSlice[ch] = spaData{
			Type:    spaDataTypeMemPtr,
			Flags:   spaDataFlagRW,
			Maxsize: uint32(frames * 4),
			Data:    floatPtrs[ch],
			Chunk:   &chunkSlice[ch],
		}
	}
	spaBuf := spaBuffer{
		NDatas: uint32(channels),
		Datas:  unsafe.Pointer(&dataSlice[0]),
	}
	pwBuf := pwBuffer{
		Buffer: &spaBuf,
	}
	bufPtr := unsafe.Pointer(&pwBuf)

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	// Expect DequeueBuffer, then QueueBuffer after processing
	mockOps.EXPECT().DequeueBuffer(fakeStream).Return(bufPtr)
	mockOps.EXPECT().QueueBuffer(fakeStream, bufPtr).Return(nil)

	fillCalled := false
	p := newPlayer(cfg, PlayerCallbacks{
		Fill: func(buf *PCMBuffer) (int, error) {
			fillCalled = true
			// Write data to verify it propagates
			for ch := 0; ch < buf.Channels; ch++ {
				for f := 0; f < buf.Frames; f++ {
					buf.Samples[ch][f] = float32(f)
				}
			}
			return buf.Frames, nil
		},
	})
	p.streamOps = mockOps

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if p.State() != PlayerStatePlaying {
		t.Fatalf("expected Playing state, got %v", p.State())
	}

	// Call onProcess directly (simulating PipeWire calling it)
	p.onProcess()

	if !fillCalled {
		t.Error("Fill callback was not called during onProcess")
	}

	// Verify chunk sizes were updated by Commit
	for ch := 0; ch < channels; ch++ {
		if chunkSlice[ch].Size != uint32(frames*4) {
			t.Errorf("chunk[%d].Size = %d, want %d", ch, chunkSlice[ch].Size, frames*4)
		}
		if chunkSlice[ch].Stride != 4 {
			t.Errorf("chunk[%d].Stride = %d, want 4", ch, chunkSlice[ch].Stride)
		}
	}

	// Verify pwBuffer.Size reflects frame count
	if pwBuf.Size != uint64(frames) {
		t.Errorf("pwBuf.Size = %d, want %d", pwBuf.Size, frames)
	}

	waitOnMainLoop(t, wg)
}

// TestProcessCallbackNilBufferReturnsEarly verifies that onProcess returns
// early when DequeueBuffer returns nil.
func TestProcessCallbackNilBufferReturnsEarly(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := defaultTestConfig()

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	fillCalled := false
	p := newPlayer(cfg, PlayerCallbacks{
		Fill: func(buf *PCMBuffer) (int, error) {
			fillCalled = true
			return buf.Frames, nil
		},
	})
	p.streamOps = mockOps

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	// DequeueBuffer returns nil — onProcess should return early
	mockOps.EXPECT().DequeueBuffer(fakeStream).Return(nil)

	p.onProcess()

	if fillCalled {
		t.Error("Fill should not be called when DequeueBuffer returns nil")
	}
}

// TestProcessCallbackQueueBufferFailsRoutesThroughFail verifies that when
// QueueBuffer returns an error, onProcess routes it through p.fail().
func TestProcessCallbackQueueBufferFailsRoutesThroughFail(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 64,
	}

	// Build a real pwBuffer
	const channels = 2
	const frames = 64
	channelData := make([][]float32, channels)
	floatPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, frames)
		floatPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
	}
	chunkSlice := make([]spaChunk, channels)
	dataSlice := make([]spaData, channels)
	for ch := 0; ch < channels; ch++ {
		dataSlice[ch] = spaData{
			Type:    spaDataTypeMemPtr,
			Flags:   spaDataFlagRW,
			Maxsize: uint32(frames * 4),
			Data:    floatPtrs[ch],
			Chunk:   &chunkSlice[ch],
		}
	}
	spaBuf := spaBuffer{
		NDatas: uint32(channels),
		Datas:  unsafe.Pointer(&dataSlice[0]),
	}
	pwBuf := pwBuffer{
		Buffer: &spaBuf,
	}
	bufPtr := unsafe.Pointer(&pwBuf)

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	queueErr := errors.New("queue buffer failed")
	errCalled := false
	var errReceived error

	p := newPlayer(cfg, PlayerCallbacks{
		Fill: func(buf *PCMBuffer) (int, error) {
			return buf.Frames, nil
		},
		OnError: func(err error) {
			errCalled = true
			errReceived = err
		},
	})
	p.streamOps = mockOps

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	// Expect DequeueBuffer to return a buffer, then QueueBuffer to fail
	mockOps.EXPECT().DequeueBuffer(fakeStream).Return(bufPtr)
	mockOps.EXPECT().QueueBuffer(fakeStream, bufPtr).Return(queueErr)

	p.onProcess()

	if !errCalled {
		t.Error("OnError should have been called when QueueBuffer fails")
	}
	if errReceived == nil {
		t.Error("expected non-nil error from OnError")
	}
	if p.State() != PlayerStateError {
		t.Errorf("expected Error state after QueueBuffer failure, got %v", p.State())
	}
}

// TestProcessCallbackBufferViewFailsRoutesThroughFail verifies that when
// newPWBufferView fails, onProcess routes the error through p.fail()
// and does not attempt to queue the buffer.
func TestProcessCallbackBufferViewFailsRoutesThroughFail(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 64,
	}

	// Build a buffer with NDatas mismatch to trigger ErrChannelMismatch
	// NDatas = 1 but config.Channels = 2
	singleData := spaData{
		Type:    spaDataTypeMemPtr,
		Maxsize: 256,
		Data:    unsafe.Pointer(new(float32)),
		Chunk:   &spaChunk{},
	}
	spaBuf := spaBuffer{
		NDatas: 1, // mismatch with Channels=2
		Datas:  unsafe.Pointer(&singleData),
	}
	pwBuf := pwBuffer{
		Buffer: &spaBuf,
	}
	bufPtr := unsafe.Pointer(&pwBuf)

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	errCalled := false
	var errReceived error

	p := newPlayer(cfg, PlayerCallbacks{
		Fill: func(buf *PCMBuffer) (int, error) {
			return buf.Frames, nil
		},
		OnError: func(err error) {
			errCalled = true
			errReceived = err
		},
	})
	p.streamOps = mockOps

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	// DequeueBuffer returns a buffer that will fail newPWBufferView
	// QueueBuffer should NOT be called
	mockOps.EXPECT().DequeueBuffer(fakeStream).Return(bufPtr)

	p.onProcess()

	if !errCalled {
		t.Error("OnError should have been called when buffer view fails")
	}
	if errReceived == nil {
		t.Error("expected non-nil error from OnError")
	}
	if !errors.Is(errReceived, ErrChannelMismatch) {
		t.Errorf("expected ErrChannelMismatch, got %v", errReceived)
	}
	if p.State() != PlayerStateError {
		t.Errorf("expected Error state after buffer view failure, got %v", p.State())
	}
}

// TestProcessCallbackProcessCMErrorSkipsQueue verifies that when processPCM
// returns an error, onProcess does not queue the buffer and lets the existing
// error behavior stand.
func TestProcessCallbackProcessCMErrorSkipsQueue(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := PlayerConfig{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 64,
	}

	// Build a real pwBuffer
	const channels = 2
	const frames = 64
	channelData := make([][]float32, channels)
	floatPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, frames)
		floatPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
	}
	chunkSlice := make([]spaChunk, channels)
	dataSlice := make([]spaData, channels)
	for ch := 0; ch < channels; ch++ {
		dataSlice[ch] = spaData{
			Type:    spaDataTypeMemPtr,
			Flags:   spaDataFlagRW,
			Maxsize: uint32(frames * 4),
			Data:    floatPtrs[ch],
			Chunk:   &chunkSlice[ch],
		}
	}
	spaBuf := spaBuffer{
		NDatas: uint32(channels),
		Datas:  unsafe.Pointer(&dataSlice[0]),
	}
	pwBuf := pwBuffer{
		Buffer: &spaBuf,
	}
	bufPtr := unsafe.Pointer(&pwBuf)

	wg := expectStartWithSync(mockOps, fakeLoop, fakeStream, cfg)

	fillErr := errors.New("fill error")
	errCalled := false
	var errReceived error

	p := newPlayer(cfg, PlayerCallbacks{
		Fill: func(buf *PCMBuffer) (int, error) {
			return 0, fillErr
		},
		OnError: func(err error) {
			errCalled = true
			errReceived = err
		},
	})
	p.streamOps = mockOps

	if err := p.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	waitOnMainLoop(t, wg)

	// DequeueBuffer returns a buffer; QueueBuffer should NOT be called
	// because processPCM (via Fill) returns an error
	mockOps.EXPECT().DequeueBuffer(fakeStream).Return(bufPtr)
	// Note: no QueueBuffer expectation

	p.onProcess()

	if !errCalled {
		t.Error("OnError should have been called when Fill returns an error")
	}
	if !errors.Is(errReceived, fillErr) {
		t.Errorf("expected fillErr, got %v", errReceived)
	}
	if p.State() != PlayerStateError {
		t.Errorf("expected Error state after Fill error, got %v", p.State())
	}
}

// TestPlayerStartFailedConnectDestroysResources verifies that when
// ConnectPlaybackStream fails after stream creation, the stream
// and loop are destroyed and no stale pointers are left.
func TestPlayerStartFailedConnectDestroysResources(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := defaultTestConfig()

	connectErr := errors.New("connect failed")

	mockOps.EXPECT().CreateMainLoop().Return(fakeLoop, nil)
	mockOps.EXPECT().CreatePlaybackStream(fakeLoop, "purego-pipewire-player", mock.AnythingOfType("func()")).Return(fakeStream, nil)
	mockOps.EXPECT().ConnectPlaybackStream(fakeStream, portout.PlaybackFormat{
		SampleRate:      cfg.SampleRate,
		Channels:        cfg.Channels,
		FramesPerBuffer: cfg.FramesPerBuffer,
	}).Return(connectErr)
	// On connect failure, stream and loop must be destroyed
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	err := p.Start()
	if err == nil {
		t.Fatal("expected Start to fail when connect fails")
	}
	if !errors.Is(err, connectErr) {
		t.Fatalf("expected connectErr, got %v", err)
	}

	// Player should be in Error state
	if p.State() != PlayerStateError {
		t.Errorf("expected Error state after failed connect, got %v", p.State())
	}

	// No stale pointers should remain
	p.mu.Lock()
	loopPtr := p.loopPtr
	streamPtr := p.streamPtr
	p.mu.Unlock()

	if loopPtr != nil {
		t.Error("expected loopPtr to be nil after failed start")
	}
	if streamPtr != nil {
		t.Error("expected streamPtr to be nil after failed start")
	}
}

// TestPlayerStartFailedActivateDestroysResources verifies that when
// SetStreamActive fails after stream creation and connection, the
// stream is disconnected (best-effort), destroyed, the loop is destroyed,
// and no stale pointers are left.
func TestPlayerStartFailedActivateDestroysResources(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	fakeStream := unsafe.Pointer(uintptr(0xBEEF))
	cfg := defaultTestConfig()

	activateErr := errors.New("activate failed")

	mockOps.EXPECT().CreateMainLoop().Return(fakeLoop, nil)
	mockOps.EXPECT().CreatePlaybackStream(fakeLoop, "purego-pipewire-player", mock.AnythingOfType("func()")).Return(fakeStream, nil)
	mockOps.EXPECT().ConnectPlaybackStream(fakeStream, portout.PlaybackFormat{
		SampleRate:      cfg.SampleRate,
		Channels:        cfg.Channels,
		FramesPerBuffer: cfg.FramesPerBuffer,
	}).Return(nil)
	mockOps.EXPECT().SetStreamActive(fakeStream, true).Return(activateErr)
	// On activate failure: disconnect (best-effort), destroy stream, destroy loop
	mockOps.EXPECT().DisconnectStream(fakeStream).Return(nil)
	mockOps.EXPECT().DestroyStream(fakeStream).Return()
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	err := p.Start()
	if err == nil {
		t.Fatal("expected Start to fail when activate fails")
	}
	if !errors.Is(err, activateErr) {
		t.Fatalf("expected activateErr, got %v", err)
	}

	// Player should be in Error state
	if p.State() != PlayerStateError {
		t.Errorf("expected Error state after failed activate, got %v", p.State())
	}

	// No stale pointers should remain
	p.mu.Lock()
	loopPtr := p.loopPtr
	streamPtr := p.streamPtr
	p.mu.Unlock()

	if loopPtr != nil {
		t.Error("expected loopPtr to be nil after failed start")
	}
	if streamPtr != nil {
		t.Error("expected streamPtr to be nil after failed start")
	}
}

// TestPlayerStartFailedCreateStreamDestroysLoop verifies that when
// CreatePlaybackStream fails after main loop creation, the loop is
// destroyed and no stale pointers are left.
func TestPlayerStartFailedCreateStreamDestroysLoop(t *testing.T) {
	mockOps := mocks.NewMockStreamOps(t)
	fakeLoop := unsafe.Pointer(uintptr(0xCAFE))
	cfg := defaultTestConfig()

	createErr := errors.New("stream creation failed")

	mockOps.EXPECT().CreateMainLoop().Return(fakeLoop, nil)
	mockOps.EXPECT().CreatePlaybackStream(fakeLoop, "purego-pipewire-player", mock.AnythingOfType("func()")).Return(nil, createErr)
	// On stream creation failure, destroy the loop
	mockOps.EXPECT().DestroyMainLoop(fakeLoop).Return()

	p := newPlayer(cfg, PlayerCallbacks{})
	p.streamOps = mockOps

	err := p.Start()
	if err == nil {
		t.Fatal("expected Start to fail when stream creation fails")
	}
	if !errors.Is(err, createErr) {
		t.Fatalf("expected createErr, got %v", err)
	}

	// Player should be in Error state
	if p.State() != PlayerStateError {
		t.Errorf("expected Error state after failed stream creation, got %v", p.State())
	}

	// No stale pointers should remain
	p.mu.Lock()
	loopPtr := p.loopPtr
	streamPtr := p.streamPtr
	p.mu.Unlock()

	if loopPtr != nil {
		t.Error("expected loopPtr to be nil after failed start")
	}
	if streamPtr != nil {
		t.Error("expected streamPtr to be nil after failed start")
	}
}
