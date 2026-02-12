package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestTask_Gather_DeniedForVisitorsInClaim(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 31}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		t.Helper()
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		j := <-resp
		a := w.agents[j.Welcome.AgentID]
		if a == nil {
			t.Fatalf("missing agent")
		}
		return a
	}

	owner := join("owner")
	visitor := join("visitor")

	// Create a claim around owner's position.
	landID := "LAND_TEST"
	w.claims[landID] = &LandClaim{
		LandID: landID,
		Owner:  owner.ID,
		Anchor: owner.Pos,
		Radius: 8,
	}

	dropPos := owner.Pos
	itemID := w.spawnItemEntity(0, owner.ID, dropPos, "PLANK", 5, "TEST")
	if itemID == "" {
		t.Fatalf("spawnItemEntity returned empty id")
	}

	visitor.Pos = dropPos
	start := visitor.Inventory["PLANK"]

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         visitor.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "GATHER", TargetID: itemID}},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: visitor.ID, Act: act}})

	if got := visitor.Inventory["PLANK"]; got != start {
		t.Fatalf("visitor inventory changed on denied pickup: got %d want %d", got, start)
	}
	if w.items[itemID] == nil {
		t.Fatalf("expected item entity to remain after denied pickup")
	}

	// Ensure the failure is E_NO_PERMISSION.
	taskID := ""
	for _, e := range visitor.Events {
		if e["type"] != "ACTION_RESULT" || e["ref"] != "K1" {
			continue
		}
		if s, _ := e["task_id"].(string); s != "" {
			taskID = s
		}
		break
	}
	if taskID == "" {
		t.Fatalf("missing task_id from ACTION_RESULT")
	}
	foundFail := false
	for _, e := range visitor.Events {
		if e["type"] != "TASK_FAIL" || e["task_id"] != taskID {
			continue
		}
		foundFail = true
		if code, _ := e["code"].(string); code != "E_NO_PERMISSION" {
			t.Fatalf("expected E_NO_PERMISSION, got %v", e["code"])
		}
		break
	}
	if !foundFail {
		t.Fatalf("expected TASK_FAIL for denied pickup")
	}

	// Owner can pick it up.
	owner.Pos = dropPos
	ownerStart := owner.Inventory["PLANK"]
	owner.Events = nil
	act2 := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         owner.ID,
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "GATHER", TargetID: itemID}},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: owner.ID, Act: act2}})
	if got := owner.Inventory["PLANK"]; got != ownerStart+5 {
		t.Fatalf("owner inventory after gather: got %d want %d", got, ownerStart+5)
	}
	if w.items[itemID] != nil {
		t.Fatalf("expected item entity removed after owner pickup")
	}
}
