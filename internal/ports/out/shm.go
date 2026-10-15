package out

// SharedMemAPI abstracts the anonymous shared memory used for video buffers
// so the stream logic can be tested without real memfds.
type SharedMemAPI interface {
	// NewSharedMem creates a sealed, size-byte memfd and maps it read-write
	// and shared. The caller owns fd and data until FreeSharedMem.
	NewSharedMem(name string, size int) (fd int, data []byte, err error)
	// FreeSharedMem unmaps data and closes fd.
	FreeSharedMem(fd int, data []byte)
}
