package spa

import (
	"encoding/binary"
	"errors"
	"math"
)

// Errors returned by the pod builder.
var (
	// ErrPodFull is returned when the destination buffer is too small.
	ErrPodFull = errors.New("spa: pod buffer full")
	// ErrPodState is returned for unbalanced Object/End or Prop outside an object.
	ErrPodState = errors.New("spa: invalid pod builder state")
)

const podMaxDepth = 4

// Fraction is a SPA fraction (num/denom).
type Fraction struct{ Num, Denom uint32 }

// PodBuilder encodes SPA pods little-endian into a caller-owned buffer. It
// never allocates and never grows past len(buf): when the buffer is full the
// first error is kept and returned by Bytes. The zero value is unusable; use
// NewPodBuilder.
type PodBuilder struct {
	buf   []byte
	n     int
	err   error
	depth int
	stack [podMaxDepth]int // offset of each open object header
}

// NewPodBuilder returns a builder writing into buf.
func NewPodBuilder(buf []byte) PodBuilder { return PodBuilder{buf: buf} }

// Bytes returns the encoded pod bytes (a prefix of the destination buffer) or
// the first error. It fails if an Object is still open.
func (b *PodBuilder) Bytes() ([]byte, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.depth != 0 {
		return nil, ErrPodState
	}
	return b.buf[:b.n], nil
}

func (b *PodBuilder) fail(err error) {
	if b.err == nil {
		b.err = err
	}
}

// room reports whether n more bytes fit, recording ErrPodFull otherwise.
func (b *PodBuilder) room(n int) bool {
	if b.err != nil {
		return false
	}
	if n < 0 || b.n+n > len(b.buf) {
		b.fail(ErrPodFull)
		return false
	}
	return true
}

func (b *PodBuilder) u32(v uint32) {
	binary.LittleEndian.PutUint32(b.buf[b.n:], v)
	b.n += 4
}

// pad8 zero-pads the write offset to a multiple of 8 bytes.
func (b *PodBuilder) pad8() {
	for b.n&7 != 0 {
		b.buf[b.n] = 0
		b.n++
	}
}

// Object opens an object pod. Close it with End.
func (b *PodBuilder) Object(objType, id uint32) {
	if !b.room(16) {
		return
	}
	if b.depth == podMaxDepth {
		b.fail(ErrPodState)
		return
	}
	b.stack[b.depth] = b.n
	b.depth++
	b.u32(0) // size, patched by End
	b.u32(TypeObject)
	b.u32(objType)
	b.u32(id)
}

// Prop starts an object property with the given key; the next value written
// is the property value.
func (b *PodBuilder) Prop(key uint32) {
	if b.err == nil && b.depth == 0 {
		b.fail(ErrPodState)
		return
	}
	if !b.room(8) {
		return
	}
	b.u32(key)
	b.u32(0) // flags
}

// End closes the innermost open object.
func (b *PodBuilder) End() {
	if b.err != nil {
		return
	}
	if b.depth == 0 {
		b.fail(ErrPodState)
		return
	}
	b.depth--
	start := b.stack[b.depth]
	binary.LittleEndian.PutUint32(b.buf[start:], uint32(b.n-start-8))
	// Padding was already room-checked by the last value; objects always
	// end on an 8-byte boundary because every value is padded.
	b.pad8()
}

func (b *PodBuilder) scalar(typ uint32, size uint32, a, c uint32) {
	// size is the body size (4 or 8); the pod is padded to 16 bytes.
	if !b.room(16) {
		return
	}
	b.u32(size)
	b.u32(typ)
	b.u32(a)
	b.u32(c)
}

// Id appends an Id pod.
func (b *PodBuilder) Id(v uint32) { b.scalar(TypeId, 4, v, 0) }

// Int appends an Int pod.
func (b *PodBuilder) Int(v int32) { b.scalar(TypeInt, 4, uint32(v), 0) }

// Rectangle appends a Rectangle pod.
func (b *PodBuilder) Rectangle(w, h uint32) { b.scalar(TypeRectangle, 8, w, h) }

// Fraction appends a Fraction pod.
func (b *PodBuilder) Fraction(num, denom uint32) { b.scalar(TypeFraction, 8, num, denom) }

