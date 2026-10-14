package core

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/bnema/purego"

	portin "github.com/bnema/purego-pipewire/internal/ports/in"
	portout "github.com/bnema/purego-pipewire/internal/ports/out"
	"github.com/bnema/purego-pipewire/spa"
)

// Video source limits. They are fixed so that nothing is allocated after
// NewVideoSource except the memfd mappings made in add_buffer.
const (
	videoMaxBuffers = 16 // Buffers param maximum, size of the buffer slot table
	videoMaxSources = 64 // size of the process-wide callback slot table
	videoPodSize    = 512

	pwStreamFlagDriver       = 0x8
	pwStreamFlagAllocBuffers = 0x100
	pwStreamStateStreaming   = 3
	pwStreamStatePaused      = 2

	defaultVideoFPS = 60
)

var (
	ErrVideoConfig       = errors.New("invalid video source config")
	ErrVideoTableFull    = errors.New("too many video sources")
	ErrVideoStreamCreate = errors.New("failed to create video stream")
	ErrVideoConnect      = errors.New("failed to connect video stream")
	ErrVideoResize       = errors.New("failed to resize video stream")
)

// VideoConfig configures a VideoSource. MaxFPS defaults to 60 and Formats to
// BGRx.
type VideoConfig struct {
	Name          string
	Width, Height int
	MaxFPS        int
	Formats       []spa.VideoFormat
}

// VideoBuffer is a buffer that was handed out by VideoSource.Dequeue. Data
// aliases a memfd mapping owned by the source: it stays valid until the buffer
// is given back with Queue or Release, and not afterwards. The structure is
// owned by the source; do not copy it, retain it or modify it.
type VideoBuffer struct {
	Index                 int
	Data                  []byte
	Stride, Width, Height int
	Format                spa.VideoFormat
}

// C layouts (amd64/arm64). Offsets are asserted in tests.

// videoPWBuffer mirrors struct pw_buffer; UserData carries slot index+1.
type videoPWBuffer struct {
	Buffer   *spaBuffer
	UserData uintptr
	Size     uint64
}

// spaMeta mirrors struct spa_meta.
type spaMeta struct {
	Type uint32
	Size uint32
	Data unsafe.Pointer
}

// spaMetaHeader mirrors struct spa_meta_header.
type spaMetaHeader struct {
	Flags     uint32
	Offset    uint32
	PTS       int64
	DTSOffset int64
	Seq       uint64
}

// videoEvents mirrors struct pw_stream_events (version 2).
type videoEvents struct {
	Version      uint32
	Destroy      uintptr
	StateChanged uintptr
	ControlInfo  uintptr
	IOChanged    uintptr
	ParamChanged uintptr
	AddBuffer    uintptr
	RemoveBuf    uintptr
	Process      uintptr
	Drained      uintptr
	Command      uintptr
	TriggerDone  uintptr
}

type videoSlot struct {
	buf      VideoBuffer
	pw       unsafe.Pointer // struct pw_buffer; nil once removed
	hdr      *spaMetaHeader // nil if the buffer has no header meta
	chunk    *spaChunk
	fd       int
	mem      []byte
	live     bool // slot holds a mapped buffer
	removed  bool // PipeWire removed the buffer while it was borrowed
	borrowed bool
}

// VideoSource is a PipeWire Video/Source node that drives the graph and
// exposes memfd-backed buffers. Every method, and every handler callback, runs
// on the loop thread: the goroutine (locked to its OS thread) that calls
// Loop.Iterate.
//
// Frames are produced with Dequeue, then writing Data, then Queue, then
// Trigger. The buffer must be queued before Trigger, because the process
// callback does nothing: PipeWire picks up queued buffers when triggered.
type VideoSource struct {
	capi    portout.VideoStreamAPI
	shm     portout.SharedMemAPI
	handler portin.VideoHandler

	stream   unsafe.Pointer
	table    int // index in videoTable
	events   videoEvents
	hook     [8]uint64 // struct spa_hook, 48 bytes
	params   [2]unsafe.Pointer
	enumPod  [videoPodSize]byte
	buffPod  [videoPodSize]byte
	metaPod  [128]byte
	cfg      VideoConfig
	formats  [4]spa.VideoFormat
	nformats int

	state      int32
	nodeID     uint32
	format     spa.VideoFormat
	width      int
	height     int
	stride     int
	negotiated bool
	closed     bool
	warned     bool

	slots [videoMaxBuffers]videoSlot
}

