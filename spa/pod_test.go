package spa

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// Golden vectors are the bytes produced by the SPA reference builder
// (spa/pod/builder.h, libspa 0.2 / PipeWire 1.6.9) for the same parameters
// via spa_pod_builder_add_object / SPA_POD_CHOICE_* on amd64.
const (
	goldenEnumFormatSingle = "b80000000f00000003000400030000000100000000000000040000000300000002000000000000000200000000000000040000000300000001000000000000000100020000000000040000000300000008000000000000000300020000000000080000000a00000040010000f00000000400020000000000080000000b0000000000000001000000050002000000000028000000130000000100000000000000080000000b0000003c0000000100000001000000010000003c00000001000000"
	goldenEnumFormatMulti  = "d00000000f000000030004000300000001000000000000000400000003000000020000000000000002000000000000000400000003000000010000000000000001000200000000001c000000130000000300000000000000040000000300000008000000080000000c000000000000000300020000000000080000000a00000080070000380400000400020000000000080000000b0000000000000001000000050002000000000028000000130000000100000000000000080000000b000000900000000100000001000000010000009000000001000000"
	goldenBuffers          = "c00000000f000000040004000500000001000000000000001c0000001300000001000000000000000400000004000000080000000200000010000000000000000200000000000000040000000400000001000000000000000300000000000000040000000400000000b004000000000004000000000000000400000004000000000500000000000005000000000000000400000004000000100000000000000006000000000000001400000013000000040000000000000004000000040000000400000000000000"
	goldenMetaHeader       = "380000000f0000000500040006000000010000000000000004000000030000000100000000000000020000000000000004000000040000002000000000000000"
)

