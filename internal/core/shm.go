package core

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	mfdCloexec      = 0x1
	mfdAllowSealing = 0x2
	fAddSeals       = 0x409 // F_ADD_SEALS
	fSealSeal       = 0x1
	fSealShrink     = 0x2
	fSealGrow       = 0x4
)

// memfdShm is the production portout.SharedMemAPI backed by memfd_create.
type memfdShm struct{}

func (memfdShm) NewSharedMem(name string, size int) (int, []byte, error) {
	if size <= 0 {
		return -1, nil, fmt.Errorf("memfd: invalid size %d", size)
	}
	cname, err := syscall.BytePtrFromString(name)
	if err != nil {
		return -1, nil, err
	}
	r, _, errno := syscall.Syscall(sysMemfdCreate, uintptr(unsafe.Pointer(cname)), mfdCloexec|mfdAllowSealing, 0)
	if errno != 0 {
		return -1, nil, fmt.Errorf("memfd_create: %w", errno)
	}
	fd := int(r)
	if err := syscall.Ftruncate(fd, int64(size)); err != nil {
		_ = syscall.Close(fd)
		return -1, nil, fmt.Errorf("memfd ftruncate: %w", err)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), fAddSeals, fSealShrink|fSealGrow|fSealSeal); errno != 0 {
		_ = syscall.Close(fd)
		return -1, nil, fmt.Errorf("memfd seal: %w", errno)
	}
	data, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = syscall.Close(fd)
		return -1, nil, fmt.Errorf("memfd mmap: %w", err)
	}
	return fd, data, nil
}

func (memfdShm) FreeSharedMem(fd int, data []byte) {
	if data != nil {
		_ = syscall.Munmap(data)
	}
	if fd >= 0 {
		_ = syscall.Close(fd)
	}
}
