package core

import (
	"errors"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/mock"

	portinmocks "github.com/bnema/purego-pipewire/internal/ports/in/mocks"
	portmocks "github.com/bnema/purego-pipewire/internal/ports/out/mocks"
	"github.com/bnema/purego-pipewire/spa"
)

func TestVideoStructLayouts(t *testing.T) {
	var pb videoPWBuffer
	if unsafe.Offsetof(pb.Buffer) != 0 || unsafe.Offsetof(pb.UserData) != 8 || unsafe.Offsetof(pb.Size) != 16 {
		t.Errorf("pw_buffer layout: user_data@%d size@%d", unsafe.Offsetof(pb.UserData), unsafe.Offsetof(pb.Size))
	}
	var sb spaBuffer
	if unsafe.Offsetof(sb.NMetas) != 0 || unsafe.Offsetof(sb.NDatas) != 4 || unsafe.Offsetof(sb.Metas) != 8 || unsafe.Offsetof(sb.Datas) != 16 {
		t.Errorf("spa_buffer layout wrong")
	}
	var m spaMeta
	if unsafe.Sizeof(m) != 16 || unsafe.Offsetof(m.Data) != 8 {
		t.Errorf("spa_meta layout: size %d", unsafe.Sizeof(m))
	}
	var h spaMetaHeader
	if unsafe.Sizeof(h) != 32 || unsafe.Offsetof(h.PTS) != 8 || unsafe.Offsetof(h.DTSOffset) != 16 || unsafe.Offsetof(h.Seq) != 24 {
		t.Errorf("spa_meta_header layout: size %d", unsafe.Sizeof(h))
	}
	var c spaChunk
	if unsafe.Sizeof(c) != 16 || unsafe.Offsetof(c.Stride) != 8 || unsafe.Offsetof(c.Flags) != 12 {
		t.Errorf("spa_chunk layout wrong")
	}
	var d spaData
	if unsafe.Sizeof(d) != 40 || unsafe.Offsetof(d.Fd) != 8 || unsafe.Offsetof(d.MapOffset) != 16 ||
		unsafe.Offsetof(d.Maxsize) != 20 || unsafe.Offsetof(d.Data) != 24 || unsafe.Offsetof(d.Chunk) != 32 {
		t.Errorf("spa_data layout: size %d", unsafe.Sizeof(d))
	}
	var e videoEvents
	for name, want := range map[string][2]uintptr{
		"state_changed": {unsafe.Offsetof(e.StateChanged), 16},
		"param_changed": {unsafe.Offsetof(e.ParamChanged), 40},
		"add_buffer":    {unsafe.Offsetof(e.AddBuffer), 48},
		"remove_buffer": {unsafe.Offsetof(e.RemoveBuf), 56},
		"process":       {unsafe.Offsetof(e.Process), 64},
		"trigger_done":  {unsafe.Offsetof(e.TriggerDone), 88},
	} {
		if want[0] != want[1] {
			t.Errorf("pw_stream_events.%s at %d, want %d", name, want[0], want[1])
		}
	}
	var s VideoSource
	if unsafe.Sizeof(s.hook) < 48 {
		t.Errorf("spa_hook storage %d bytes, need 48", unsafe.Sizeof(s.hook))
	}
}

// videoRig is a VideoSource wired to mocked C calls and mocked shared memory,
// with a fake negotiated format. C-side structures are Go memory, which is
// enough because the production code only reads them through pointers.
type videoRig struct {
	t       *testing.T
	api     *portmocks.MockVideoStreamAPI
	shm     *portmocks.MockSharedMemAPI
	handler *portinmocks.MockVideoHandler
	src     *VideoSource
	stream  unsafe.Pointer
	streamB [8]byte
}

const rigW, rigH = 4, 2 // stride 16, size 32

