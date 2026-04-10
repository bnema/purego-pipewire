package core

import (
	"errors"
	"testing"
	"unsafe"
)

// TestNewPWBufferViewWrapsWritablePlanarFloatData verifies that newPWBufferView
// correctly interprets a C-layout pwBuffer as planar float32 channel data.
func TestNewPWBufferViewWrapsWritablePlanarFloatData(t *testing.T) {
	const channels = 2
	const frames = 64

	// Allocate planar float32 backing memory for each channel.
	channelData := make([][]float32, channels)
	channelPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, frames)
		channelPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
	}

	// Build the C-layout chunk structs.
	chunks := make([]spaChunk, channels)
	for ch := 0; ch < channels; ch++ {
		chunks[ch] = spaChunk{
			Offset: 0,
			Size:   0, // will be set on Commit
			Stride: 0,
			Flags:  0,
		}
	}

	// Build the C-layout spaData structs.
	datas := make([]spaData, channels)
	for ch := 0; ch < channels; ch++ {
		datas[ch] = spaData{
			Type:    2,                  // SPA_DATA_MemPtr
			Flags:   3,                  // SPA_DATA_FLAG_READABLE | SPA_DATA_FLAG_WRITABLE
			Maxsize: uint32(frames * 4), // bytes
			Data:    channelPtrs[ch],
			Chunk:   &chunks[ch],
		}
	}

	// Build the C-layout spaBuffer.
	buf := spaBuffer{
		NMetas: 0,
		NDatas: uint32(channels),
		Metas:  nil,
		Datas:  unsafe.Pointer(&datas[0]),
	}

	// Build the C-layout pwBuffer.
	pwBuf := pwBuffer{
		Buffer:   &buf,
		UserData: nil,
		Size:     0,
	}

	view, err := newPWBufferView(unsafe.Pointer(&pwBuf), channels, frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcm := view.PCM()
	if pcm.Channels != channels {
		t.Errorf("expected %d channels, got %d", channels, pcm.Channels)
	}
	if pcm.Frames != frames {
		t.Errorf("expected %d frames, got %d", frames, pcm.Frames)
	}
	if len(pcm.Samples) != channels {
		t.Fatalf("expected %d sample slices, got %d", channels, len(pcm.Samples))
	}
	for ch := 0; ch < channels; ch++ {
		if len(pcm.Samples[ch]) != frames {
			t.Errorf("channel %d: expected %d frames, got %d", ch, frames, len(pcm.Samples[ch]))
		}
	}

	// Verify the PCM buffer shares memory with channelData (not a copy).
	pcm.Samples[0][0] = 1.0
	if channelData[0][0] != 1.0 {
		t.Error("PCM buffer does not share memory with underlying channel data — write did not propagate")
	}

	// Commit and verify chunk updates.
	const commitFrames = 32
	view.Commit(commitFrames)

	if pwBuf.Size != uint64(commitFrames) {
		t.Errorf("pwBuffer.Size = %d, want %d", pwBuf.Size, commitFrames)
	}
	for ch := 0; ch < channels; ch++ {
		if chunks[ch].Size != uint32(commitFrames*4) {
			t.Errorf("chunk[%d].Size = %d, want %d", ch, chunks[ch].Size, commitFrames*4)
		}
		if chunks[ch].Stride != 4 {
			t.Errorf("chunk[%d].Stride = %d, want 4", ch, chunks[ch].Stride)
		}
	}
}

// TestNewPWBufferViewRejectsNilPointer verifies that a nil buffer pointer
// returns ErrNilBufferPointer.
func TestNewPWBufferViewRejectsNilPointer(t *testing.T) {
	_, err := newPWBufferView(nil, 2, 64)
	if !errors.Is(err, ErrNilBufferPointer) {
		t.Fatalf("expected ErrNilBufferPointer, got %v", err)
	}
}

