package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestSign_SetAndOpenAndObsEntity(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 42}, cats, "writer")

	self := h.LastObs().Self.Pos
	signPos := world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}
	h.SetBlock(signPos, "AIR")

	h.AddInventory("SIGN", 1)
	obs := h.Step(nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "SIGN",
		BlockPos: signPos.ToArray(),
	}}, nil)

	signID := findEntityIDAt(obs, "SIGN", signPos.ToArray())
	if signID == "" {
		t.Fatalf("expected SIGN entity at %v; entities=%v", signPos, obs.Entities)
	}

	// Sign should start without has_text tag.
	for _, e := range obs.Entities {
		if e.ID != signID {
			continue
		}
		for _, tag := range e.Tags {
			if tag == "has_text" {
				t.Fatalf("unexpected has_text tag before SET_SIGN: %+v", e)
			}
		}
	}

	h.ClearAgentEvents()
	obs = h.Step([]protocol.InstantReq{{
		ID:       "I_set",
		Type:     "SET_SIGN",
		TargetID: signID,
		Text:     "Hello Sign",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_set"); got != "" {
		t.Fatalf("SET_SIGN expected ok, got code=%q events=%v", got, obs.Events)
	}

	// Sign entity should now include has_text tag.
	foundHasText := false
	for _, e := range obs.Entities {
		if e.ID != signID {
			continue
		}
		for _, tag := range e.Tags {
			if tag == "has_text" {
				foundHasText = true
				break
			}
		}
	}
	if !foundHasText {
		t.Fatalf("expected has_text tag after SET_SIGN; entities=%v", obs.Entities)
	}

	h.ClearAgentEvents()
	obs = h.Step(nil, []protocol.TaskReq{{
		ID:       "K_open",
		Type:     "OPEN",
		TargetID: signID,
	}}, nil)

	found := false
	for _, e := range obs.Events {
		if e["type"] == "SIGN" && e["sign_id"] == signID {
			if e["text"] != "Hello Sign" {
				t.Fatalf("unexpected SIGN event text: %+v", e)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected SIGN event from OPEN; events=%v", obs.Events)
	}
}

func TestSign_SetPermissionDeniedForVisitorInsideClaim(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", WorldType: "OVERWORLD", Seed: 42}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")
	h.AddInventoryFor(owner, "BATTERY", 1)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 1)

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q events=%v", got, h.LastObsFor(owner).Events)
	}

	// Place a sign inside the claim.
	signPos := world.Vec3i{X: anchor.X + 1, Y: 0, Z: anchor.Z}
	h.SetBlock(signPos, "AIR")
	h.AddInventoryFor(owner, "SIGN", 1)
	obsOwner := h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "SIGN",
		BlockPos: signPos.ToArray(),
	}}, nil)
	signID := findEntityIDAt(obsOwner, "SIGN", signPos.ToArray())
	if signID == "" {
		t.Fatalf("expected SIGN entity after placement; entities=%v", obsOwner.Entities)
	}

	// Visitor attempts to edit sign and should be denied.
	h.SetAgentPosFor(visitor, signPos)
	h.ClearAgentEventsFor(visitor)
	obsV := h.StepFor(visitor, []protocol.InstantReq{{
		ID:       "I_set",
		Type:     "SET_SIGN",
		TargetID: signID,
		Text:     "nope",
	}}, nil, nil)
	if got := actionResultCode(obsV, "I_set"); got != "E_NO_PERMISSION" {
		t.Fatalf("expected E_NO_PERMISSION, got code=%q events=%v", got, obsV.Events)
	}
}

func TestSign_SnapshotRoundTrip_PreservesText(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	cfg := world.WorldConfig{ID: "test", Seed: 42}
	h1 := NewHarness(t, cfg, cats, "writer")

	// Place and write a sign.
	signPos := world.Vec3i{X: 10, Y: 0, Z: 10}
	h1.SetAgentPos(signPos)
	h1.SetBlock(signPos, "AIR")
	h1.AddInventory("SIGN", 1)
	obs := h1.Step(nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "SIGN",
		BlockPos: signPos.ToArray(),
	}}, nil)
	signID := findEntityIDAt(obs, "SIGN", signPos.ToArray())
	if signID == "" {
		t.Fatalf("expected SIGN entity after placement; entities=%v", obs.Entities)
	}
	h1.Step([]protocol.InstantReq{{
		ID:       "I_set",
		Type:     "SET_SIGN",
		TargetID: signID,
		Text:     "persist me",
	}}, nil, nil)

	_, snap := h1.Snapshot()

	// Import into a fresh world and validate OPEN returns the same text.
	w2, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world.New: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("ImportSnapshot: %v", err)
	}
	h2 := NewHarnessWithWorld(t, w2, cats, "reader")
	h2.SetAgentPos(signPos)
	h2.StepNoop()

	signID2 := findEntityIDAt(h2.LastObs(), "SIGN", signPos.ToArray())
	if signID2 == "" {
		t.Fatalf("expected SIGN entity after import; entities=%v", h2.LastObs().Entities)
	}

	h2.ClearAgentEvents()
	obs2 := h2.Step(nil, []protocol.TaskReq{{
		ID:       "K_open",
		Type:     "OPEN",
		TargetID: signID2,
	}}, nil)

	found := false
	for _, e := range obs2.Events {
		if e["type"] == "SIGN" && e["sign_id"] == signID2 {
			if e["text"] != "persist me" {
				t.Fatalf("unexpected SIGN event text after import: %+v", e)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected SIGN event after import; events=%v", obs2.Events)
	}
}