func newRig(t *testing.T) *videoRig {
	t.Helper()
	r := &videoRig{
		t:       t,
		api:     portmocks.NewMockVideoStreamAPI(t),
		shm:     portmocks.NewMockSharedMemAPI(t),
		handler: portinmocks.NewMockVideoHandler(t),
	}
	r.stream = unsafe.Pointer(&r.streamB[0])
	core := &Core{ptr: unsafe.Pointer(&r.streamB[4]), capi: r.api, shm: r.shm}

	var gotProps *byte
	r.api.EXPECT().PWPropertiesNewString(mock.Anything).Run(func(args *byte) { gotProps = args }).Return(unsafe.Pointer(&r.streamB[1])).Once()
	r.api.EXPECT().PWStreamNew(core.ptr, mock.Anything, unsafe.Pointer(&r.streamB[1])).Return(r.stream).Once()
	var events *videoEvents
	r.api.EXPECT().PWStreamAddListener(r.stream, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_, _, ev, _ unsafe.Pointer) { events = (*videoEvents)(ev) }).Return(0).Once()
	var connectFlags uint32
	r.api.EXPECT().PWStreamConnect(r.stream, int32(1), uint32(spa.IDInvalid), mock.Anything, mock.Anything, uint32(1)).
		Run(func(_ unsafe.Pointer, _ int32, _ uint32, flags uint32, _ unsafe.Pointer, _ uint32) {
			connectFlags = flags
		}).
		Return(0).Once()

	src, err := core.NewVideoSource(VideoConfig{Name: "test-src", Width: rigW, Height: rigH}, r.handler)
	if err != nil {
		t.Fatal(err)
	}
	r.src = src
	t.Cleanup(func() {
		if r.src.table >= 0 {
			videoTable[r.src.table].Store(nil)
		}
	})

	if connectFlags != pwStreamFlagDriver|pwStreamFlagAllocBuffers {
		t.Fatalf("connect flags = %#x, want DRIVER|ALLOC_BUFFERS", connectFlags)
	}
	if events == nil || events.Version != 2 || events.StateChanged == 0 || events.ParamChanged == 0 ||
		events.AddBuffer == 0 || events.RemoveBuf == 0 || events.Process == 0 || events.TriggerDone == 0 {
		t.Fatalf("events not fully registered: %+v", events)
	}
	if got := cString(unsafe.Pointer(gotProps)); got != `media.class=Video/Source node.name="test-src"` {
		t.Fatalf("props = %q", got)
	}
	if videoTable[src.table].Load() != src {
		t.Fatal("source not in callback slot table")
	}
	return r
}

// negotiate feeds a Format param to the source, as PipeWire would.
func (r *videoRig) negotiate() {
	r.t.Helper()
	var dst [256]byte
	b := spa.NewPodBuilder(dst[:])
	b.Object(spa.TypeObjectFormat, spa.ParamFormat)
	b.Prop(spa.FormatKeyMediaType)
	b.Id(spa.MediaTypeVideo)
	b.Prop(spa.FormatKeyMediaSubtype)
	b.Id(spa.MediaSubtypeRaw)
	b.Prop(spa.FormatKeyVideoFormat)
	b.Id(uint32(spa.VideoFormatBGRx))
	b.Prop(spa.FormatKeyVideoSize)
	b.Rectangle(rigW, rigH)
	b.End()
	pod, err := b.Bytes()
	if err != nil {
		r.t.Fatal(err)
	}
	var params unsafe.Pointer
	r.api.EXPECT().PWStreamUpdateParams(r.stream, mock.Anything, uint32(2)).
		Run(func(_, p unsafe.Pointer, _ uint32) { params = p }).Return(0).Once()
	r.handler.EXPECT().FormatChanged(spa.VideoFormatBGRx, rigW, rigH, rigW*4).Once()
	r.src.onParamChanged(spa.ParamFormat, unsafe.Pointer(&pod[0]))
	if params != unsafe.Pointer(&r.src.params[0]) {
		r.t.Fatal("update_params not given the slot-owned params array")
	}
}

// fakeBuf is a PipeWire-side buffer: pw_buffer + spa_buffer + data + chunk + header meta.
type fakeBuf struct {
	pw    videoPWBuffer
	sb    spaBuffer
	data  spaData
	chunk spaChunk
	meta  spaMeta
	hdr   spaMetaHeader
}