var (
	videoTable [videoMaxSources]atomic.Pointer[VideoSource]

	videoCBOnce sync.Once
	videoCB     struct {
		stateChanged, paramChanged, addBuffer, removeBuffer, process, triggerDone uintptr
	}
)

// uptr turns a raw callback argument into a pointer without a
// uintptr-to-unsafe.Pointer conversion; the value is C memory or a small
// integer, never Go memory.
func uptr(v uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&v)) }

func videoLookup(a *purego.CallbackArgs) *VideoSource {
	idx := int(a.Int(0)) - 1
	if uint(idx) >= videoMaxSources {
		return nil
	}
	return videoTable[idx].Load()
}

func registerVideoCallbacks() {
	videoCBOnce.Do(func() {
		videoCB.stateChanged = purego.NewCallbackInts(func(a *purego.CallbackArgs) uintptr {
			if s := videoLookup(a); s != nil {
				s.onStateChanged(int32(a.Int(2)), uptr(a.Int(3)))
			}
			return 0
		})
		videoCB.paramChanged = purego.NewCallbackInts(func(a *purego.CallbackArgs) uintptr {
			if s := videoLookup(a); s != nil {
				s.onParamChanged(uint32(a.Int(1)), uptr(a.Int(2)))
			}
			return 0
		})
		videoCB.addBuffer = purego.NewCallbackInts(func(a *purego.CallbackArgs) uintptr {
			if s := videoLookup(a); s != nil {
				s.onAddBuffer(uptr(a.Int(1)))
			}
			return 0
		})
		videoCB.removeBuffer = purego.NewCallbackInts(func(a *purego.CallbackArgs) uintptr {
			if s := videoLookup(a); s != nil {
				s.onRemoveBuffer(uptr(a.Int(1)))
			}
			return 0
		})
		// process and trigger_done intentionally do nothing: buffers are
		// queued by the owner before Trigger.
		videoCB.process = purego.NewCallbackInts(func(*purego.CallbackArgs) uintptr { return 0 })
		videoCB.triggerDone = purego.NewCallbackInts(func(*purego.CallbackArgs) uintptr { return 0 })
	})
}

// NewVideoSource creates and connects a Video/Source stream on the core. The
// stream drives the graph (DRIVER) and allocates its own memfd buffers
// (ALLOC_BUFFERS). Call it, and every other method, on the loop thread.
func (c *Core) NewVideoSource(cfg VideoConfig, h portin.VideoHandler) (*VideoSource, error) {
	if c.ptr == nil || c.capi == nil {
		return nil, ErrVideoStreamCreate
	}
	if h == nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 1<<15 || cfg.Height > 1<<15 {
		return nil, ErrVideoConfig
	}
	if cfg.MaxFPS <= 0 {
		cfg.MaxFPS = defaultVideoFPS
	}
	s := &VideoSource{capi: c.capi, shm: c.shm, handler: h, cfg: cfg, table: -1}
	if len(cfg.Formats) == 0 {
		s.formats[0], s.nformats = spa.VideoFormatBGRx, 1
	} else {
		if len(cfg.Formats) > len(s.formats) {
			return nil, ErrVideoConfig
		}
		s.nformats = copy(s.formats[:], cfg.Formats)
	}
	s.cfg.Formats = nil // the slot-owned array is the source of truth
	name := cfg.Name
	if name == "" {
		name = "purego-pipewire"
	}
	if strings.ContainsAny(name, "\"\\") || strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return nil, fmt.Errorf("%w: invalid name %q", ErrVideoConfig, name)
	}
	if _, err := spa.VideoEnumFormat(s.enumPod[:], cfg.Width, cfg.Height, cfg.MaxFPS, s.formats[:s.nformats]...); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVideoConfig, err)
	}

	registerVideoCallbacks()
	for i := range videoTable {
		if videoTable[i].CompareAndSwap(nil, s) {
			s.table = i
			break
		}
	}
	if s.table < 0 {
		return nil, ErrVideoTableFull
	}

	cname := append([]byte(name), 0)
	props := append([]byte(`media.class=Video/Source node.name="`+name+`"`), 0)
	propsPtr := c.capi.PWPropertiesNewString(&props[0])
	if propsPtr == nil {
		videoTable[s.table].Store(nil)
		return nil, ErrVideoStreamCreate
	}
	s.stream = c.capi.PWStreamNew(c.ptr, &cname[0], propsPtr) // takes ownership of props
	if s.stream == nil {
		videoTable[s.table].Store(nil)
		return nil, ErrVideoStreamCreate
	}

	s.events = videoEvents{
		Version:      2,
		StateChanged: videoCB.stateChanged,
		ParamChanged: videoCB.paramChanged,
		AddBuffer:    videoCB.addBuffer,
		RemoveBuf:    videoCB.removeBuffer,
		Process:      videoCB.process,
		TriggerDone:  videoCB.triggerDone,
	}
	data := uptr(uintptr(s.table + 1))
	c.capi.PWStreamAddListener(s.stream, unsafe.Pointer(&s.hook[0]), unsafe.Pointer(&s.events), data)

	s.params[0] = unsafe.Pointer(&s.enumPod[0])
	ret := c.capi.PWStreamConnect(s.stream, 1 /* PW_DIRECTION_OUTPUT */, spa.IDInvalid,
		pwStreamFlagDriver|pwStreamFlagAllocBuffers, unsafe.Pointer(&s.params[0]), 1)
	if ret < 0 {
		s.closed = true
		s.teardown()
		return nil, fmt.Errorf("%w: pw_stream_connect returned %d", ErrVideoConnect, ret)
	}
	return s, nil
}

