package runtime

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestContainerIDRoundTrip(t *testing.T) {
	pos := modelpkg.Vec3i{X: 3, Y: 0, Z: -7}
	id := ContainerID("CHEST", pos)
	typ, got, ok := ParseContainerID(id)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if typ != "CHEST" || got != pos {
		t.Fatalf("unexpected parse result: typ=%s pos=%+v", typ, got)
	}
}

func TestEnsureConveyorCardinal(t *testing.T) {
	conveyors := map[modelpkg.Vec3i]modelpkg.ConveyorMeta{}
	p := modelpkg.Vec3i{X: 1, Y: 0, Z: 1}
	EnsureConveyor(conveyors, p, 1, 1)
	m := conveyors[p]
	if m.DX == 0 && m.DZ == 0 {
		t.Fatalf("expected direction set")
	}
	if m.DX != 0 && m.DZ != 0 {
		t.Fatalf("expected cardinal direction only, got dx=%d dz=%d", m.DX, m.DZ)
	}
}
