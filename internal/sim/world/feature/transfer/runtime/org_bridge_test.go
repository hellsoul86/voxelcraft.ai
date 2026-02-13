package runtime

import (
	"testing"

	orgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
)

func TestOrgStateTransferRoundtrip(t *testing.T) {
	in := []orgpkg.State{{
		OrgID:       "ORG_1",
		Kind:        "GUILD",
		Name:        "g1",
		CreatedTick: 5,
		MetaVersion: 2,
		Members: map[string]string{
			"A1": "LEADER",
		},
	}}
	ts := TransfersFromStates(in)
	if len(ts) != 1 || ts[0].OrgID != "ORG_1" {
		t.Fatalf("unexpected transfers: %+v", ts)
	}
	back := StatesFromTransfers(ts)
	if len(back) != 1 || back[0].Members["A1"] != "LEADER" {
		t.Fatalf("unexpected states after roundtrip: %+v", back)
	}
}
