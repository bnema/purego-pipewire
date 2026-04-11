package capi

import (
	"encoding/binary"
	"errors"
	"testing"
	"unsafe"

	portout "github.com/bnema/purego-pipewire/internal/ports/out"
)

// TestBuildRawAudioParamsProducesOneConnectParam verifies that buildRawAudioParams
// produces exactly one connect param with the correct SPA_PARAM_EnumFormat structure
// for a valid PlaybackFormat with planar float32.
func TestBuildRawAudioParamsProducesOneConnectParam(t *testing.T) {
	fmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}

	cp, err := buildRawAudioParams(fmt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cp.Count() != 1 {
		t.Fatalf("expected 1 param, got %d", cp.Count())
	}

	// The param must be a non-nil pointer.
	ptr := cp.Pointer()
	if ptr == nil {
		t.Fatal("param pointer must not be nil")
	}

	// Read the POD header from storage to verify structure.
	// An SPA POD Object starts with: {uint32 size, uint32 type}
	// type must be SPA_TYPE_Object (15).
	podSize := binary.LittleEndian.Uint32(cp.storage[0:4])
	podType := binary.LittleEndian.Uint32(cp.storage[4:8])

	const spaTypeObject uint32 = 15
	if podType != spaTypeObject {
		t.Fatalf("expected POD type SPA_TYPE_Object (%d), got %d", spaTypeObject, podType)
	}

	// Immediately after the pod header is object body: {uint32 objType, uint32 id}
	objType := binary.LittleEndian.Uint32(cp.storage[8:12])
	objID := binary.LittleEndian.Uint32(cp.storage[12:16])

	const wantSpaTypeObjectFormat uint32 = 0x40003
	const spaParamEnumFormat uint32 = 3
	if objType != wantSpaTypeObjectFormat {
		t.Fatalf("expected object type SPA_TYPE_OBJECT_Format (0x%x), got 0x%x", wantSpaTypeObjectFormat, objType)
	}
	if objID != spaParamEnumFormat {
		t.Fatalf("expected object id SPA_PARAM_EnumFormat (%d), got %d", spaParamEnumFormat, objID)
	}

	// The POD body size must cover all properties.
	// Object body (8 bytes) + 5 properties × 24 bytes = 128 bytes
	expectedBodySize := uint32(8 + 5*24)
	if podSize != expectedBodySize {
		t.Errorf("expected POD body size %d, got %d", expectedBodySize, podSize)
	}

	// Verify the first property: SPA_FORMAT_mediaType → Id(SPA_MEDIA_TYPE_audio)
	dissectProperty(t, cp.storage[16:40], "mediaType",
		spaFormatMediaType, spaTypeId, uint32(spaMediaTypeAudio))

	// Verify the second property: SPA_FORMAT_mediaSubtype → Id(SPA_MEDIA_SUBTYPE_raw)
	dissectProperty(t, cp.storage[40:64], "mediaSubtype",
		spaFormatMediaSubtype, spaTypeId, uint32(spaMediaSubtypeRaw))

	// Verify the third property: SPA_FORMAT_AUDIO_format → Id(SPA_AUDIO_FORMAT_F32P)
	dissectProperty(t, cp.storage[64:88], "audio.format",
		spaFormatAudioFormat, spaTypeId, spaAudioFormatF32P)

	// Verify the fourth property: SPA_FORMAT_AUDIO_rate → Int(sample_rate)
	dissectIntProperty(t, cp.storage[88:112], "audio.rate",
		spaFormatAudioRate, 48000)

	// Verify the fifth property: SPA_FORMAT_AUDIO_channels → Int(channels)
	dissectIntProperty(t, cp.storage[112:136], "audio.channels",
		spaFormatAudioChannels, 2)
}

// TestBuildRawAudioParamsRejectsInvalidFormat verifies that buildRawAudioParams
// rejects zero or negative format values.
func TestBuildRawAudioParamsRejectsInvalidFormat(t *testing.T) {
	tests := []struct {
		name   string
		fmt    portout.PlaybackFormat
		errMsg string
	}{
		{
			name:   "zero_sample_rate",
			fmt:    portout.PlaybackFormat{SampleRate: 0, Channels: 2, FramesPerBuffer: 1024},
			errMsg: "sample rate",
		},
		{
			name:   "negative_sample_rate",
			fmt:    portout.PlaybackFormat{SampleRate: -1, Channels: 2, FramesPerBuffer: 1024},
			errMsg: "sample rate",
		},
		{
			name:   "zero_channels",
			fmt:    portout.PlaybackFormat{SampleRate: 48000, Channels: 0, FramesPerBuffer: 1024},
			errMsg: "channels",
		},
		{
			name:   "negative_channels",
			fmt:    portout.PlaybackFormat{SampleRate: 48000, Channels: -1, FramesPerBuffer: 1024},
			errMsg: "channels",
		},
		{
			name:   "zero_frames_per_buffer",
			fmt:    portout.PlaybackFormat{SampleRate: 48000, Channels: 2, FramesPerBuffer: 0},
			errMsg: "frames per buffer",
		},
		{
			name:   "negative_frames_per_buffer",
			fmt:    portout.PlaybackFormat{SampleRate: 48000, Channels: 2, FramesPerBuffer: -1},
			errMsg: "frames per buffer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildRawAudioParams(tt.fmt)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
			if !errors.Is(err, ErrInvalidPlaybackFormat) {
				t.Errorf("expected error wrapping ErrInvalidPlaybackFormat, got: %v", err)
			}
		})
	}
}