func newFakeBuf(dataType uint32) *fakeBuf {
	f := &fakeBuf{}
	f.data.Type = dataType
	f.data.Chunk = &f.chunk
	f.meta = spaMeta{Type: spa.MetaTypeHeader, Size: spa.MetaHeaderSize, Data: unsafe.Pointer(&f.hdr)}
	f.sb = spaBuffer{NMetas: 1, NDatas: 1, Metas: unsafe.Pointer(&f.meta), Datas: unsafe.Pointer(&f.data)}
	f.pw.Buffer = &f.sb
	return f
}

func (f *fakeBuf) ptr() unsafe.Pointer { return unsafe.Pointer(&f.pw) }

// add runs add_buffer with a memfd that the shm mock "creates".
func (r *videoRig) add(f *fakeBuf, fd int) []byte {
	r.t.Helper()
	mem := make([]byte, rigW*4*rigH)
	r.shm.EXPECT().NewSharedMem("pw-video", rigW*4*rigH).Return(fd, mem, nil).Once()
	r.handler.EXPECT().BuffersChanged().Once()
	r.src.onAddBuffer(f.ptr())
	return mem
}

func TestVideoSourceNegotiationAndAddBuffer(t *testing.T) {
	r := newRig(t)
	r.negotiate()
	f := newFakeBuf(1 << spa.DataMemFd)
	mem := r.add(f, 42)

	if f.data.Type != spa.DataMemFd || f.data.Flags != spa.DataFlagReadWrite || f.data.Fd != 42 ||
		f.data.MapOffset != 0 || f.data.Maxsize != uint32(len(mem)) || f.data.Data != unsafe.Pointer(&mem[0]) {
		t.Fatalf("spa_data not filled: %+v", f.data)
	}
	if f.chunk.Offset != 0 || f.chunk.Size != uint32(len(mem)) || f.chunk.Stride != rigW*4 {
		t.Fatalf("chunk = %+v", f.chunk)
	}
	if f.pw.UserData != 1 {
		t.Fatalf("user_data = %d, want slot 0 + 1", f.pw.UserData)
	}

	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(f.ptr()).Once()
	b := r.src.Dequeue()
	if b == nil || len(b.Data) != len(mem) || &b.Data[0] != &mem[0] || b.Stride != rigW*4 || b.Width != rigW || b.Height != rigH || b.Format != spa.VideoFormatBGRx {
		t.Fatalf("Dequeue = %+v", b)
	}

	var queued unsafe.Pointer
	r.api.EXPECT().PWStreamQueueBuffer(r.stream, f.ptr()).Run(func(_, p unsafe.Pointer) { queued = p }).Return(0).Once()
	r.src.Queue(b, 7, 1500*time.Millisecond, false)
	if queued != f.ptr() {
		t.Fatal("buffer not queued")
	}
	if f.chunk.Size != uint32(len(mem)) || f.chunk.Stride != rigW*4 || f.chunk.Flags != 0 {
		t.Fatalf("chunk after Queue = %+v", f.chunk)
	}
	if f.hdr.Seq != 7 || f.hdr.PTS != int64(1500*time.Millisecond) || f.hdr.Flags != 0 {
		t.Fatalf("header after Queue = %+v", f.hdr)
	}

	// Second round, corrupted.
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(f.ptr()).Once()
	b = r.src.Dequeue()
	if b == nil {
		t.Fatal("buffer not re-dequeued")
	}
	r.api.EXPECT().PWStreamQueueBuffer(r.stream, f.ptr()).Return(0).Once()
	r.src.Queue(b, 8, 0, true)
	if f.chunk.Size != 0 || f.chunk.Flags != spa.ChunkFlagCorrupted || f.hdr.Flags != spa.MetaHeaderFlagCorrupted || f.hdr.Seq != 8 {
		t.Fatalf("corrupted frame: chunk %+v hdr %+v", f.chunk, f.hdr)
	}

	// Trigger is separate and goes straight to the C call.
	r.api.EXPECT().PWStreamTriggerProcess(r.stream).Return(0).Once()
	r.src.Trigger()

	r.expectDestroy(1)
	r.src.Destroy()
}

