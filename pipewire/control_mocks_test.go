package pipewire_test

import (
	outmocks "github.com/bnema/purego-pipewire/internal/ports/out/mocks"
	playermocks "github.com/bnema/purego-pipewire/pipewire/mocks"
	"github.com/bnema/purego-pipewire/spa"
	"testing"
	"unsafe"
)

func TestControlMockVariadicExpectations(t *testing.T) {
	v := spa.PropVolume.Value(1)
	p := playermocks.NewMockPlayer(t)
	p.EXPECT().SetControl(v, v).Return(nil).Once()
	if err := p.SetControl(v, v); err != nil {
		t.Fatal(err)
	}
	ops := outmocks.NewMockStreamOps(t)
	ops.EXPECT().SetStreamControl(unsafe.Pointer(nil), v, v).Return(nil).Once()
	if err := ops.SetStreamControl(nil, v, v); err != nil {
		t.Fatal(err)
	}
	api := outmocks.NewMockCAPI(t)
	api.EXPECT().PWStreamSetControl(unsafe.Pointer(nil), uint32(1), uint32(1), (*float32)(nil), uint32(0)).Return(int32(0)).Once()
	if ret := api.PWStreamSetControl(nil, 1, 1, nil, uint32(0)); ret != 0 {
		t.Fatal(ret)
	}
}
