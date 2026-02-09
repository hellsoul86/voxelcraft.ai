package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestDeterminism_FixedActionsSameDigest(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}

	w1, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}
	w2, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}

	join := func(w *World, name string) string {
		out := make(chan []byte, 1)
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: true, Out: out, Resp: resp})
		r := <-resp
		return r.Welcome.AgentID
	}

	a1w1 := join(w1, "bot")
	a1w2 := join(w2, "bot")
	if a1w1 != a1w2 {
		t.Fatalf("agent id mismatch: %s vs %s", a1w1, a1w2)
	}

	// Simulate 50 ticks with the same action stream.
	for tick := uint64(0); tick < 50; tick++ {
		var acts1 []ActionEnvelope
		var acts2 []ActionEnvelope
		if tick == 0 {
			act := protocol.ActMsg{
				Type:            protocol.TypeAct,
				ProtocolVersion: protocol.Version,
				Tick:            tick,
				AgentID:         a1w1,
				Tasks: []protocol.TaskReq{
					{ID: "K1", Type: "MOVE_TO", Target: [3]int{10, 0, -10}, Tolerance: 1.2},
				},
			}
			acts1 = append(acts1, ActionEnvelope{AgentID: a1w1, Act: act})
			acts2 = append(acts2, ActionEnvelope{AgentID: a1w2, Act: act})
		}

		w1.step(nil, nil, acts1)
		w2.step(nil, nil, acts2)

		d1 := w1.stateDigest(tick)
		d2 := w2.stateDigest(tick)
		if d1 != d2 {
			t.Fatalf("digest mismatch at tick %d: %s vs %s", tick, d1, d2)
		}
	}
}
