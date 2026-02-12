package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestToggleSwitch_TogglesState(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 42}, cats, "bot")

	posArr := h.LastObs().Self.Pos
	pos := world.Vec3i{X: posArr[0], Y: 0, Z: posArr[2]}
	h.SetBlock(pos, "AIR")

	h.AddInventory("SWITCH", 1)
	obs := h.Step(nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "SWITCH",
		BlockPos: pos.ToArray(),
	}}, nil)

	switchID := findEntityIDAt(obs, "SWITCH", pos.ToArray())
	if switchID == "" {
		t.Fatalf("expected SWITCH entity at %v; entities=%v", pos, obs.Entities)
	}
	if !switchHasState(obs, switchID, "off") {
		t.Fatalf("expected default state off; entity=%v", switchEntity(obs, switchID))
	}

	h.ClearAgentEvents()
	obs = h.Step([]protocol.InstantReq{{
		ID:       "I_toggle1",
		Type:     "TOGGLE_SWITCH",
		TargetID: switchID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_toggle1"); got != "" {
		t.Fatalf("toggle expected ok, got code=%q events=%v", got, obs.Events)
	}
	if !switchHasState(obs, switchID, "on") {
		t.Fatalf("expected state on after toggle; entity=%v", switchEntity(obs, switchID))
	}

	h.ClearAgentEvents()
	obs = h.Step([]protocol.InstantReq{{
		ID:       "I_toggle2",
		Type:     "TOGGLE_SWITCH",
		TargetID: switchID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_toggle2"); got != "" {
		t.Fatalf("toggle expected ok, got code=%q events=%v", got, obs.Events)
	}
	if !switchHasState(obs, switchID, "off") {
		t.Fatalf("expected state off after second toggle; entity=%v", switchEntity(obs, switchID))
	}
}

func TestSnapshotExportImport_SwitchStateRoundTrip(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	cfg := world.WorldConfig{ID: "test", Seed: 42}
	h1 := NewHarness(t, cfg, cats, "bot")

	pos := world.Vec3i{X: 10, Y: 0, Z: 10}
	h1.SetAgentPos(pos)
	h1.SetBlock(pos, "AIR")
	h1.AddInventory("SWITCH", 1)
	obs := h1.Step(nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "SWITCH",
		BlockPos: pos.ToArray(),
	}}, nil)
	switchID := findEntityIDAt(obs, "SWITCH", pos.ToArray())
	if switchID == "" {
		t.Fatalf("expected SWITCH entity after placement; entities=%v", obs.Entities)
	}
	h1.Step([]protocol.InstantReq{{
		ID:       "I_toggle",
		Type:     "TOGGLE_SWITCH",
		TargetID: switchID,
	}}, nil, nil)

	_, snap := h1.Snapshot()

	w2, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world.New: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("ImportSnapshot: %v", err)
	}
	h2 := NewHarnessWithWorld(t, w2, cats, "observer")
	h2.SetAgentPos(pos)
	h2.StepNoop()

	obs2 := h2.LastObs()
	switchID2 := findEntityIDAt(obs2, "SWITCH", pos.ToArray())
	if switchID2 == "" {
		t.Fatalf("expected SWITCH entity after import; entities=%v", obs2.Entities)
	}
	if !switchHasState(obs2, switchID2, "on") {
		t.Fatalf("expected state on after import; entity=%v", switchEntity(obs2, switchID2))
	}
}

func switchEntity(obs protocol.ObsMsg, id string) *protocol.EntityObs {
	for i := range obs.Entities {
		e := obs.Entities[i]
		if e.ID == id {
			return &e
		}
	}
	return nil
}

func switchHasState(obs protocol.ObsMsg, id string, want string) bool {
	for _, e := range obs.Entities {
		if e.ID != id {
			continue
		}
		for _, tag := range e.Tags {
			if tag == "state:"+want {
				return true
			}
		}
		return false
	}
	return false
}