// NodeID returns the PipeWire node id, or 0 until the stream is paused or
// streaming.
func (s *VideoSource) NodeID() uint32 { return s.nodeID }

// State returns the last stream state (see VideoHandler.StateChanged).
func (s *VideoSource) State() int32 { return s.state }

// Dequeue returns a free buffer to draw into, or nil when none is available.
// The buffer is borrowed until Queue or Release. Buffers that could not be
// given a memfd are skipped (they stay out of the queue).
func (s *VideoSource) Dequeue() *VideoBuffer {
	if s.stream == nil || !s.negotiated {
		return nil
	}
	for i := 0; i < 2*videoMaxBuffers; i++ {
		p := s.capi.PWStreamDequeueBuffer(s.stream)
		if p == nil {
			return nil
		}
		idx := int((*videoPWBuffer)(p).UserData) - 1
		if uint(idx) >= videoMaxBuffers {
			continue // unusable buffer: never handed out
		}
		sl := &s.slots[idx]
		if !sl.live || sl.removed || sl.borrowed || sl.pw != p {
			continue
		}
		sl.borrowed = true
		return &sl.buf
	}
	return nil
}

// slotOf validates b against the slot table.
func (s *VideoSource) slotOf(b *VideoBuffer) *videoSlot {
	if b == nil || uint(b.Index) >= videoMaxBuffers {
		return nil
	}
	sl := &s.slots[b.Index]
	if &sl.buf != b || !sl.live || !sl.borrowed {
		return nil
	}
	return sl
}

// Queue hands a filled buffer back to PipeWire with its frame sequence and
// presentation time. When corrupted is set the frame is flagged corrupted and
// carries no data. Call Trigger after Queue: the buffer is queued before the
// graph cycle is triggered. Queue on a buffer PipeWire already removed only
// releases it.
func (s *VideoSource) Queue(b *VideoBuffer, seq uint64, pts time.Duration, corrupted bool) {
	sl := s.slotOf(b)
	if sl == nil {
		return
	}
	if sl.removed || s.stream == nil {
		s.freeSlot(sl)
		return
	}
	size := uint32(sl.buf.Stride * sl.buf.Height)
	if c := sl.chunk; c != nil {
		c.Offset, c.Size, c.Stride, c.Flags = 0, size, int32(sl.buf.Stride), 0
		if corrupted {
			c.Size, c.Flags = 0, spa.ChunkFlagCorrupted
		}
	}
	if h := sl.hdr; h != nil {
		h.Flags, h.Offset, h.PTS, h.DTSOffset, h.Seq = 0, 0, int64(pts), 0, seq
		if corrupted {
			h.Flags = spa.MetaHeaderFlagCorrupted
		}
	}
	sl.borrowed = false
	s.capi.PWStreamQueueBuffer(s.stream, sl.pw)
}

