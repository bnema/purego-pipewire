package core

import (
	"testing"

	portmocks "github.com/bnema/purego-pipewire/internal/ports/out/mocks"
)

func TestRuntimeInitCallsPWInitOnce(t *testing.T) {
	api := portmocks.NewMockCAPI(t)
	api.EXPECT().PWInit((*int32)(nil), (***byte)(nil)).Once()

	r := NewRuntime(api)
	if err := r.Init(); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if err := r.Init(); err != nil {
		t.Fatalf("second Init returned error: %v", err)
	}
}

func TestRuntimeDeinitCallsPWDeinit(t *testing.T) {
	api := portmocks.NewMockCAPI(t)
	api.EXPECT().PWDeinit().Once()

	r := NewRuntime(api)
	r.Deinit()
}
