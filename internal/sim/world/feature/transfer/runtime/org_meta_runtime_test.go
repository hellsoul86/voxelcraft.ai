package runtime

import (
	"testing"

	transferorgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestApplyOrgMetaUpsert(t *testing.T) {
	orgs := map[string]*modelpkg.Organization{}
	agents := map[string]*modelpkg.Agent{
		"a1": {ID: "a1"},
	}
	ApplyOrgMetaUpsert(ApplyOrgMetaUpsertInput{
		Orgs:   orgs,
		Agents: agents,
		Incoming: []transferorgpkg.State{
			{
				OrgID: "ORG001",
				Kind:  string(modelpkg.OrgGuild),
				Name:  "Guild",
				Members: map[string]string{
					"a1": string(modelpkg.OrgLeader),
				},
			},
		},
	})

	if orgs["ORG001"] == nil {
		t.Fatalf("expected org upsert")
	}
	if agents["a1"].OrgID != "ORG001" {
		t.Fatalf("expected agent org assignment, got %q", agents["a1"].OrgID)
	}
}

func TestSnapshotOrgMeta(t *testing.T) {
	orgs := map[string]*modelpkg.Organization{
		"ORG1": {
			OrgID:   "ORG1",
			Kind:    modelpkg.OrgGuild,
			Name:    "A",
			Members: map[string]modelpkg.OrgRole{"a1": modelpkg.OrgMember},
		},
	}
	resp := SnapshotOrgMeta(orgs)
	if resp.Err != "" {
		t.Fatalf("unexpected error: %s", resp.Err)
	}
	if len(resp.Orgs) != 1 || resp.Orgs[0].OrgID != "ORG1" {
		t.Fatalf("unexpected snapshot: %+v", resp.Orgs)
	}
}