// choiceBegin writes the choice header; it returns the header offset.
func (b *PodBuilder) choiceBegin(choiceType, childSize, childType uint32, nvals int) (int, bool) {
	// Room for the header, the values and the padding choiceEnd adds.
	if !b.room((b.n+24+nvals*int(childSize)+7)&^7 - b.n) {
		return 0, false
	}
	start := b.n
	b.u32(0) // size, patched in choiceEnd
	b.u32(TypeChoice)
	b.u32(choiceType)
	b.u32(0) // flags
	b.u32(childSize)
	b.u32(childType)
	return start, true
}

func (b *PodBuilder) choiceEnd(start int) {
	binary.LittleEndian.PutUint32(b.buf[start:], uint32(b.n-start-8))
	b.pad8()
}

// ChoiceEnumId appends an enum choice of Ids. vals is written verbatim: the
// first value is the default and the remaining values are the alternatives.
func (b *PodBuilder) ChoiceEnumId(vals ...uint32) {
	if len(vals) == 0 {
		b.fail(ErrPodState)
		return
	}
	start, ok := b.choiceBegin(ChoiceEnum, 4, TypeId, len(vals))
	if !ok {
		return
	}
	for _, v := range vals {
		b.u32(v)
	}
	b.choiceEnd(start)
}

// ChoiceRangeInt appends a range choice of Ints.
func (b *PodBuilder) ChoiceRangeInt(def, min, max int32) {
	start, ok := b.choiceBegin(ChoiceRange, 4, TypeInt, 3)
	if !ok {
		return
	}
	b.u32(uint32(def))
	b.u32(uint32(min))
	b.u32(uint32(max))
	b.choiceEnd(start)
}

// ChoiceRangeFraction appends a range choice of Fractions.
func (b *PodBuilder) ChoiceRangeFraction(def, min, max Fraction) {
	start, ok := b.choiceBegin(ChoiceRange, 8, TypeFraction, 3)
	if !ok {
		return
	}
	for _, f := range [3]Fraction{def, min, max} {
		b.u32(f.Num)
		b.u32(f.Denom)
	}
	b.choiceEnd(start)
}

// ChoiceFlagsInt appends a flags choice of Ints with a single flags value.
func (b *PodBuilder) ChoiceFlagsInt(flags uint32) {
	start, ok := b.choiceBegin(ChoiceFlags, 4, TypeInt, 1)
	if !ok {
		return
	}
	b.u32(flags)
	b.choiceEnd(start)
}

// VideoEnumFormat encodes an EnumFormat param for raw video of the given
// size, maximum frame rate and formats (the first format is the default).
// Modifiers are omitted (shared memory only).
func VideoEnumFormat(dst []byte, w, h, maxFPS int, formats ...VideoFormat) ([]byte, error) {
	if len(formats) == 0 || w <= 0 || h <= 0 || maxFPS <= 0 {
		return nil, ErrPodState
	}
	b := NewPodBuilder(dst)
	b.Object(TypeObjectFormat, ParamEnumFormat)
	b.Prop(FormatKeyMediaType)
	b.Id(MediaTypeVideo)
	b.Prop(FormatKeyMediaSubtype)
	b.Id(MediaSubtypeRaw)
	b.Prop(FormatKeyVideoFormat)
	if len(formats) == 1 {
		b.Id(uint32(formats[0]))
	} else {
		b.podEnumFormats(formats)
	}
	b.Prop(FormatKeyVideoSize)
	b.Rectangle(uint32(w), uint32(h))
	b.Prop(FormatKeyVideoFramerate)
	b.Fraction(0, 1)
	b.Prop(FormatKeyVideoMaxRate)
	b.ChoiceRangeFraction(
		Fraction{uint32(maxFPS), 1},
		Fraction{1, 1},
		Fraction{uint32(maxFPS), 1},
	)
	b.End()
	return b.Bytes()
}

// podEnumFormats writes an enum choice of video formats. As in SPA's own
// builders, the first value is the default and is repeated in the list of
// alternatives.
func (b *PodBuilder) podEnumFormats(formats []VideoFormat) {
	start, ok := b.choiceBegin(ChoiceEnum, 4, TypeId, len(formats)+1)
	if !ok {
		return
	}
	b.u32(uint32(formats[0]))
	for _, f := range formats {
		b.u32(uint32(f))
	}
	b.choiceEnd(start)
}

