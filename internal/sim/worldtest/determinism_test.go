package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestDeterminism_FixedActionsSameDigest(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}

	w1, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}
	w2, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}

	join := func(w *world.World, name string) string {
		resp := make(chan world.JoinResponse, 1)
		_, _ = w.StepOnce([]world.JoinRequest{{Name: name, DeltaVoxels: true, Out: nil, Resp: resp}}, nil, nil)
		r := <-resp
		return r.Welcome.AgentID
	}

	a1w1 := join(w1, "bot")
	a1w2 := join(w2, "bot")
	if a1w1 != a1w2 {
		t.Fatalf("agent id mismatch: %s vs %s", a1w1, a1w2)
	}

	startTick := w1.CurrentTick()
	if got := w2.CurrentTick(); got != startTick {
		t.Fatalf("world tick mismatch after join: w1=%d w2=%d", startTick, got)
	}

	// Simulate 50 ticks with the same action stream.
	for i := uint64(0); i < 50; i++ {
		wantTick := startTick + i
		var acts1 []world.ActionEnvelope
		var acts2 []world.ActionEnvelope
		if i == 0 {
			act := protocol.ActMsg{
				Type:            protocol.TypeAct,
				ProtocolVersion: protocol.Version,
				Tick:            wantTick,
				AgentID:         a1w1,
				Tasks: []protocol.TaskReq{
					{ID: "K1", Type: "MOVE_TO", Target: [3]int{10, 0, -10}, Tolerance: 1.2},
				},
			}
			acts1 = append(acts1, world.ActionEnvelope{AgentID: a1w1, Act: act})
			acts2 = append(acts2, world.ActionEnvelope{AgentID: a1w2, Act: act})
		}

		t1, d1 := w1.StepOnce(nil, nil, acts1)
		t2, d2 := w2.StepOnce(nil, nil, acts2)
		if t1 != wantTick || t2 != wantTick {
			t.Fatalf("tick mismatch: got w1=%d w2=%d want %d", t1, t2, wantTick)
		}
		if d1 != d2 {
			t.Fatalf("digest mismatch at tick %d: %s vs %s", wantTick, d1, d2)
		}
	}
}
