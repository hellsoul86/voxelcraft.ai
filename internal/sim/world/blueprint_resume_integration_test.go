package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestBuildBlueprint_ResumeByReissue_SkipsCorrectBlocks(t *testing.T) {
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
	w.step([]JoinRequest{{Name: "builder", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// Ensure enough materials across retries.
	a.Inventory["PLANK"] = 20

	anchor := Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	clearBlueprintFootprint(t, w, "road_segment", anchor, 0)

	// Start building and let the first tick place some blocks.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0}},
	}}})
	if a.WorkTask == nil {
		t.Fatalf("expected work task to start")
	}
	taskID := a.WorkTask.TaskID

	// Cancel and re-issue the same blueprint build at the same anchor/rotation.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Cancel:          []string{taskID},
	}}})
	if a.WorkTask != nil {
		t.Fatalf("expected work task canceled")
	}

	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0}},
	}}})

	// road_segment is short; advance until done.
	for i := 0; i < 5; i++ {
		w.step(nil, nil, nil)
		if a.WorkTask == nil {
			break
		}
	}
	if a.WorkTask != nil {
		t.Fatalf("expected work task done")
	}
	if !w.checkBlueprintPlaced("road_segment", anchor, 0) {
		t.Fatalf("expected blueprint to be fully placed after re-issue")
	}
}
