package spa

// Media type constants for SPA media types.
const (
	MediaTypeUnknown     = 0
	MediaTypeAudio       = 1
	MediaTypeVideo       = 2
	MediaTypeImage       = 3
	MediaTypeBinary      = 4
	MediaTypeStream      = 5
	MediaTypeApplication = 6
)

// Direction constants for port directions.
const (
	DirectionInput  = 0
	DirectionOutput = 1
)

// SPA pod types (spa/utils/type.h).
const (
	TypeId        = 3
	TypeInt       = 4
	TypeRectangle = 10
	TypeFraction  = 11
	TypeObject    = 15
	TypeChoice    = 19

	TypeObjectFormat       = 0x40003 // SPA_TYPE_OBJECT_Format
	TypeObjectParamBuffers = 0x40004 // SPA_TYPE_OBJECT_ParamBuffers
	TypeObjectParamMeta    = 0x40005 // SPA_TYPE_OBJECT_ParamMeta
)

// SPA choice types (spa/utils/defs.h).
const (
	ChoiceNone  = 0
	ChoiceRange = 1
	ChoiceEnum  = 3
	ChoiceFlags = 4
)

// SPA param ids (spa/param/param.h).
const (
	ParamEnumFormat = 3
	ParamFormat     = 4
	ParamBuffers    = 5
	ParamMeta       = 6
)

// SPA format property keys (spa/param/format.h).
const (
	FormatKeyMediaType      = 1
	FormatKeyMediaSubtype   = 2
	FormatKeyVideoFormat    = 0x20001
	FormatKeyVideoModifier  = 0x20002
	FormatKeyVideoSize      = 0x20003
	FormatKeyVideoFramerate = 0x20004
	FormatKeyVideoMaxRate   = 0x20005
)

// SPA media subtypes (spa/param/format.h).
const MediaSubtypeRaw = 1

// SPA buffers param keys (spa/param/buffers.h).
const (
	BuffersKeyBuffers  = 1
	BuffersKeyBlocks   = 2
	BuffersKeySize     = 3
	BuffersKeyStride   = 4
	BuffersKeyAlign    = 5
	BuffersKeyDataType = 6
)

// SPA meta param keys (spa/param/buffers.h) and meta types (spa/buffer/meta.h).
const (
	MetaKeyType = 1
	MetaKeySize = 2

	MetaTypeHeader = 1  // SPA_META_Header
	MetaHeaderSize = 32 // sizeof(struct spa_meta_header)
)

// SPA data types (spa/buffer/buffer.h) and flags.
const (
	DataMemFd  = 2
	DataDmaBuf = 3

	DataFlagReadWrite = 3 // SPA_DATA_FLAG_READWRITE

	ChunkFlagCorrupted      = 1          // SPA_CHUNK_FLAG_CORRUPTED
	MetaHeaderFlagCorrupted = 2          // SPA_META_HEADER_FLAG_CORRUPTED
	IDInvalid               = 0xffffffff // SPA_ID_INVALID
)

// SPA raw video formats (spa/param/video/raw.h).
type VideoFormat uint32

const (
	VideoFormatBGRx VideoFormat = 8
	VideoFormatBGRA VideoFormat = 12
)
