package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestBlueprintAutoPull_PullsFromNearbyChest(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
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
	a.Inventory["PLANK"] = 0

	anchor := Vec3i{X: a.Pos.X, Y: 40, Z: a.Pos.Z}
	chestPos := Vec3i{X: anchor.X + 5, Y: anchor.Y, Z: anchor.Z}
	ch := w.ensureContainer(chestPos, "CHEST")
	ch.Inventory["PLANK"] = 10

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})
	// road_segment places 5 blocks at 2 blocks/tick => 3 ticks total including start tick.
	w.step(nil, nil, nil)
	w.step(nil, nil, nil)

	if a.WorkTask != nil {
		t.Fatalf("expected work task done")
	}
	if got := ch.Inventory["PLANK"]; got != 5 {
		t.Fatalf("chest plank remaining: got %d want %d", got, 5)
	}
	if got := a.Inventory["PLANK"]; got != 0 {
		t.Fatalf("agent plank remaining (pulled then consumed): got %d want %d", got, 0)
	}
}

func TestBlueprintAutoPull_SameLandOnly(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "owner", DeltaVoxels: false, Out: nil, Resp: resp})
	r := <-resp
	a := w.agents[r.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	a.Inventory["PLANK"] = 0

	claimAnchor := Vec3i{X: 0, Y: 40, Z: 0}
	landID := w.newLandID(a.ID)
	w.claims[landID] = &LandClaim{
		LandID:  landID,
		Owner:   a.ID,
		Anchor:  claimAnchor,
		Radius:  32,
		Flags:   ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
		Members: map[string]bool{},
	}

	// Blueprint anchor is inside claim (edge), but a nearby chest is outside claim (still within pull range).
	anchor := Vec3i{X: 32, Y: 40, Z: 0}
	outsideChestPos := Vec3i{X: 64, Y: 40, Z: 0} // dist=32 from anchor, but outside claim dx=64 from claim anchor.
	ch := w.ensureContainer(outsideChestPos, "CHEST")
	ch.Inventory["PLANK"] = 100

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	if a.WorkTask != nil {
		t.Fatalf("expected build task to fail immediately due to missing materials")
	}
	foundFail := false
	for _, e := range a.Events {
		if e["type"] == "TASK_FAIL" {
			foundFail = true
			if e["code"] != "E_NO_RESOURCE" {
				t.Fatalf("expected E_NO_RESOURCE, got %v", e["code"])
			}
			if e["message"] != "missing PLANK x5" {
				t.Fatalf("unexpected message: %v", e["message"])
			}
		}
	}
	if !foundFail {
		t.Fatalf("expected TASK_FAIL event")
	}
}
