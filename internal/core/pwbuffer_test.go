package core

import (
	"errors"
	"testing"
	"unsafe"
)

// Named constants for SPA data type and flag values (from <spa/buffer/buffer.h>).
const (
	spaDataTypeMemPtr uint32 = 2 // SPA_DATA_MemPtr
	spaDataFlagRW     uint32 = 3 // SPA_DATA_FLAG_READABLE | SPA_DATA_FLAG_WRITABLE
)

// helper to build a channels×frames buffer view for tests.
// Returns the view, the chunks slice, and the pwBuffer pointer.
func newTestBufferView(t *testing.T, channels, frames, maxsizeFrames int) (*pwBufferView, []spaChunk, *pwBuffer) {
	t.Helper()
	channelData := make([][]float32, channels)
	channelPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, maxsizeFrames)
		channelPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
	}

	chunks := make([]spaChunk, channels)
	datas := make([]spaData, channels)
	for ch := 0; ch < channels; ch++ {
		chunks[ch] = spaChunk{}
		datas[ch] = spaData{
			Type:    spaDataTypeMemPtr,
			Flags:   spaDataFlagRW,
			Maxsize: uint32(maxsizeFrames * 4),
			Data:    channelPtrs[ch],
			Chunk:   &chunks[ch],
		}
	}

	buf := spaBuffer{
		NDatas: uint32(channels),
		Datas:  unsafe.Pointer(&datas[0]),
	}
	pwBuf := pwBuffer{
		Buffer: &buf,
	}

	view, err := newPWBufferView(unsafe.Pointer(&pwBuf), channels, frames)
	if err != nil {
		t.Fatalf("unexpected error building test buffer: %v", err)
	}
	return view, chunks, &pwBuf
}

// TestNewPWBufferViewWrapsWritablePlanarFloatData verifies that newPWBufferView
// correctly interprets a C-layout pwBuffer as planar float32 channel data.
func TestNewPWBufferViewWrapsWritablePlanarFloatData(t *testing.T) {
	const channels = 2
	const frames = 64

	view, _, _ := newTestBufferView(t, channels, frames, frames)

	pcm := view.PCM()
	if pcm.Channels != channels {
		t.Errorf("expected %d channels, got %d", channels, pcm.Channels)
	}
	if pcm.Frames != frames {
		t.Errorf("expected %d frames, got %d", frames, pcm.Frames)
	}
	if pcm.Stride != 4 {
		t.Errorf("expected Stride=4 for planar float32, got %d", pcm.Stride)
	}
	if len(pcm.Samples) != channels {
		t.Fatalf("expected %d sample slices, got %d", channels, len(pcm.Samples))
	}
	for ch := 0; ch < channels; ch++ {
		if len(pcm.Samples[ch]) != frames {
			t.Errorf("channel %d: expected %d frames, got %d", ch, frames, len(pcm.Samples[ch]))
		}
	}
}

// TestNewPWBufferViewPCMStrideIsSet verifies that PCM() returns a buffer
// with Stride set to 4 (bytes per float32 for planar layout).
func TestNewPWBufferViewPCMStrideIsSet(t *testing.T) {
	const channels = 1
	const frames = 32

	view, _, _ := newTestBufferView(t, channels, frames, frames)

	pcm := view.PCM()
	if pcm.Stride != 4 {
		t.Errorf("PCMBuffer.Stride = %d, want 4 (sizeof float32)", pcm.Stride)
	}
}

