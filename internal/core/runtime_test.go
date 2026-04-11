package core

import (
	"testing"

	portmocks "github.com/bnema/purego-pipewire/internal/ports/out/mocks"
)

func TestRuntimeInitRejectsDifferentCAPIAfterProcessWideInit(t *testing.T) {
	api1 := portmocks.NewMockCAPI(t)
	api2 := portmocks.NewMockCAPI(t)

	api1.EXPECT().PWDeinit().Once()
	api1.EXPECT().PWInit((*int32)(nil), (***byte)(nil)).Once()

	r1 := NewRuntime(api1)
	if err := r1.Init(); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	r2 := NewRuntime(api2)
	if err := r2.Init(); err == nil {
		t.Fatal("second Init returned nil error, want mismatch rejection")
	}

	r2.Deinit()
	r1.Deinit()

	api2.AssertNotCalled(t, "PWInit", (*int32)(nil), (***byte)(nil))
	api2.AssertNotCalled(t, "PWDeinit")
}

func TestRuntimeInitRejectsNilCAPI(t *testing.T) {
	r := NewRuntime(nil)
	if err := r.Init(); err == nil {
		t.Fatal("Init returned nil error, want nil CAPI rejection")
	}

	r.Deinit()
}

func TestRuntimeDeinitWaitsForLastInitializedRuntime(t *testing.T) {
	api1 := portmocks.NewMockCAPI(t)

	deinitCalls := 0
	api1.EXPECT().PWInit((*int32)(nil), (***byte)(nil)).Once()
	api1.EXPECT().PWDeinit().Run(func() {
		deinitCalls++
	}).Once()

	r1 := NewRuntime(api1)
	if err := r1.Init(); err != nil {
		t.Fatalf("first Init returned error: %v", err)
	}
	r2 := NewRuntime(api1)
	if err := r2.Init(); err != nil {
		t.Fatalf("second Init returned error: %v", err)
	}

	r1.Deinit()
	if deinitCalls != 0 {
		t.Fatalf("PWDeinit called after first Deinit, want 0 calls before last runtime deinitializes")
	}

	NewRuntime(api1).Deinit()
	if deinitCalls != 0 {
		t.Fatalf("PWDeinit called after non-initialized runtime Deinit, want 0 calls before last runtime deinitializes")
	}

	r2.Deinit()
	if deinitCalls != 1 {
		t.Fatalf("PWDeinit called %d times, want 1", deinitCalls)
	}
}
