package capi

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestLoopLockUnlocksOnPanic(t *testing.T) {
	oldLock, oldUnlock := pw_loop_lock, pw_loop_unlock
	t.Cleanup(func() { pw_loop_lock, pw_loop_unlock = oldLock, oldUnlock })
	locked, unlocked := false, false
	pw_loop_lock = func(unsafe.Pointer) int32 { locked = true; return 0 }
	pw_loop_unlock = func(unsafe.Pointer) int32 { unlocked = true; return 0 }
	func() {
		defer func() {
			if recover() == nil {
				t.Error("missing panic")
			}
		}()
		defaultWithLoopLock(opaquePtr(), func() int32 { runtime.Gosched(); panic("test") })
	}()
	if !locked || !unlocked {
		t.Fatalf("lock=%v unlock=%v", locked, unlocked)
	}
}

func TestLoopLockErrors(t *testing.T) {
	oldLock, oldUnlock := pw_loop_lock, pw_loop_unlock
	t.Cleanup(func() { pw_loop_lock, pw_loop_unlock = oldLock, oldUnlock })
	pw_loop_lock = func(unsafe.Pointer) int32 { return -2 }
	pw_loop_unlock = func(unsafe.Pointer) int32 { t.Error("unlock after failed lock"); return 0 }
	if ret := defaultWithLoopLock(opaquePtr(), func() int32 { t.Error("called after failed lock"); return 0 }); ret != -2 {
		t.Fatal(ret)
	}
	pw_loop_lock = func(unsafe.Pointer) int32 { return 0 }
	pw_loop_unlock = func(unsafe.Pointer) int32 { return -3 }
	if ret := defaultWithLoopLock(opaquePtr(), func() int32 { return 0 }); ret != -3 {
		t.Fatal(ret)
	}
	if ret := defaultWithLoopLock(opaquePtr(), func() int32 { return -4 }); ret != -4 {
		t.Fatal(ret)
	}
}
