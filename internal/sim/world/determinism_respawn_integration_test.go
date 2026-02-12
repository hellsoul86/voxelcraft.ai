package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestDeterminism_MultiAgentRespawnItemIDsStable(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
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

	joinN := func(w *World, n int) []*Agent {
		out := make([]*Agent, 0, n)
		for i := 0; i < n; i++ {
			resp := make(chan JoinResponse, 1)
			w.handleJoin(JoinRequest{Name: "a", DeltaVoxels: false, Out: nil, Resp: resp})
			j := <-resp
			out = append(out, w.agents[j.Welcome.AgentID])
		}
		return out
	}

	a1 := joinN(w1, 3)
	a2 := joinN(w2, 3)

	// Force simultaneous respawns at different positions so item entity IDs are order-sensitive.
	for i := 0; i < 3; i++ {
		p := Vec3i{X: 100 + i*10, Y: 40, Z: -100 - i*7}
		a1[i].Pos = p
		a2[i].Pos = p
		a1[i].Inventory = map[string]int{"PLANK": 10}
		a2[i].Inventory = map[string]int{"PLANK": 10}
		a1[i].HP = 0
		a2[i].HP = 0
	}

	w1.systemEnvironment(0)
	w2.systemEnvironment(0)

	d1 := w1.stateDigest(0)
	d2 := w2.stateDigest(0)
	if d1 != d2 {
		t.Fatalf("digest mismatch after multi-agent respawn: %s vs %s", d1, d2)
	}
}