// Release gives a dequeued buffer back without queueing a frame. If PipeWire
// removed the buffer in the meantime its memory is unmapped now. b.Data must
// not be used afterwards.
func (s *VideoSource) Release(b *VideoBuffer) {
	sl := s.slotOf(b)
	if sl == nil {
		return
	}
	if sl.removed || s.stream == nil {
		s.freeSlot(sl)
		return
	}
	sl.borrowed = false
	s.capi.PWStreamReturnBuffer(s.stream, sl.pw)
}

// Trigger starts a graph cycle (pw_stream_trigger_process). Queue the filled
// buffer first.
func (s *VideoSource) Trigger() {
	if s.stream != nil {
		s.capi.PWStreamTriggerProcess(s.stream)
	}
}

// Resize renegotiates the stream with a new size. Buffers PipeWire replaces
// stay valid while borrowed; hand them back with Queue or Release (ideally
// resize when none is borrowed).
func (s *VideoSource) Resize(w, h int) error {
	if s.stream == nil {
		return ErrVideoResize
	}
	if w <= 0 || h <= 0 || w > 1<<15 || h > 1<<15 {
		return ErrVideoConfig
	}
	if _, err := spa.VideoEnumFormat(s.enumPod[:], w, h, s.cfg.MaxFPS, s.formats[:s.nformats]...); err != nil {
		return fmt.Errorf("%w: %v", ErrVideoResize, err)
	}
	s.cfg.Width, s.cfg.Height = w, h
	s.params[0] = unsafe.Pointer(&s.enumPod[0])
	if ret := s.capi.PWStreamUpdateParams(s.stream, unsafe.Pointer(&s.params[0]), 1); ret < 0 {
		return fmt.Errorf("%w: pw_stream_update_params returned %d", ErrVideoResize, ret)
	}
	return nil
}

// Destroy disconnects and destroys the stream. PipeWire removes every buffer
// during the disconnect; buffers that are still borrowed keep their mapping
// until Release (or Queue). Call Destroy only when no buffer is borrowed,
// unless the caller releases them afterwards. No handler callback runs after
// Destroy starts. Destroy is idempotent.
func (s *VideoSource) Destroy() {
	s.closed = true
	s.teardown()
}

func (s *VideoSource) teardown() {
	if s.stream != nil {
		stream := s.stream
		s.capi.PWStreamDisconnect(stream)
		s.capi.PWStreamDestroy(stream)
		s.stream = nil
	}
	if s.table >= 0 {
		videoTable[s.table].Store(nil)
		s.table = -1
	}
	for i := range s.slots {
		sl := &s.slots[i]
		if !sl.live {
			continue
		}
		sl.pw, sl.hdr, sl.chunk = nil, nil, nil
		if sl.borrowed {
			sl.removed = true
		} else {
			s.freeSlot(sl)
		}
	}
}

// freeSlot unmaps the slot's memory and makes the slot reusable.
func (s *VideoSource) freeSlot(sl *videoSlot) {
	if sl.live {
		s.shm.FreeSharedMem(sl.fd, sl.mem)
	}
	*sl = videoSlot{}
}

// --- stream event handlers (loop thread) ---

func (s *VideoSource) onStateChanged(state int32, errp unsafe.Pointer) {
	s.state = state
	if state == pwStreamStatePaused || state == pwStreamStateStreaming {
		s.nodeID = s.capi.PWStreamGetNodeId(s.stream)
	}
	if state == pwStreamStateStreaming && !s.warned {
		s.warned = true
		if !s.capi.PWStreamIsDriving(s.stream) {
			slog.Warn("purego-pipewire: video stream is not driving the graph", "node", s.nodeID)
		}
	}
	if s.closed {
		return
	}
	msg := ""
	if state < 0 {
		msg = cString(errp)
	}
	s.handler.StateChanged(state, s.nodeID, msg)
}

func cString(p unsafe.Pointer) string {
	if p == nil {
		return ""
	}
	n := 0
	for n < 512 && *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(p), n))
}