// TestBuildRawAudioParamsDifferentRates verifies the builder encodes different
// sample rates and channel counts correctly.
func TestBuildRawAudioParamsDifferentRates(t *testing.T) {
	fmt := portout.PlaybackFormat{
		SampleRate:      44100,
		Channels:        1,
		FramesPerBuffer: 512,
	}

	cp, err := buildRawAudioParams(fmt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The fourth property should be Int(44100)
	dissectIntProperty(t, cp.storage[88:112], "audio.rate",
		spaFormatAudioRate, 44100)

	// The fifth property should be Int(1)
	dissectIntProperty(t, cp.storage[112:136], "audio.channels",
		spaFormatAudioChannels, 1)
}

// TestConnectParamsHelpers verifies the connectParams helper methods.
func TestConnectParamsHelpers(t *testing.T) {
	fmt := portout.PlaybackFormat{
		SampleRate:      48000,
		Channels:        2,
		FramesPerBuffer: 1024,
	}

	cp, err := buildRawAudioParams(fmt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cp.Count() != 1 {
		t.Errorf("Count() = %d, want 1", cp.Count())
	}

	ptr := cp.Pointer()
	if ptr == nil {
		t.Error("Pointer() returned nil")
	}

	// Pointer() must return the address of the params slice entry (spa_pod **),
	// NOT the raw spa_pod * that the entry points to.
	expectedParamsAddr := unsafe.Pointer(&cp.params[0])
	if ptr != expectedParamsAddr {
		t.Errorf("Pointer() = %v, want &params[0] = %v", ptr, expectedParamsAddr)
	}

	// Pointer() must NOT equal the storage start (that would be spa_pod *,
	// which is the wrong indirection level for pw_stream_connect).
	storageAddr := unsafe.Pointer(&cp.storage[0])
	if ptr == storageAddr {
		t.Errorf("Pointer() must not equal &storage[0] (%v); it should be one indirection level above", storageAddr)
	}
}

// dissectProperty checks an Id-valued property at the given byte slice.
func dissectProperty(t *testing.T, prop []byte, label string, wantKey uint32, wantType uint32, wantValue uint32) {
	t.Helper()
	key := binary.LittleEndian.Uint32(prop[0:4])
	flags := binary.LittleEndian.Uint32(prop[4:8])
	valPodSize := binary.LittleEndian.Uint32(prop[8:12])
	valPodType := binary.LittleEndian.Uint32(prop[12:16])
	val := binary.LittleEndian.Uint32(prop[16:20])

	if key != wantKey {
		t.Errorf("%s: key = 0x%x, want 0x%x", label, key, wantKey)
	}
	if flags != 0 {
		t.Errorf("%s: flags = %d, want 0", label, flags)
	}
	if valPodSize != 4 {
		t.Errorf("%s: value pod size = %d, want 4", label, valPodSize)
	}
	if valPodType != wantType {
		t.Errorf("%s: value pod type = %d, want %d", label, valPodType, wantType)
	}
	if val != wantValue {
		t.Errorf("%s: value = %d, want %d", label, val, wantValue)
	}
}

// dissectIntProperty checks an Int-valued property at the given byte slice.
func dissectIntProperty(t *testing.T, prop []byte, label string, wantKey uint32, wantValue int32) {
	t.Helper()
	key := binary.LittleEndian.Uint32(prop[0:4])
	flags := binary.LittleEndian.Uint32(prop[4:8])
	valPodSize := binary.LittleEndian.Uint32(prop[8:12])
	valPodType := binary.LittleEndian.Uint32(prop[12:16])
	val := int32(binary.LittleEndian.Uint32(prop[16:20]))

	const spaTypeInt uint32 = 4
	if key != wantKey {
		t.Errorf("%s: key = 0x%x, want 0x%x", label, key, wantKey)
	}
	if flags != 0 {
		t.Errorf("%s: flags = %d, want 0", label, flags)
	}
	if valPodSize != 4 {
		t.Errorf("%s: value pod size = %d, want 4", label, valPodSize)
	}
	if valPodType != spaTypeInt {
		t.Errorf("%s: value pod type = %d, want %d", label, valPodType, spaTypeInt)
	}
	if val != wantValue {
		t.Errorf("%s: value = %d, want %d", label, val, wantValue)
	}
}
