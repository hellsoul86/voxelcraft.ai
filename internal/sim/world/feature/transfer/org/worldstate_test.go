package org

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestStatesFromOrganizations(t *testing.T) {
	orgs := map[string]*modelpkg.Organization{
		"O1": {
			OrgID: "O1",
			Kind:  modelpkg.OrgGuild,
			Name:  "g",
			Members: map[string]modelpkg.OrgRole{
				"A1": modelpkg.OrgMember,
			},
		},
	}
	states := StatesFromOrganizations(orgs)
	if len(states) != 1 || states[0].OrgID != "O1" {
		t.Fatalf("unexpected states: %+v", states)
	}
}

func TestApplyStatesAndReconcileAgents(t *testing.T) {
	orgs := map[string]*modelpkg.Organization{}
	ApplyStates(orgs, []State{{
		OrgID: "O2",
		Kind:  string(modelpkg.OrgCity),
		Name:  "city",
		Members: map[string]string{
			"A1": string(modelpkg.OrgLeader),
		},
	}}, nil)
	if orgs["O2"] == nil || orgs["O2"].Kind != modelpkg.OrgCity {
		t.Fatalf("failed to apply states")
	}

	agents := map[string]*modelpkg.Agent{
		"A1": {ID: "A1", OrgID: ""},
		"A2": {ID: "A2", OrgID: "O2"},
	}
	ReconcileAgentsOrg(agents, orgs, map[string]string{"A1": "O2"})
	if agents["A1"].OrgID != "O2" {
		t.Fatalf("expected owner map assignment")
	}
	if agents["A2"].OrgID != "" {
		t.Fatalf("expected stale membership cleared")
	}
}
