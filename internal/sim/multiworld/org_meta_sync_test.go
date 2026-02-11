package multiworld

import (
	"path/filepath"
	"testing"

	"voxelcraft.ai/internal/sim/world"
)

func TestOrgMetaSync_MergeAndAttachAcrossWorlds(t *testing.T) {
	runtimes, stop := testRuntimes(t)
	defer stop()

	cfg := testManagerConfig()
	mgr, err := NewManager(cfg, runtimes, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	// World1 transfer advertises initial org and leader.
	mgr.mergeOrgMetaFromTransfer(&world.OrgTransfer{
		OrgID:       "ORG100001",
		Kind:        world.OrgGuild,
		Name:        "SyncGuild",
		CreatedTick: 1,
		Members: map[string]world.OrgRole{
			"A1": world.OrgLeader,
		},
	})

	// World2 adds another member; manager should converge to union.
	mgr.mergeOrgMetaFromTransfer(&world.OrgTransfer{
		OrgID:       "ORG100001",
		Kind:        world.OrgGuild,
		Name:        "SyncGuild",
		CreatedTick: 1,
		Members: map[string]world.OrgRole{
			"A2": world.OrgMember,
		},
	})

	tr := &world.AgentTransfer{OrgID: "ORG100001"}
	mgr.attachOrgMetaToTransfer(tr)
	if tr.Org == nil {
		t.Fatalf("expected org metadata attached to transfer")
	}
	if _, ok := tr.Org.Members["A1"]; !ok {
		t.Fatalf("expected member A1 in converged org metadata")
	}
	if _, ok := tr.Org.Members["A2"]; !ok {
		t.Fatalf("expected member A2 in converged org metadata")
	}
}