// expectDestroy expects the disconnect/destroy sequence and the free of n live slots.
func (r *videoRig) expectDestroy(n int) {
	r.api.EXPECT().PWStreamDisconnect(r.stream).Return(0).Once()
	r.api.EXPECT().PWStreamDestroy(r.stream).Once()
	if n > 0 {
		r.shm.EXPECT().FreeSharedMem(mock.Anything, mock.Anything).Times(n)
	}
}

func TestVideoSourceMemfdFailureMakesBufferUnusable(t *testing.T) {
	r := newRig(t)
	r.negotiate()

	bad := newFakeBuf(1 << spa.DataMemFd)
	r.shm.EXPECT().NewSharedMem("pw-video", rigW*4*rigH).Return(-1, nil, errors.New("memfd_create: boom")).Once()
	r.src.onAddBuffer(bad.ptr())
	if bad.pw.UserData != 0 {
		t.Fatalf("failed buffer got user_data %d", bad.pw.UserData)
	}

	good := newFakeBuf(1 << spa.DataMemFd)
	mem := r.add(good, 7)

	// Dequeue skips the unusable buffer and returns the good one.
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(bad.ptr()).Once()
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(good.ptr()).Once()
	b := r.src.Dequeue()
	if b == nil || &b.Data[0] != &mem[0] {
		t.Fatalf("Dequeue = %+v, want the good buffer", b)
	}
	r.api.EXPECT().PWStreamReturnBuffer(r.stream, good.ptr()).Return(0).Once()
	r.src.Release(b)

	// A DMA-BUF-only buffer is also unusable and allocates nothing.
	dma := newFakeBuf(1 << spa.DataDmaBuf)
	r.src.onAddBuffer(dma.ptr())
	if dma.pw.UserData != 0 {
		t.Fatalf("dmabuf buffer got user_data %d", dma.pw.UserData)
	}

	// Nothing is dequeueable and no buffer is available: nil, no panic.
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(nil).Once()
	if b := r.src.Dequeue(); b != nil {
		t.Fatalf("Dequeue = %+v, want nil", b)
	}
	r.expectDestroy(1)
	r.src.Destroy()
}

func TestVideoSourceRemoveBorrowedKeepsMappingUntilRelease(t *testing.T) {
	r := newRig(t)
	r.negotiate()
	f := newFakeBuf(1 << spa.DataMemFd)
	mem := r.add(f, 9)

	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(f.ptr()).Once()
	b := r.src.Dequeue()
	if b == nil {
		t.Fatal("no buffer")
	}

	// PipeWire removes the buffer while the owner still holds it: no unmap yet
	// (FreeSharedMem has no expectation, so a call would fail the test).
	r.handler.EXPECT().BuffersChanged().Once()
	r.src.onRemoveBuffer(f.ptr())
	b.Data[0] = 0xAB // still writable
	if mem[0] != 0xAB || f.pw.UserData != 0 {
		t.Fatalf("mapping not kept: mem[0]=%x user_data=%d", mem[0], f.pw.UserData)
	}

	// A removed slot is never returned by Dequeue (PipeWire no longer owns it,
	// so even if it showed up the source would not hand it out).
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(f.ptr()).Once()
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(nil).Once()
	if got := r.src.Dequeue(); got != nil {
		t.Fatalf("removed buffer dequeued: %+v", got)
	}

	// Queue on a removed slot must not call pw_stream_queue_buffer, and
	// releases the mapping.
	r.shm.EXPECT().FreeSharedMem(9, mem).Once()
	r.api.AssertNotCalled(t, "PWStreamQueueBuffer", mock.Anything, mock.Anything)
	r.src.Queue(b, 1, 0, false)
	r.api.AssertNotCalled(t, "PWStreamQueueBuffer", mock.Anything, mock.Anything)

	// The slot is reusable afterwards.
	f2 := newFakeBuf(1 << spa.DataMemFd)
	r.add(f2, 10)
	if f2.pw.UserData != 1 {
		t.Fatalf("slot not reused, user_data = %d", f2.pw.UserData)
	}
	r.expectDestroy(1)
	r.src.Destroy()
}

