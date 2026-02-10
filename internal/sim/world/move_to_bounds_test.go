package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestMoveTo_OutOfBoundsRejectedAtStart(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 41, BoundaryR: 8}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "mover", DeltaVoxels: false, Out: nil, Resp: resp})
	jr := <-resp
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	w.tick.Store(1)

	a.Events = nil
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: [3]int{w.cfg.BoundaryR + 1, 0, 0}, Tolerance: 1.2}},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	if a.MoveTask != nil {
		t.Fatalf("expected no move task for out-of-bounds target")
	}

	found := false
	for _, e := range a.Events {
		if e["type"] != "ACTION_RESULT" || e["ref"] != "K1" {
			continue
		}
		found = true
		if ok, _ := e["ok"].(bool); ok {
			t.Fatalf("expected ok=false for out-of-bounds MOVE_TO")
		}
		if code, _ := e["code"].(string); code != "E_INVALID_TARGET" {
			t.Fatalf("expected E_INVALID_TARGET, got %v", e["code"])
		}
		break
	}
	if !found {
		t.Fatalf("expected ACTION_RESULT for rejected MOVE_TO")
	}
}
