package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestWanted_TagAndCityCoreBlock(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 5}, cats, "mayor")
	mayor := h.DefaultAgentID
	criminal := h.Join("criminal")

	// Create a CITY org.
	h.ClearAgentEventsFor(mayor)
	obs := h.StepFor(mayor, []protocol.InstantReq{{
		ID:      "I_create",
		Type:    "CREATE_ORG",
		OrgKind: "CITY",
		OrgName: "TestCity",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_create"); got != "" {
		t.Fatalf("CREATE_ORG expected ok, got code=%q events=%v", got, obs.Events)
	}
	orgID := actionResultFieldString(obs, "I_create", "org_id")
	if orgID == "" {
		t.Fatalf("missing org_id; events=%v", obs.Events)
	}

	anchorArr := h.LastObsFor(mayor).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	clearArea(t, h, anchor, 2)

	// Claim a small land parcel near the mayor and deed it to the CITY org.
	h.AddInventoryFor(mayor, "BATTERY", 1)
	h.AddInventoryFor(mayor, "CRYSTAL_SHARD", 1)
	h.ClearAgentEventsFor(mayor)
	obs = h.StepFor(mayor, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 8,
	}}, nil)
	if got := actionResultCode(obs, "K_claim"); got != "" {
		t.Fatalf("CLAIM_LAND expected ok, got code=%q events=%v", got, obs.Events)
	}
	landID := actionResultFieldString(obs, "K_claim", "land_id")
	if landID == "" {
		t.Fatalf("missing land_id; events=%v", obs.Events)
	}

	h.ClearAgentEventsFor(mayor)
	obs = h.StepFor(mayor, []protocol.InstantReq{{
		ID:       "I_deed",
		Type:     "DEED_LAND",
		LandID:   landID,
		NewOwner: orgID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_deed"); got != "" {
		t.Fatalf("DEED_LAND expected ok, got code=%q events=%v", got, obs.Events)
	}

	// Place the criminal just outside the core (core radius == claim radius here) and mark as wanted.
	startX := anchor.X + 9 // outside claim/core (radius=8) but within OBS entity radius 16
	crimPos := world.Vec3i{X: startX, Y: 0, Z: anchor.Z}
	h.SetAgentPosFor(criminal, crimPos)
	h.SetAgentReputationFor(criminal, -1, -1, -1, 100) // wanted: 0 < RepLaw < 200
	// Ensure a clear straight line to the core entry cell.
	for x := anchor.X; x <= startX; x++ {
		h.SetBlock(world.Vec3i{X: x, Y: 0, Z: anchor.Z}, "AIR")
	}

	h.StepNoop()

	// Mayor should see the criminal as "wanted".
	obsMayor := h.LastObsFor(mayor)
	foundWanted := false
	for _, e := range obsMayor.Entities {
		if e.Type != "AGENT" || e.ID != criminal {
			continue
		}
		for _, tag := range e.Tags {
			if tag == "wanted" {
				foundWanted = true
				break
			}
		}
	}
	if !foundWanted {
		t.Fatalf("expected wanted tag in OBS entities: entities=%v", obsMayor.Entities)
	}

	// Movement into CITY core should be blocked for wanted agents (non-members).
	h.ClearAgentEventsFor(criminal)
	obsCrim := h.StepFor(criminal, nil, []protocol.TaskReq{{
		ID:        "K_move",
		Type:      "MOVE_TO",
		Target:    anchor.ToArray(),
		Tolerance: 1.2,
	}}, nil)
	if !hasTaskFail(obsCrim, "E_NO_PERMISSION") {
		t.Fatalf("expected TASK_FAIL E_NO_PERMISSION; events=%v", obsCrim.Events)
	}
	for _, task := range obsCrim.Tasks {
		if task.Kind == "MOVE_TO" {
			t.Fatalf("expected move task to fail and be cleared; tasks=%v", obsCrim.Tasks)
		}
	}
}

func TestCityCore_AllowsNonWanted(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 6}, cats, "mayor")
	mayor := h.DefaultAgentID
	visitor := h.Join("visitor")

	// CITY org + deeded claim.
	obs := h.StepFor(mayor, []protocol.InstantReq{{
		ID:      "I_create",
		Type:    "CREATE_ORG",
		OrgKind: "CITY",
		OrgName: "TestCity",
	}}, nil, nil)
	orgID := actionResultFieldString(obs, "I_create", "org_id")
	if orgID == "" {
		t.Fatalf("missing org_id; events=%v", obs.Events)
	}

	anchorArr := h.LastObsFor(mayor).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	clearArea(t, h, anchor, 2)

	h.AddInventoryFor(mayor, "BATTERY", 1)
	h.AddInventoryFor(mayor, "CRYSTAL_SHARD", 1)
	obs = h.StepFor(mayor, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 8,
	}}, nil)
	landID := actionResultFieldString(obs, "K_claim", "land_id")
	if landID == "" {
		t.Fatalf("missing land_id; events=%v", obs.Events)
	}
	_ = h.StepFor(mayor, []protocol.InstantReq{{
		ID:       "I_deed",
		Type:     "DEED_LAND",
		LandID:   landID,
		NewOwner: orgID,
	}}, nil, nil)

	// Visitor outside claim, but not wanted -> should be able to step into core.
	startX := anchor.X + 9
	h.SetAgentPosFor(visitor, world.Vec3i{X: startX, Y: 0, Z: anchor.Z})
	for x := anchor.X; x <= startX; x++ {
		h.SetBlock(world.Vec3i{X: x, Y: 0, Z: anchor.Z}, "AIR")
	}

	obsVis := h.StepFor(visitor, nil, []protocol.TaskReq{{
		ID:        "K_move",
		Type:      "MOVE_TO",
		Target:    anchor.ToArray(),
		Tolerance: 1.2,
	}}, nil)

	if got := obsVis.Self.Pos[0]; got != startX-1 {
		t.Fatalf("expected visitor to move one step toward core; pos=%v startX=%d", obsVis.Self.Pos, startX)
	}
}

