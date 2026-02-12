package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func joinN(t *testing.T, w *world.World, n int) []string {
	t.Helper()
	joins := make([]world.JoinRequest, 0, n)
	resps := make([]chan world.JoinResponse, 0, n)
	for i := 0; i < n; i++ {
		resp := make(chan world.JoinResponse, 1)
		resps = append(resps, resp)
		joins = append(joins, world.JoinRequest{Name: "a", DeltaVoxels: false, Out: nil, Resp: resp})
	}
	_, _ = w.StepOnce(joins, nil, nil)

	ids := make([]string, 0, n)
	for _, ch := range resps {
		j := <-ch
		ids = append(ids, j.Welcome.AgentID)
	}
	return ids
}

func TestDeterminism_MultiAgentRespawnItemIDsStable(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := world.WorldConfig{
		ID:          "test",
		WorldType:   "OVERWORLD",
		TickRateHz:  5,
		DayTicks:    6000,
		ObsRadius:   7,
		Height:      1,
		Seed:        42,
		BoundaryR:   4000,
		StarterItems: map[string]int{},
	}

	w1, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}
	w2, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}

	a1 := joinN(t, w1, 3)
	a2 := joinN(t, w2, 3)

	// Force simultaneous respawns at different positions so item entity IDs are order-sensitive.
	for i := 0; i < 3; i++ {
		p := world.Vec3i{X: 100 + i*10, Y: 0, Z: -100 - i*7}
		if ok := w1.DebugSetAgentPos(a1[i], p); !ok {
			t.Fatalf("DebugSetAgentPos(w1) returned false")
		}
		if ok := w2.DebugSetAgentPos(a2[i], p); !ok {
			t.Fatalf("DebugSetAgentPos(w2) returned false")
		}
		if ok := w1.DebugAddInventory(a1[i], "PLANK", 10); !ok {
			t.Fatalf("DebugAddInventory(w1) returned false")
		}
		if ok := w2.DebugAddInventory(a2[i], "PLANK", 10); !ok {
			t.Fatalf("DebugAddInventory(w2) returned false")
		}
		if ok := w1.DebugSetAgentVitals(a1[i], 0, -1, -1); !ok {
			t.Fatalf("DebugSetAgentVitals(w1) returned false")
		}
		if ok := w2.DebugSetAgentVitals(a2[i], 0, -1, -1); !ok {
			t.Fatalf("DebugSetAgentVitals(w2) returned false")
		}
	}

	// One tick should respawn them deterministically.
	_, d1 := w1.StepOnce(nil, nil, nil)
	_, d2 := w2.StepOnce(nil, nil, nil)
	if d1 != d2 {
		t.Fatalf("digest mismatch after multi-agent respawn: %s vs %s", d1, d2)
	}
}

