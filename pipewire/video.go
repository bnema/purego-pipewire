package pipewire

import (
	"github.com/bnema/purego-pipewire/internal/core"
	"github.com/bnema/purego-pipewire/spa"
)

// VideoFormat is a SPA raw video format (BGRx=8, BGRA=12).
type VideoFormat = spa.VideoFormat

// Supported video formats. They map to wl_shm XRGB8888 (BGRx) and ARGB8888
// (BGRA).
const (
	VideoFormatBGRx = spa.VideoFormatBGRx
	VideoFormatBGRA = spa.VideoFormatBGRA
)

// pw_stream_state values reported to VideoHandler.StateChanged.
const (
	StreamStateError       int32 = -1
	StreamStateUnconnected int32 = 0
	StreamStateConnecting  int32 = 1
	StreamStatePaused      int32 = 2
	StreamStateStreaming   int32 = 3
)

// Errors returned by NewVideoSource and VideoSource, for errors.Is.
var (
	ErrVideoConfig       = core.ErrVideoConfig
	ErrVideoTableFull    = core.ErrVideoTableFull
	ErrVideoStreamCreate = core.ErrVideoStreamCreate
	ErrVideoConnect      = core.ErrVideoConnect
	ErrVideoResize       = core.ErrVideoResize
	ErrVideoBorrowed     = core.ErrVideoBorrowed
)

// VideoConfig configures a VideoSource: the node name, the initial size, the
// maximum frame rate (60 when zero) and the accepted formats (BGRx when
// empty, else BGRx and/or BGRA; the first is preferred).
type VideoConfig = core.VideoConfig

// VideoHandler receives VideoSource events. Every method runs on the loop
// thread, inside Loop.Iterate (or, for the first events, inside the call that
// triggered them). Implementations must not block and must not allocate.
type VideoHandler interface {
	// StateChanged reports a stream state change; see the StreamState
	// constants. nodeID is the PipeWire node id once known (0 before). err
	// carries the message when state is StreamStateError.
	StateChanged(state int32, nodeID uint32, err string)
	// FormatChanged reports the negotiated format and its row stride.
	FormatChanged(f VideoFormat, w, h, stride int)
	// BuffersChanged is called after a buffer was added or removed;
	// Dequeue may now return a buffer.
	BuffersChanged()
}

// VideoBuffer is a buffer handed out by VideoSource.Dequeue. Data aliases a
// mapped memfd owned by the source; it is valid until the buffer is given back
// with Queue or Release. The value is owned by the source: do not copy it or
// keep it after giving it back.
type VideoBuffer = core.VideoBuffer

// VideoSource is a PipeWire Video/Source node that drives the graph and
// exposes shared-memory (memfd) buffers. All methods run on the loop thread.
//
// Produce a frame with Dequeue, write Data, Queue, then Trigger — in that
// order: the buffer must be queued before Trigger. A steady-state cycle
// performs no heap allocation.
//
//	b := src.Dequeue()           // nil: no buffer is free
//	// ... fill b.Data (b.Stride bytes per row) ...
//	src.Queue(b, seq, pts, false)
//	src.Trigger()
//
// Buffers PipeWire removes (renegotiation, Resize, Destroy, consumer
// disconnect) stay mapped while borrowed and are unmapped when given back with
// Queue or Release. Resize fails with ErrVideoBorrowed while a buffer is
// borrowed; Destroy is best called when none is. DMA-BUF is not supported.
type VideoSource = core.VideoSource
