package core

import (
	"os"
	"strconv"
	"syscall"
	"testing"
)

func TestMemfdShmRoundTrip(t *testing.T) {
	fd, data, err := memfdShm{}.NewSharedMem("pw-video-test", 4096)
	if err != nil {
		t.Skipf("memfd_create unavailable: %v", err)
	}
	if len(data) != 4096 {
		t.Fatalf("len = %d", len(data))
	}
	data[0], data[4095] = 1, 2
	// Re-map through the fd: the memory is shared and sealed against resizing.
	again, err := syscall.Mmap(fd, 0, 4096, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		t.Fatal(err)
	}
	if again[0] != 1 || again[4095] != 2 {
		t.Fatal("memfd is not shared")
	}
	if err := syscall.Ftruncate(fd, 8192); err == nil {
		t.Fatal("sealed memfd was resized")
	}
	_ = syscall.Munmap(again)
	memfdShm{}.FreeSharedMem(fd, data)
	if _, err := os.Stat("/proc/self/fd/" + strconv.Itoa(fd)); err == nil {
		t.Fatal("fd still open after FreeSharedMem")
	}
	if _, _, err := (memfdShm{}).NewSharedMem("x", 0); err == nil {
		t.Fatal("zero size accepted")
	}
}
