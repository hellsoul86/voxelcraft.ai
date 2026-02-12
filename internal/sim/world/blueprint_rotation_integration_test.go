package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestBuildBlueprint_RotationAffectsPlacementAndValidation(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "builder", DeltaVoxels: false, Out: nil, Resp: resp})
	r := <-resp
	a := w.agents[r.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	a.Inventory["PLANK"] = 10

	anchor := Vec3i{X: 10, Y: 0, Z: 10}
	rot := 1
	clearBlueprintFootprint(t, w, "road_segment", anchor, rot)

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: rot},
		},
	}

	// road_segment places 5 blocks at 2 blocks/tick => 3 ticks total including start tick.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})
	w.step(nil, nil, nil)
	w.step(nil, nil, nil)

	if a.WorkTask != nil {
		t.Fatalf("expected work task done")
	}

	plankID := cats.Blocks.Index["PLANK"]
	// rotation=1 should place along +X axis: offsets (0..4, 0)
	for i := 0; i < 5; i++ {
		p := Vec3i{X: anchor.X + i, Y: 0, Z: anchor.Z}
		if got := w.chunks.GetBlock(p); got != plankID {
			t.Fatalf("block at %+v: got %d want %d (PLANK)", p, got, plankID)
		}
	}

	if !w.checkBlueprintPlaced("road_segment", anchor, rot) {
		t.Fatalf("expected rotated blueprint validation to pass")
	}
	if w.checkBlueprintPlaced("road_segment", anchor, 0) {
		t.Fatalf("expected unrotated blueprint validation to fail")
	}
}

func TestBuildBlueprint_Rotation_LShape(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "builder", DeltaVoxels: false, Out: nil, Resp: resp})
	r := <-resp
	a := w.agents[r.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	a.Inventory["PLANK"] = 20

	anchor := Vec3i{X: 10, Y: 0, Z: 10}
	rot := 1
	clearBlueprintFootprint(t, w, "road_turn", anchor, rot)

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_turn", Anchor: anchor.ToArray(), Rotation: rot},
		},
	}

	// road_turn places 8 blocks at 2 blocks/tick => 4 ticks total including start tick.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})
	w.step(nil, nil, nil)
	w.step(nil, nil, nil)
	w.step(nil, nil, nil)

	if a.WorkTask != nil {
		t.Fatalf("expected work task done")
	}

	plankID := cats.Blocks.Index["PLANK"]
	// rotation=1 should place:
	// - a 5-long segment along +X at Z=0 (relative), and
	// - a 3-long segment along -Z at X=4 (relative), excluding the corner overlap.
	expect := []Vec3i{
		{X: anchor.X + 0, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 1, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 2, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 3, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z - 1},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z - 2},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z - 3},
	}
	for _, p := range expect {
		if got := w.chunks.GetBlock(p); got != plankID {
			t.Fatalf("block at %+v: got %d want %d (PLANK)", p, got, plankID)
		}
	}

	if !w.checkBlueprintPlaced("road_turn", anchor, rot) {
		t.Fatalf("expected rotated blueprint validation to pass")
	}
	if w.checkBlueprintPlaced("road_turn", anchor, 0) {
		t.Fatalf("expected unrotated blueprint validation to fail")
	}
}
