package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestSnapshotExport_FiltersExpiredAgentMemory(t *testing.T) {
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
	}

	h := NewHarness(t, cfg, cats, "bot")
	actTick := h.W.CurrentTick()

	h.Step([]protocol.InstantReq{
		{ID: "I1", Type: "SAVE_MEMORY", Key: "keep", Value: "v", TTLTicks: 100},
		{ID: "I2", Type: "SAVE_MEMORY", Key: "expired", Value: "x", TTLTicks: 1},
	}, nil, nil)

	// Export at tick=actTick+1 so the second key is expired (expiry == actTick+1).
	snapTick := actTick + 1
	snap := h.W.ExportSnapshot(snapTick)

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

	keep, ok := av.Memory["keep"]
	if !ok || keep.Value != "v" || keep.ExpiryTick != actTick+100 {
		t.Fatalf("keep memory not exported: ok=%v keep=%+v actTick=%d", ok, keep, actTick)
	}
	if _, ok := av.Memory["expired"]; ok {
		t.Fatalf("expected expired key to be filtered on snapshot export")
	}
}