func TestVideoSourceRemoveBorrowedThenRelease(t *testing.T) {
	r := newRig(t)
	r.negotiate()
	f := newFakeBuf(1 << spa.DataMemFd)
	mem := r.add(f, 11)
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(f.ptr()).Once()
	b := r.src.Dequeue()

	r.handler.EXPECT().BuffersChanged().Once()
	r.src.onRemoveBuffer(f.ptr())
	r.shm.EXPECT().FreeSharedMem(11, mem).Once()
	r.src.Release(b)
	r.api.AssertNotCalled(t, "PWStreamReturnBuffer", mock.Anything, mock.Anything)

	// A second Release/Queue of the same buffer is ignored.
	r.src.Release(b)
	r.src.Queue(b, 0, 0, false)
	r.expectDestroy(0)
	r.src.Destroy()
}

func TestVideoSourceRemoveIdleUnmapsImmediately(t *testing.T) {
	r := newRig(t)
	r.negotiate()
	f := newFakeBuf(1 << spa.DataMemFd)
	mem := r.add(f, 12)
	r.shm.EXPECT().FreeSharedMem(12, mem).Once()
	r.handler.EXPECT().BuffersChanged().Once()
	r.src.onRemoveBuffer(f.ptr())
	r.expectDestroy(0)
	r.src.Destroy()
}

func TestVideoSourceDestroyKeepsBorrowedMappingUntilRelease(t *testing.T) {
	r := newRig(t)
	r.negotiate()
	f := newFakeBuf(1 << spa.DataMemFd)
	mem := r.add(f, 13)
	r.api.EXPECT().PWStreamDequeueBuffer(r.stream).Return(f.ptr()).Once()
	b := r.src.Dequeue()

	r.expectDestroy(0)
	r.src.Destroy()
	b.Data[1] = 1 // still writable after Destroy
	if mem[1] != 1 {
		t.Fatal("mapping lost on Destroy")
	}
	r.shm.EXPECT().FreeSharedMem(13, mem).Once()
	r.src.Release(b)

	// Destroy is idempotent and Dequeue/Trigger after it are no-ops.
	r.src.Destroy()
	r.src.Trigger()
	if r.src.Dequeue() != nil {
		t.Fatal("Dequeue after Destroy returned a buffer")
	}
}

func TestVideoSourceStateChanged(t *testing.T) {
	r := newRig(t)
	r.api.EXPECT().PWStreamGetNodeId(r.stream).Return(77).Twice()
	r.api.EXPECT().PWStreamIsDriving(r.stream).Return(false).Once()
	r.handler.EXPECT().StateChanged(int32(2), uint32(77), "").Once()
	r.src.onStateChanged(2, nil)
	r.handler.EXPECT().StateChanged(int32(3), uint32(77), "").Once()
	r.src.onStateChanged(3, nil)
	if r.src.NodeID() != 77 || r.src.State() != 3 {
		t.Fatalf("node %d state %d", r.src.NodeID(), r.src.State())
	}

	msg := append([]byte("boom"), 0)
	r.handler.EXPECT().StateChanged(int32(-1), uint32(77), "boom").Once()
	r.src.onStateChanged(-1, unsafe.Pointer(&msg[0]))

	r.expectDestroy(0)
	r.src.Destroy()
}

func TestVideoSourceResize(t *testing.T) {
	r := newRig(t)
	var p unsafe.Pointer
	r.api.EXPECT().PWStreamUpdateParams(r.stream, mock.Anything, uint32(1)).Run(func(_, params unsafe.Pointer, _ uint32) { p = params }).Return(0).Once()
	if err := r.src.Resize(8, 6); err != nil {
		t.Fatal(err)
	}
	if p != unsafe.Pointer(&r.src.params[0]) {
		t.Fatal("params not slot-owned")
	}
	if err := r.src.Resize(0, 6); err == nil {
		t.Fatal("Resize(0, 6) accepted")
	}
	r.api.EXPECT().PWStreamUpdateParams(r.stream, mock.Anything, uint32(1)).Return(-22).Once()
	if err := r.src.Resize(8, 6); err == nil {
		t.Fatal("Resize accepted a failing update_params")
	}
	r.expectDestroy(0)
	r.src.Destroy()
	if err := r.src.Resize(8, 6); err == nil {
		t.Fatal("Resize after Destroy accepted")
	}
}