// TestNewPWBufferViewRejectsChannelMismatch verifies that a buffer whose
// NDatas does not match the requested channel count returns ErrChannelMismatch.
func TestNewPWBufferViewRejectsChannelMismatch(t *testing.T) {
	// Build a buffer claiming 1 data slot.
	singleData := spaData{
		Type:    2,
		Maxsize: 256,
		Data:    unsafe.Pointer(new(float32)),
		Chunk:   &spaChunk{},
	}
	buf := spaBuffer{
		NMetas: 0,
		NDatas: 1, // only 1 channel
		Datas:  unsafe.Pointer(&singleData),
	}
	pwBuf := pwBuffer{
		Buffer: &buf,
	}

	_, err := newPWBufferView(unsafe.Pointer(&pwBuf), 2, 64) // requesting 2 channels
	if !errors.Is(err, ErrChannelMismatch) {
		t.Fatalf("expected ErrChannelMismatch, got %v", err)
	}
}

// TestNewPWBufferViewRejectsUnmappedData verifies that a nil data pointer
// in a spaData slot returns ErrUnmappedData.
func TestNewPWBufferViewRejectsUnmappedData(t *testing.T) {
	datas := make([]spaData, 2)
	datas[0] = spaData{
		Type:    2,
		Maxsize: 256,
		Data:    nil, // nil — should trigger ErrUnmappedData
		Chunk:   &spaChunk{},
	}
	datas[1] = spaData{
		Type:    2,
		Maxsize: 256,
		Data:    unsafe.Pointer(new(float32)),
		Chunk:   &spaChunk{},
	}

	buf := spaBuffer{
		NMetas: 0,
		NDatas: 2,
		Datas:  unsafe.Pointer(&datas[0]),
	}
	pwBuf := pwBuffer{
		Buffer: &buf,
	}

	_, err := newPWBufferView(unsafe.Pointer(&pwBuf), 2, 64)
	if !errors.Is(err, ErrUnmappedData) {
		t.Fatalf("expected ErrUnmappedData, got %v", err)
	}
}

// TestNewPWBufferViewRejectsNilSpaBuffer verifies that a nil spa_buffer
// pointer inside the pwBuffer returns ErrNilBufferPointer.
func TestNewPWBufferViewRejectsNilSpaBuffer(t *testing.T) {
	pwBuf := pwBuffer{
		Buffer:   nil, // nil — should trigger ErrNilBufferPointer
		UserData: nil,
		Size:     0,
	}

	_, err := newPWBufferView(unsafe.Pointer(&pwBuf), 2, 64)
	if !errors.Is(err, ErrNilBufferPointer) {
		t.Fatalf("expected ErrNilBufferPointer, got %v", err)
	}
}

// TestCommitUpdatesSizeAndStride verifies that Commit writes the correct
// byte sizes and strides for planar float32 data.
func TestCommitUpdatesSizeAndStride(t *testing.T) {
	const channels = 2
	const frames = 128

	channelData := make([][]float32, channels)
	channelPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, frames)
		channelPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
	}

	chunks := make([]spaChunk, channels)
	datas := make([]spaData, channels)
	for ch := 0; ch < channels; ch++ {
		datas[ch] = spaData{
			Type:    2,
			Maxsize: uint32(frames * 4),
			Data:    channelPtrs[ch],
			Chunk:   &chunks[ch],
		}
	}

	buf := spaBuffer{
		NMetas: 0,
		NDatas: uint32(channels),
		Datas:  unsafe.Pointer(&datas[0]),
	}
	pwBuf := pwBuffer{
		Buffer: &buf,
	}

	view, err := newPWBufferView(unsafe.Pointer(&pwBuf), channels, frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Write some audio data to the PCM buffer.
	const writtenFrames = 96
	pcm := view.PCM()
	for ch := 0; ch < channels; ch++ {
		for f := 0; f < writtenFrames; f++ {
			pcm.Samples[ch][f] = float32(f) * 0.5
		}
	}

	view.Commit(writtenFrames)

	// Verify byte size = frames × sizeof(float32) per chunk.
	for ch := 0; ch < channels; ch++ {
		wantSize := uint32(writtenFrames * 4)
		if chunks[ch].Size != wantSize {
			t.Errorf("chunk[%d].Size = %d, want %d", ch, chunks[ch].Size, wantSize)
		}
		if chunks[ch].Stride != 4 {
			t.Errorf("chunk[%d].Stride = %d, want 4", ch, chunks[ch].Stride)
		}
	}

	// Verify pwBuffer.Size reflects frame count (not bytes).
	if pwBuf.Size != uint64(writtenFrames) {
		t.Errorf("pwBuffer.Size = %d, want %d", pwBuf.Size, writtenFrames)
	}
}
