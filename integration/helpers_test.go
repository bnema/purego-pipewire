//go:build integration

package integration

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type pollFd struct {
	fd      int32
	events  int16
	revents int16
}

const pollIn = 0x1

// pollPFD and pollTS are package-level because syscall.Syscall6 forces
// pointer arguments passed through uintptr(unsafe.Pointer(&x)) to the heap.
// Only the test goroutine (locked to its thread) uses them.
var (
	pollPFD pollFd
	pollTS  syscall.Timespec
)

// waitReadable waits up to timeout for fd to become readable (or for a
// signal). It does not allocate, so it can sit inside testing.AllocsPerRun.
func waitReadable(fd int, timeout time.Duration) {
	pollPFD = pollFd{fd: int32(fd), events: pollIn}
	pollTS = syscall.Timespec{Sec: int64(timeout / time.Second), Nsec: int64(timeout % time.Second)}
	_, _, _ = syscall.Syscall6(syscall.SYS_PPOLL, uintptr(unsafe.Pointer(&pollPFD)), 1, uintptr(unsafe.Pointer(&pollTS)), 0, 0, 0)
}

// isMapped reports whether the address of p lies inside a mapping listed in
// /proc/self/maps.
func isMapped(p *byte) bool {
	addr := uint64(uintptr(unsafe.Pointer(p)))
	b, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return true
	}
	for _, line := range strings.Split(string(b), "\n") {
		var lo, hi uint64
		if _, err := fmt.Sscanf(line, "%x-%x", &lo, &hi); err == nil && addr >= lo && addr < hi {
			return true
		}
	}
	return false
}
