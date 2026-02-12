package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestSnapshotExport_ContainsRateLimitWindows(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := world.WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
		RateLimits: world.RateLimitConfig{
			SayWindowTicks: 50,
			SayMax:         5,
		},
	}

	h := NewHarness(t, cfg, cats, "bot")
	actTick := h.W.CurrentTick()

	inst := make([]protocol.InstantReq, 0, 5)
	for i := 1; i <= 5; i++ {
		inst = append(inst, protocol.InstantReq{
			ID:      "I" + string(rune('0'+i)),
			Type:    "SAY",
			Channel: "LOCAL",
			Text:    "hi",
		})
	}
	h.Step(inst, nil, nil)

	snap := h.W.ExportSnapshot(actTick)

	var av *snapshot.AgentV1
	for i := range snap.Agents {
		a := &snap.Agents[i]
		if a.ID == h.DefaultAgentID {
			av = a
			break
		}
	}
	if av == nil {
		t.Fatalf("missing agent in snapshot")
	}

	rw, ok := av.RateWindows["SAY"]
	if !ok {
		t.Fatalf("missing SAY rate window after export")
	}
	if got, want := rw.StartTick, actTick; got != want {
		t.Fatalf("StartTick: got %d want %d", got, want)
	}
	if got, want := rw.Count, 5; got != want {
		t.Fatalf("Count: got %d want %d", got, want)
	}
}
