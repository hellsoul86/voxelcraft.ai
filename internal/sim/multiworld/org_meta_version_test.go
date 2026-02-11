package multiworld

import (
	"testing"

	"voxelcraft.ai/internal/sim/world"
)

func TestChooseNewerOrgMeta_ByMetaVersion(t *testing.T) {
	old := OrgMeta{
		OrgID:       "ORG1",
		Kind:        world.OrgGuild,
		Name:        "G",
		CreatedTick: 10,
		MetaVersion: 2,
		Members:     map[string]world.OrgRole{"A": world.OrgLeader},
	}
	newer := old
	newer.MetaVersion = 3
	newer.Members = map[string]world.OrgRole{"A": world.OrgLeader, "B": world.OrgMember}
	if !chooseNewerOrgMeta(old, newer) {
		t.Fatalf("expected higher meta version to win")
	}
	if chooseNewerOrgMeta(newer, old) {
		t.Fatalf("expected lower meta version to lose")
	}
}

func TestChooseNewerOrgMeta_TieBreakDeterministic(t *testing.T) {
	a := OrgMeta{
		OrgID:       "ORG1",
		Kind:        world.OrgGuild,
		Name:        "G",
		MetaVersion: 7,
		Members:     map[string]world.OrgRole{"A": world.OrgLeader},
	}
	b := OrgMeta{
		OrgID:       "ORG1",
		Kind:        world.OrgGuild,
		Name:        "G",
		MetaVersion: 7,
		Members:     map[string]world.OrgRole{"A": world.OrgLeader, "B": world.OrgMember},
	}
	if !chooseNewerOrgMeta(a, b) {
		t.Fatalf("expected deterministic tie-break to choose b")
	}
	if chooseNewerOrgMeta(b, a) {
		t.Fatalf("expected deterministic tie-break not to choose a over b")
	}
}
