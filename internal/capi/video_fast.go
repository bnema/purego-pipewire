package capi

import (
	"unsafe"

	"bnema.dev/purego"
)

// The generated bindings call C through purego.RegisterLibFunc, whose
// reflect-based wrappers allocate on every call. The calls made once per
// video frame (and once per loop iteration) must not allocate, so they go
// through purego.Syscall6 instead: the methods below shadow the generated
// adapter methods of the same name on defaultCAPI. Everything else keeps using
// the generated bindings.
//
// All hot-path functions take only pointers/integers and return an integer
// (or nothing), which is what Syscall6 supports.
var (
	symLoopGetFd           uintptr
	symLoopEnter           uintptr
	symLoopLeave           uintptr
	symLoopIterate         uintptr
	symStreamDequeueBuffer uintptr
	symStreamQueueBuffer   uintptr
	symStreamReturnBuffer  uintptr
	symStreamTrigger       uintptr
)

func registerVideoFast(handle uintptr) error {
	for _, s := range []struct {
		dst  *uintptr
		name string
	}{
		{&symLoopGetFd, "pw_loop_get_fd"},
		{&symLoopEnter, "pw_loop_enter"},
		{&symLoopLeave, "pw_loop_leave"},
		{&symLoopIterate, "pw_loop_iterate"},
		{&symStreamDequeueBuffer, "pw_stream_dequeue_buffer"},
		{&symStreamQueueBuffer, "pw_stream_queue_buffer"},
		{&symStreamReturnBuffer, "pw_stream_return_buffer"},
		{&symStreamTrigger, "pw_stream_trigger_process"},
	} {
		addr, err := purego.Dlsym(handle, s.name)
		if err != nil {
			return err
		}
		*s.dst = addr
	}
	return nil
}

// uptr converts a C pointer returned as an integer without a
// uintptr-to-unsafe.Pointer conversion (the memory is owned by C).
func uptr(v uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&v)) }

func (*defaultCAPI) PWLoopGetFd(loop unsafe.Pointer) int32 {
	r, _, _ := purego.Syscall6(symLoopGetFd, uintptr(loop), 0, 0, 0, 0, 0)
	return int32(r)
}

func (*defaultCAPI) PWLoopEnter(loop unsafe.Pointer) {
	purego.Syscall6(symLoopEnter, uintptr(loop), 0, 0, 0, 0, 0)
}

func (*defaultCAPI) PWLoopLeave(loop unsafe.Pointer) {
	purego.Syscall6(symLoopLeave, uintptr(loop), 0, 0, 0, 0, 0)
}

func (*defaultCAPI) PWLoopIterate(loop unsafe.Pointer, timeout int32) int32 {
	r, _, _ := purego.Syscall6(symLoopIterate, uintptr(loop), uintptr(uint32(timeout)), 0, 0, 0, 0)
	return int32(r)
}

func (*defaultCAPI) PWStreamDequeueBuffer(stream unsafe.Pointer) unsafe.Pointer {
	r, _, _ := purego.Syscall6(symStreamDequeueBuffer, uintptr(stream), 0, 0, 0, 0, 0)
	return uptr(r)
}

func (*defaultCAPI) PWStreamQueueBuffer(stream unsafe.Pointer, buffer unsafe.Pointer) int32 {
	r, _, _ := purego.Syscall6(symStreamQueueBuffer, uintptr(stream), uintptr(buffer), 0, 0, 0, 0)
	return int32(r)
}

func (*defaultCAPI) PWStreamReturnBuffer(stream unsafe.Pointer, buffer unsafe.Pointer) int32 {
	r, _, _ := purego.Syscall6(symStreamReturnBuffer, uintptr(stream), uintptr(buffer), 0, 0, 0, 0)
	return int32(r)
}

func (*defaultCAPI) PWStreamTriggerProcess(stream unsafe.Pointer) int32 {
	r, _, _ := purego.Syscall6(symStreamTrigger, uintptr(stream), 0, 0, 0, 0, 0)
	return int32(r)
}
