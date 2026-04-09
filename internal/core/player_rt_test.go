package core

import (
	"errors"
	"io"
	"testing"
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
