package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestMoveTo_DetourWhenPrimaryAndSecondaryBlocked(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	respCh := make(chan JoinResponse, 1)
	w.step([]JoinRequest{{Name: "bot", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	start := Vec3i{X: 10, Y: 0, Z: 10}
	a.Pos = start
	target := Vec3i{X: start.X + 5, Y: 0, Z: start.Z}

	stone := cats.Blocks.Index["STONE"]

	// Make primary (+X), secondary (+Z), and back (-X) all blocked, but leave a detour via -Z.
	setAir(w, start)
	setSolid(w, Vec3i{X: start.X + 1, Y: 0, Z: start.Z}, stone) // +X
	setSolid(w, Vec3i{X: start.X, Y: 0, Z: start.Z + 1}, stone) // +Z
	setSolid(w, Vec3i{X: start.X - 1, Y: 0, Z: start.Z}, stone) // -X

	// Clear a corridor along -Z then +X so a detour exists within depth 16.
	for x := start.X; x <= target.X; x++ {
		setAir(w, Vec3i{X: x, Y: 0, Z: start.Z - 1})
	}

	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: target.ToArray(), Tolerance: 1.2}},
	}}})

	if got := a.Pos; got != (Vec3i{X: start.X, Y: 0, Z: start.Z - 1}) {
		t.Fatalf("pos=%+v want detour step %+v", got, Vec3i{X: start.X, Y: 0, Z: start.Z - 1})
	}
}