func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestVideoEnumFormatGoldenSingle(t *testing.T) {
	var dst [512]byte
	got, err := VideoEnumFormat(dst[:], 320, 240, 60, VideoFormatBGRx)
	if err != nil {
		t.Fatal(err)
	}
	if want := mustHex(t, goldenEnumFormatSingle); hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

func TestVideoEnumFormatGoldenMulti(t *testing.T) {
	var dst [512]byte
	got, err := VideoEnumFormat(dst[:], 1920, 1080, 144, VideoFormatBGRx, VideoFormatBGRA)
	if err != nil {
		t.Fatal(err)
	}
	if want := mustHex(t, goldenEnumFormatMulti); hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

func TestVideoBuffersGolden(t *testing.T) {
	var dst [512]byte
	got, err := VideoBuffers(dst[:], 320*240*4, 320*4, 2, 8, 16, 1<<DataMemFd)
	if err != nil {
		t.Fatal(err)
	}
	if want := mustHex(t, goldenBuffers); hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

func TestMetaHeaderGolden(t *testing.T) {
	var dst [128]byte
	got, err := MetaHeader(dst[:])
	if err != nil {
		t.Fatal(err)
	}
	if want := mustHex(t, goldenMetaHeader); hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

// formatPod builds a negotiated (non-choice) Format object for parser tests.
func formatPod(t testing.TB, dst []byte, format VideoFormat, w, h uint32) []byte {
	t.Helper()
	b := NewPodBuilder(dst)
	b.Object(TypeObjectFormat, ParamFormat)
	b.Prop(FormatKeyMediaType)
	b.Id(MediaTypeVideo)
	b.Prop(FormatKeyMediaSubtype)
	b.Id(MediaSubtypeRaw)
	b.Prop(FormatKeyVideoFormat)
	b.Id(uint32(format))
	b.Prop(FormatKeyVideoSize)
	b.Rectangle(w, h)
	b.Prop(FormatKeyVideoFramerate)
	b.Fraction(60, 1)
	b.End()
	pod, err := b.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return pod
}

func TestParseVideoFormatRoundTrip(t *testing.T) {
	var dst [256]byte
	for _, tc := range []struct {
		f    VideoFormat
		w, h int
	}{{VideoFormatBGRx, 320, 240}, {VideoFormatBGRA, 3840, 2160}} {
		pod := formatPod(t, dst[:], tc.f, uint32(tc.w), uint32(tc.h))
		f, w, h, ok := ParseVideoFormat(pod)
		if !ok || VideoFormat(f) != tc.f || w != tc.w || h != tc.h {
			t.Fatalf("parse = %d %d %d %v, want %d %d %d", f, w, h, ok, tc.f, tc.w, tc.h)
		}
	}
}

func TestParseVideoFormatRejectsMalformed(t *testing.T) {
	var dst [256]byte
	pod := append([]byte(nil), formatPod(t, dst[:], VideoFormatBGRx, 320, 240)...)

	// Every truncation must be rejected without panicking.
	for n := 0; n < len(pod); n++ {
		if _, _, _, ok := ParseVideoFormat(pod[:n]); ok {
			t.Fatalf("truncated to %d bytes accepted", n)
		}
	}
	// Non-object, wrong object type, oversized declared size.
	bad := append([]byte(nil), pod...)
	binary.LittleEndian.PutUint32(bad[4:], TypeInt)
	if _, _, _, ok := ParseVideoFormat(bad); ok {
		t.Fatal("non-object accepted")
	}
	bad = append([]byte(nil), pod...)
	binary.LittleEndian.PutUint32(bad[8:], TypeObjectParamBuffers)
	if _, _, _, ok := ParseVideoFormat(bad); ok {
		t.Fatal("wrong object type accepted")
	}
	bad = append([]byte(nil), pod...)
	binary.LittleEndian.PutUint32(bad[0:], 1<<30)
	if _, _, _, ok := ParseVideoFormat(bad); ok {
		t.Fatal("oversized pod accepted")
	}
	// Property value size overflowing the object.
	bad = append([]byte(nil), pod...)
	binary.LittleEndian.PutUint32(bad[16+8:], 1<<31)
	if _, _, _, ok := ParseVideoFormat(bad); ok {
		t.Fatal("oversized property accepted")
	}
	// Zero and absurd sizes.
	if _, _, _, ok := ParseVideoFormat(formatPod(t, dst[:], VideoFormatBGRx, 0, 240)); ok {
		t.Fatal("zero width accepted")
	}
	if _, _, _, ok := ParseVideoFormat(formatPod(t, dst[:], VideoFormatBGRx, 1<<24, 240)); ok {
		t.Fatal("huge width accepted")
	}
	// An EnumFormat with a choice for the format is not a negotiated format.
	var big [512]byte
	enum, err := VideoEnumFormat(big[:], 320, 240, 60, VideoFormatBGRx, VideoFormatBGRA)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := ParseVideoFormat(enum); ok {
		t.Fatal("choice format accepted")
	}
}

func TestPodBuilderErrors(t *testing.T) {
	// Buffer too small: error, no panic, for every size up to the full pod.
	var full [512]byte
	pod, err := VideoEnumFormat(full[:], 320, 240, 60, VideoFormatBGRx, VideoFormatBGRA)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < len(pod); n++ {
		small := make([]byte, n)
		if _, err := VideoEnumFormat(small, 320, 240, 60, VideoFormatBGRx, VideoFormatBGRA); err != ErrPodFull {
			t.Fatalf("size %d: err = %v, want ErrPodFull", n, err)
		}
	}
	if _, err := VideoBuffers(make([]byte, 16), 1, 1, 2, 8, 16, 4); err != ErrPodFull {
		t.Fatalf("buffers small: %v", err)
	}
	if _, err := MetaHeader(make([]byte, 8)); err != ErrPodFull {
		t.Fatalf("meta small: %v", err)
	}
	// Invalid arguments.
	if _, err := VideoEnumFormat(full[:], 0, 240, 60, VideoFormatBGRx); err == nil {
		t.Fatal("zero width accepted")
	}
	if _, err := VideoEnumFormat(full[:], 320, 240, 60); err == nil {
		t.Fatal("no formats accepted")
	}
	if _, err := VideoBuffers(full[:], 0, 4, 2, 8, 16, 4); err == nil {
		t.Fatal("zero size accepted")
	}
	// Unbalanced builder use.
	b := NewPodBuilder(full[:])
	b.Prop(1)
	if _, err := b.Bytes(); err != ErrPodState {
		t.Fatalf("prop outside object: %v", err)
	}
	b = NewPodBuilder(full[:])
	b.Object(TypeObjectFormat, ParamFormat)
	if _, err := b.Bytes(); err != ErrPodState {
		t.Fatalf("open object: %v", err)
	}
	b = NewPodBuilder(full[:])
	b.End()
	if _, err := b.Bytes(); err != ErrPodState {
		t.Fatalf("end without object: %v", err)
	}
}

func TestPodAllocations(t *testing.T) {
	var dst [512]byte
	pod := append([]byte(nil), formatPod(t, dst[:], VideoFormatBGRx, 320, 240)...)
	allocs := testing.AllocsPerRun(200, func() {
		e, _ := VideoEnumFormat(dst[:], 320, 240, 60, VideoFormatBGRx, VideoFormatBGRA)
		_, _ = VideoBuffers(dst[:], 307200, 1280, 2, 8, 16, 1<<DataMemFd)
		_, _ = MetaHeader(dst[:])
		_ = e
		_, _, _, _ = ParseVideoFormat(pod)
	})
	if allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}
