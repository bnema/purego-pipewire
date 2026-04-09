package spa

// Format represents a SPA media format identifier.
type Format uint32

// Standard SPA audio/video formats.
const (
	FormatUnknown  Format = 0
	FormatEncoded  Format = 1
	FormatS8       Format = 2
	FormatU8       Format = 3
	FormatS16LE    Format = 4
	FormatS16BE    Format = 5
	FormatU16LE    Format = 6
	FormatU16BE    Format = 7
	FormatS24LE    Format = 8
	FormatS24BE    Format = 9
	FormatU24LE    Format = 10
	FormatU24BE    Format = 11
	FormatS32LE    Format = 12
	FormatS32BE    Format = 13
	FormatU32LE    Format = 14
	FormatU32BE    Format = 15
	FormatF32LE    Format = 16
	FormatF32BE    Format = 17
	FormatF64LE    Format = 18
	FormatF64BE    Format = 19
	FormatS24_32LE Format = 20
	FormatS24_32BE Format = 21
	FormatU24_32LE Format = 22
	FormatU24_32BE Format = 23
)