func TestNewVideoSourceErrors(t *testing.T) {
	api := portmocks.NewMockVideoStreamAPI(t)
	shm := portmocks.NewMockSharedMemAPI(t)
	h := portinmocks.NewMockVideoHandler(t)
	core := &Core{ptr: unsafe.Pointer(new(byte)), capi: api, shm: shm}

	if _, err := core.NewVideoSource(VideoConfig{Width: 0, Height: 10}, h); !errors.Is(err, ErrVideoConfig) {
		t.Errorf("zero width: %v", err)
	}
	if _, err := core.NewVideoSource(VideoConfig{Width: 1 << 15, Height: 1 << 14}, h); !errors.Is(err, ErrVideoConfig) {
		t.Errorf("frame size over int32: %v", err)
	}
	if _, err := core.NewVideoSource(VideoConfig{Width: 10, Height: 10}, nil); !errors.Is(err, ErrVideoConfig) {
		t.Errorf("nil handler: %v", err)
	}
	if _, err := core.NewVideoSource(VideoConfig{Name: `a"b`, Width: 10, Height: 10}, h); !errors.Is(err, ErrVideoConfig) {
		t.Errorf("quote in name: %v", err)
	}
	if _, err := (&Core{}).NewVideoSource(VideoConfig{Width: 10, Height: 10}, h); !errors.Is(err, ErrVideoStreamCreate) {
		t.Errorf("disconnected core: %v", err)
	}

	// properties allocation failure
	api.EXPECT().PWPropertiesNewString(mock.Anything).Return(nil).Once()
	if _, err := core.NewVideoSource(VideoConfig{Width: 10, Height: 10}, h); !errors.Is(err, ErrVideoStreamCreate) {
		t.Errorf("props failure: %v", err)
	}

	// pw_stream_new failure
	props := unsafe.Pointer(new(byte))
	api.EXPECT().PWPropertiesNewString(mock.Anything).Return(props).Once()
	api.EXPECT().PWStreamNew(core.ptr, mock.Anything, props).Return(nil).Once()
	if _, err := core.NewVideoSource(VideoConfig{Width: 10, Height: 10}, h); !errors.Is(err, ErrVideoStreamCreate) {
		t.Errorf("stream_new failure: %v", err)
	}

	// pw_stream_connect failure tears the stream down and frees the slot
	stream := unsafe.Pointer(new(byte))
	api.EXPECT().PWPropertiesNewString(mock.Anything).Return(props).Once()
	api.EXPECT().PWStreamNew(core.ptr, mock.Anything, props).Return(stream).Once()
	api.EXPECT().PWStreamAddListener(stream, mock.Anything, mock.Anything, mock.Anything).Return(0).Once()
	api.EXPECT().PWStreamConnect(stream, int32(1), uint32(spa.IDInvalid), mock.Anything, mock.Anything, uint32(1)).Return(-1).Once()
	api.EXPECT().PWStreamDisconnect(stream).Return(0).Once()
	api.EXPECT().PWStreamDestroy(stream).Once()
	if _, err := core.NewVideoSource(VideoConfig{Width: 10, Height: 10}, h); !errors.Is(err, ErrVideoConnect) {
		t.Errorf("connect failure: %v", err)
	}
	for i := range videoTable {
		if videoTable[i].Load() != nil {
			t.Errorf("callback slot %d leaked", i)
		}
	}
}

