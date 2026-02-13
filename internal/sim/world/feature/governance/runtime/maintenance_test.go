package runtime

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestTickClaimsMaintenance(t *testing.T) {
	claims := map[string]*modelpkg.LandClaim{
		"L1": {LandID: "L1", Owner: "a1", MaintenanceDueTick: 10},
	}
	paid := false
	TickClaimsMaintenance(TickClaimsMaintenanceInput{
		NowTick:  10,
		DayTicks: 100,
		Claims:   claims,
	}, TickClaimsMaintenanceHooks{
		Pay: func(c *modelpkg.LandClaim) bool {
			paid = true
			return false
		},
	})
	if !paid {
		t.Fatalf("expected pay hook to be called")
	}
	c := claims["L1"]
	if c.MaintenanceStage != 1 {
		t.Fatalf("expected stage 1, got %d", c.MaintenanceStage)
	}
	if c.MaintenanceDueTick <= 10 {
		t.Fatalf("expected due tick to advance, got %d", c.MaintenanceDueTick)
	}
}

func TestPayMaintenanceAgentAndOrg(t *testing.T) {
	org := &modelpkg.Organization{
		OrgID:           "ORG1",
		TreasuryByWorld: map[string]map[string]int{"W": {"IRON_INGOT": 2}},
	}
	agent := &modelpkg.Agent{ID: "a1", Inventory: map[string]int{"IRON_INGOT": 2}}
	cost := map[string]int{"IRON_INGOT": 1}

	okOrg := PayMaintenance(PayMaintenanceInput{
		Claim:       &modelpkg.LandClaim{Owner: "ORG1"},
		DefaultCost: cost,
		GetOrg: func(owner string) *modelpkg.Organization {
			if owner == "ORG1" {
				return org
			}
			return nil
		},
		OrgTreasury: func(o *modelpkg.Organization) map[string]int {
			return o.TreasuryFor("W")
		},
		GetAgent: func(string) *modelpkg.Agent { return nil },
	})
	if !okOrg {
		t.Fatalf("expected org maintenance payment")
	}

	okAgent := PayMaintenance(PayMaintenanceInput{
		Claim:       &modelpkg.LandClaim{Owner: "a1"},
		DefaultCost: cost,
		GetOrg:      func(string) *modelpkg.Organization { return nil },
		OrgTreasury: nil,
		GetAgent: func(owner string) *modelpkg.Agent {
			if owner == "a1" {
				return agent
			}
			return nil
		},
	})
	if !okAgent {
		t.Fatalf("expected agent maintenance payment")
	}
}