// TestNewPWBufferViewSharedMemory verifies that writes to the PCMBuffer
// propagate to the underlying channel data (not a copy).
func TestNewPWBufferViewSharedMemory(t *testing.T) {
	const channels = 2
	const frames = 64

	channelData := make([][]float32, channels)
	channelPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, frames)
		channelPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
	}

	chunks := make([]spaChunk, channels)
	datas := make([]spaData, channels)
	for ch := 0; ch < channels; ch++ {
		chunks[ch] = spaChunk{}
		datas[ch] = spaData{
			Type:    spaDataTypeMemPtr,
			Flags:   spaDataFlagRW,
			Maxsize: uint32(frames * 4),
			Data:    channelPtrs[ch],
			Chunk:   &chunks[ch],
		}
	}

	buf := spaBuffer{
		NDatas: uint32(channels),
		Datas:  unsafe.Pointer(&datas[0]),
	}
	pwBuf := pwBuffer{Buffer: &buf}

	view, err := newPWBufferView(unsafe.Pointer(&pwBuf), channels, frames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pcm := view.PCM()
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
	singleData := spaData{
		Type:    spaDataTypeMemPtr,
		Maxsize: 256,
		Data:    unsafe.Pointer(new(float32)),
		Chunk:   &spaChunk{},
	}
	buf := spaBuffer{
		NDatas: 1, // only 1 channel
		Datas:  unsafe.Pointer(&singleData),
	}
	pwBuf := pwBuffer{Buffer: &buf}

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
		Type:    spaDataTypeMemPtr,
		Maxsize: 256,
		Data:    nil, // nil — should trigger ErrUnmappedData
		Chunk:   &spaChunk{},
	}
	datas[1] = spaData{
		Type:    spaDataTypeMemPtr,
		Maxsize: 256,
		Data:    unsafe.Pointer(new(float32)),
		Chunk:   &spaChunk{},
	}

	buf := spaBuffer{
		NDatas: 2,
		Datas:  unsafe.Pointer(&datas[0]),
	}
	pwBuf := pwBuffer{Buffer: &buf}

	_, err := newPWBufferView(unsafe.Pointer(&pwBuf), 2, 64)
	if !errors.Is(err, ErrUnmappedData) {
		t.Fatalf("expected ErrUnmappedData, got %v", err)
	}
}

// TestNewPWBufferViewRejectsNilSpaBuffer verifies that a nil spa_buffer
// pointer inside the pwBuffer returns ErrNilBufferPointer.
func TestNewPWBufferViewRejectsNilSpaBuffer(t *testing.T) {
	pwBuf := pwBuffer{
		Buffer: nil,
	}

	_, err := newPWBufferView(unsafe.Pointer(&pwBuf), 2, 64)
	if !errors.Is(err, ErrNilBufferPointer) {
		t.Fatalf("expected ErrNilBufferPointer, got %v", err)
	}
}

// TestNewPWBufferViewRejectsOversizedFrames verifies that requesting more
// frames than the buffer can hold (based on Maxsize) returns ErrFrameOverflow.
func TestNewPWBufferViewRejectsOversizedFrames(t *testing.T) {
	const channels = 2
	const maxFrames = 32
	const requestedFrames = 64 // exceeds maxFrames

	channelData := make([][]float32, channels)
	channelPtrs := make([]unsafe.Pointer, channels)
	for ch := 0; ch < channels; ch++ {
		channelData[ch] = make([]float32, maxFrames)
		channelPtrs[ch] = unsafe.Pointer(&channelData[ch][0])
	}

	datas := make([]spaData, channels)
	for ch := 0; ch < channels; ch++ {
		datas[ch] = spaData{
			Type:    spaDataTypeMemPtr,
			Maxsize: uint32(maxFrames * 4), // only room for 32 frames
			Data:    channelPtrs[ch],
			Chunk:   &spaChunk{},
		}
	}

	buf := spaBuffer{
		NDatas: uint32(channels),
		Datas:  unsafe.Pointer(&datas[0]),
	}
	pwBuf := pwBuffer{Buffer: &buf}

	_, err := newPWBufferView(unsafe.Pointer(&pwBuf), channels, requestedFrames)
	if !errors.Is(err, ErrFrameOverflow) {
		t.Fatalf("expected ErrFrameOverflow, got %v", err)
	}
}

// TestCommitUpdatesSizeAndStride verifies that Commit writes the correct
// byte sizes and strides for planar float32 data.
func TestCommitUpdatesSizeAndStride(t *testing.T) {
	const channels = 2
	const frames = 128

	view, chunks, pwBuf := newTestBufferView(t, channels, frames, frames)

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