func TestNewVideoSourceTableFull(t *testing.T) {
	api := portmocks.NewMockVideoStreamAPI(t)
	h := portinmocks.NewMockVideoHandler(t)
	core := &Core{ptr: unsafe.Pointer(new(byte)), capi: api, shm: portmocks.NewMockSharedMemAPI(t)}
	dummy := &VideoSource{}
	for i := range videoTable {
		videoTable[i].Store(dummy)
	}
	defer func() {
		for i := range videoTable {
			videoTable[i].Store(nil)
		}
	}()
	if _, err := core.NewVideoSource(VideoConfig{Width: 10, Height: 10}, h); !errors.Is(err, ErrVideoTableFull) {
		t.Fatalf("err = %v, want ErrVideoTableFull", err)
	}
}

func TestLoopContextCore(t *testing.T) {
	api := portmocks.NewMockCAPI(t)
	r := NewRuntime(api)

	loopPtr := unsafe.Pointer(new(byte))
	api.EXPECT().PWLoopNew(unsafe.Pointer(nil)).Return(nil).Once()
	if _, err := r.NewLoop(); !errors.Is(err, ErrLoopCreate) {
		t.Fatalf("NewLoop nil: %v", err)
	}
	api.EXPECT().PWLoopNew(unsafe.Pointer(nil)).Return(loopPtr).Once()
	loop, err := r.NewLoop()
	if err != nil {
		t.Fatal(err)
	}

	api.EXPECT().PWLoopGetFd(loopPtr).Return(9).Once()
	if loop.Fd() != 9 {
		t.Fatal("Fd")
	}
	// Iterate: enter, iterate(0), leave in that order; a negative result is not fatal.
	var order []string
	api.EXPECT().PWLoopEnter(loopPtr).Run(func(unsafe.Pointer) { order = append(order, "enter") }).Once()
	api.EXPECT().PWLoopIterate(loopPtr, int32(0)).Run(func(unsafe.Pointer, int32) { order = append(order, "iterate") }).Return(-4).Once()
	api.EXPECT().PWLoopLeave(loopPtr).Run(func(unsafe.Pointer) { order = append(order, "leave") }).Once()
	loop.Iterate()
	if len(order) != 3 || order[0] != "enter" || order[1] != "iterate" || order[2] != "leave" {
		t.Fatalf("Iterate order = %v", order)
	}

	if _, err := r.NewContext(nil); !errors.Is(err, ErrContextCreate) {
		t.Fatalf("NewContext(nil loop): %v", err)
	}
	api.EXPECT().PWContextNew(loopPtr, unsafe.Pointer(nil), uintptr(0)).Return(nil).Once()
	if _, err := r.NewContext(loop); !errors.Is(err, ErrContextCreate) {
		t.Fatalf("NewContext nil ctx: %v", err)
	}
	ctxPtr := unsafe.Pointer(new(byte))
	api.EXPECT().PWContextNew(loopPtr, unsafe.Pointer(nil), uintptr(0)).Return(ctxPtr).Once()
	ctx, err := r.NewContext(loop)
	if err != nil {
		t.Fatal(err)
	}

	api.EXPECT().PWContextConnect(ctxPtr, unsafe.Pointer(nil), uintptr(0)).Return(nil).Once()
	if _, err := ctx.Connect(); !errors.Is(err, ErrCoreConnect) {
		t.Fatalf("Connect nil: %v", err)
	}
	corePtr := unsafe.Pointer(new(byte))
	api.EXPECT().PWContextConnect(ctxPtr, unsafe.Pointer(nil), uintptr(0)).Return(corePtr).Once()
	c, err := ctx.Connect()
	if err != nil {
		t.Fatal(err)
	}
	api.EXPECT().PWCoreDisconnect(corePtr).Return(0).Once()
	c.Disconnect()
	c.Disconnect() // idempotent
	api.EXPECT().PWContextDestroy(ctxPtr).Once()
	ctx.Destroy()
	ctx.Destroy()
	if _, err := ctx.Connect(); err == nil {
		t.Fatal("Connect after Destroy accepted")
	}
	api.EXPECT().PWLoopDestroy(loopPtr).Once()
	loop.Destroy()
	loop.Destroy()
}