func (s *VideoSource) onParamChanged(id uint32, param unsafe.Pointer) {
	if id != spa.ParamFormat || s.stream == nil {
		return
	}
	if param == nil {
		s.negotiated = false
		return
	}
	size := int(*(*uint32)(param))
	if size < 8 || size > 4096 {
		return
	}
	pod := unsafe.Slice((*byte)(param), 8+size)
	f, w, h, ok := spa.ParseVideoFormat(pod)
	if !ok {
		return
	}
	format := spa.VideoFormat(f)
	allowed := false
	for _, af := range s.formats[:s.nformats] {
		allowed = allowed || af == format
	}
	if !allowed {
		return
	}
	stride := w * 4
	buffers, err := spa.VideoBuffers(s.buffPod[:], stride*h, stride, 2, 8, videoMaxBuffers, 1<<spa.DataMemFd)
	if err != nil {
		return
	}
	meta, err := spa.MetaHeader(s.metaPod[:])
	if err != nil {
		return
	}
	s.format, s.width, s.height, s.stride, s.negotiated = format, w, h, stride, true
	s.params[0] = unsafe.Pointer(&buffers[0])
	s.params[1] = unsafe.Pointer(&meta[0])
	s.capi.PWStreamUpdateParams(s.stream, unsafe.Pointer(&s.params[0]), 2)
	if !s.closed {
		s.handler.FormatChanged(format, w, h, stride)
	}
}

func (s *VideoSource) onAddBuffer(p unsafe.Pointer) {
	if p == nil {
		return
	}
	pb := (*videoPWBuffer)(p)
	pb.UserData = 0 // unusable until a slot is assigned
	sb := pb.Buffer
	if sb == nil || sb.NDatas == 0 || sb.Datas == nil || !s.negotiated {
		return
	}
	d := (*spaData)(sb.Datas)
	if d.Type&(1<<spa.DataMemFd) == 0 {
		return // DMA-BUF only: not supported
	}
	idx := -1
	for i := range s.slots {
		if !s.slots[i].live {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	size := s.stride * s.height
	fd, mem, err := s.shm.NewSharedMem("pw-video", size)
	if err != nil || len(mem) < size {
		if err == nil {
			s.shm.FreeSharedMem(fd, mem)
		}
		return
	}
	d.Type, d.Flags, d.Fd = spa.DataMemFd, spa.DataFlagReadWrite, int64(fd)
	d.MapOffset, d.Maxsize, d.Data = 0, uint32(size), unsafe.Pointer(&mem[0])
	if d.Chunk != nil {
		d.Chunk.Offset, d.Chunk.Size, d.Chunk.Stride, d.Chunk.Flags = 0, uint32(size), int32(s.stride), 0
	}
	sl := &s.slots[idx]
	*sl = videoSlot{
		buf:   VideoBuffer{Index: idx, Data: mem, Stride: s.stride, Width: s.width, Height: s.height, Format: s.format},
		pw:    p,
		chunk: d.Chunk,
		fd:    fd,
		mem:   mem,
		live:  true,
	}
	if sb.Metas != nil {
		metas := unsafe.Slice((*spaMeta)(sb.Metas), int(sb.NMetas))
		for i := range metas {
			if metas[i].Type == spa.MetaTypeHeader && metas[i].Size >= spa.MetaHeaderSize && metas[i].Data != nil {
				sl.hdr = (*spaMetaHeader)(metas[i].Data)
				break
			}
		}
	}
	pb.UserData = uintptr(idx + 1)
	if !s.closed {
		s.handler.BuffersChanged()
	}
}

func (s *VideoSource) onRemoveBuffer(p unsafe.Pointer) {
	if p == nil {
		return
	}
	pb := (*videoPWBuffer)(p)
	idx := int(pb.UserData) - 1
	if uint(idx) >= videoMaxBuffers {
		return
	}
	sl := &s.slots[idx]
	if !sl.live || sl.pw != p {
		return
	}
	pb.UserData = 0
	if sl.borrowed {
		// The owner may still write into the mapping: keep it until Release.
		sl.removed = true
		sl.pw, sl.hdr, sl.chunk = nil, nil, nil
	} else {
		s.freeSlot(sl)
	}
	if !s.closed {
		s.handler.BuffersChanged()
	}
}