// VideoBuffers encodes a Buffers param: one block of size bytes with the
// given stride, a buffer count range (minBuf..maxBuf, defBuf by default) and
// the allowed SPA data types as a bit mask (for example 1<<DataMemFd).
func VideoBuffers(dst []byte, size, stride, minBuf, defBuf, maxBuf int, dataTypes uint32) ([]byte, error) {
	if size <= 0 || stride <= 0 || minBuf <= 0 || defBuf < minBuf || maxBuf < defBuf ||
		size > math.MaxInt32 || stride > math.MaxInt32 || maxBuf > math.MaxInt32 {
		return nil, ErrPodState
	}
	b := NewPodBuilder(dst)
	b.Object(TypeObjectParamBuffers, ParamBuffers)
	b.Prop(BuffersKeyBuffers)
	b.ChoiceRangeInt(int32(defBuf), int32(minBuf), int32(maxBuf))
	b.Prop(BuffersKeyBlocks)
	b.Int(1)
	b.Prop(BuffersKeySize)
	b.Int(int32(size))
	b.Prop(BuffersKeyStride)
	b.Int(int32(stride))
	b.Prop(BuffersKeyAlign)
	b.Int(16)
	b.Prop(BuffersKeyDataType)
	b.ChoiceFlagsInt(dataTypes)
	b.End()
	return b.Bytes()
}

// MetaHeader encodes a Meta param requesting a spa_meta_header (32 bytes).
func MetaHeader(dst []byte) ([]byte, error) {
	b := NewPodBuilder(dst)
	b.Object(TypeObjectParamMeta, ParamMeta)
	b.Prop(MetaKeyType)
	b.Id(MetaTypeHeader)
	b.Prop(MetaKeySize)
	b.Int(MetaHeaderSize)
	b.End()
	return b.Bytes()
}

// ParseVideoFormat extracts the video format and size from a Format param
// (an Object pod). It does not allocate. ok is false for malformed,
// truncated or non-raw-video pods.
//
// PipeWire wraps every value of a negotiated Format in a Choice of type None
// holding a single value; plain values are accepted as well. Choices that
// still offer alternatives (Enum, Range, ...) are not a fixed format and are
// rejected.
func ParseVideoFormat(pod []byte) (format uint32, w, h int, ok bool) {
	if len(pod) < 16 {
		return 0, 0, 0, false
	}
	le := binary.LittleEndian
	size := int(le.Uint32(pod[0:]))
	if le.Uint32(pod[4:]) != TypeObject || size < 8 || 8+size > len(pod) {
		return 0, 0, 0, false
	}
	if le.Uint32(pod[8:]) != TypeObjectFormat {
		return 0, 0, 0, false
	}
	end := 8 + size
	off := 16
	var haveFmt, haveSize, isVideo, isRaw bool
	for off+16 <= end {
		key := le.Uint32(pod[off:])
		vsize := int(le.Uint32(pod[off+8:]))
		vtype := le.Uint32(pod[off+12:])
		body := off + 16
		if vsize < 0 || body+vsize > end {
			return 0, 0, 0, false
		}
		next := (body + vsize + 7) &^ 7
		if next <= off {
			return 0, 0, 0, false
		}
		if vtype == TypeChoice {
			// Choice body: type, flags, then the child pod header (size, type)
			// and the values.
			if vsize < 16 || le.Uint32(pod[body:]) != ChoiceNone {
				return 0, 0, 0, false
			}
			childSize := int(le.Uint32(pod[body+8:]))
			vtype = le.Uint32(pod[body+12:])
			body += 16
			vsize -= 16
			if childSize < 0 || childSize > vsize {
				return 0, 0, 0, false
			}
			vsize = childSize
		}
		switch {
		case key == FormatKeyMediaType && vtype == TypeId && vsize >= 4:
			isVideo = le.Uint32(pod[body:]) == MediaTypeVideo
		case key == FormatKeyMediaSubtype && vtype == TypeId && vsize >= 4:
			isRaw = le.Uint32(pod[body:]) == MediaSubtypeRaw
		case key == FormatKeyVideoFormat && vtype == TypeId && vsize >= 4:
			format = le.Uint32(pod[body:])
			haveFmt = true
		case key == FormatKeyVideoSize && vtype == TypeRectangle && vsize >= 8:
			uw, uh := le.Uint32(pod[body:]), le.Uint32(pod[body+4:])
			if uw == 0 || uh == 0 || uw > 1<<20 || uh > 1<<20 {
				return 0, 0, 0, false
			}
			w, h = int(uw), int(uh)
			haveSize = true
		}
		off = next
	}
	if !(haveFmt && haveSize && isVideo && isRaw) {
		return 0, 0, 0, false
	}
	return format, w, h, true
}
