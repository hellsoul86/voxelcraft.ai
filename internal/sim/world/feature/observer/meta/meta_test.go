package meta

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestAttachObsEventsAndMeta(t *testing.T) {
	a := &modelpkg.Agent{
		ID:     "A1",
		Events: []protocol.Event{{"type": "TEST", "t": uint64(1)}},
		Fun:    modelpkg.FunScore{Novelty: 2},
		PendingMemory: []protocol.MemoryKV{
			{Key: "k", Value: "v"},
		},
	}
	a.EventCursor = 7
	obs := &protocol.ObsMsg{}
	AttachObsEventsAndMeta(a, obs, 100)

	if obs.ObsID == "" || obs.EventsCursor != 7 {
		t.Fatalf("expected obs id/cursor set, got id=%q cursor=%d", obs.ObsID, obs.EventsCursor)
	}
	if len(obs.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(obs.Events))
	}
	if obs.FunScore == nil || obs.FunScore.Novelty != 2 {
		t.Fatalf("expected fun score novelty=2")
	}
	if len(obs.Memory) != 1 || obs.Memory[0].Key != "k" {
		t.Fatalf("expected memory included")
	}
	if len(a.PendingMemory) != 0 {
		t.Fatalf("expected pending memory cleared")
	}
}
