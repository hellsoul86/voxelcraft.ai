package worldtest

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestInstant_EatRestoresStats(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:           "test",
		Seed:         42,
		StarterItems: map[string]int{"BERRIES": 2},
	}, cats, "eater")
	h.StepNoop()

	if ok := h.W.DebugSetAgentVitals(h.DefaultAgentID, 10, 0, 0); !ok {
		t.Fatalf("DebugSetAgentVitals returned false")
	}

	h.Step([]protocol.InstantReq{{ID: "I1", Type: "EAT", ItemID: "BERRIES", Count: 1}}, nil, nil)
	obs := h.LastObs()

	if got := invCount(obs.Inventory, "BERRIES"); got != 1 {
		t.Fatalf("berries: got %d want %d", got, 1)
	}
	if got := obs.Self.HP; got != 12 {
		t.Fatalf("hp: got %d want %d", got, 12)
	}
	if got := obs.Self.Hunger; got != 4 {
		t.Fatalf("hunger: got %d want %d", got, 4)
	}
	// Stamina is normalized in OBS.
	// Eating grants +100 stamina milli, then survival system applies recovery (+1 when hunger<5).
	if got := obs.Self.Stamina; math.Abs(got-0.101) > 1e-9 {
		t.Fatalf("stamina: got %v want %v", got, 0.101)
	}
}

func TestRespawn_OnHPZero_DropsItemsAndResetsVitals(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:           "test",
		Seed:         42,
		StarterItems: map[string]int{"PLANK": 10, "COAL": 10},
	}, cats, "victim")
	h.StepNoop()

	dropPos := world.Vec3i{X: 123, Y: 0, Z: -321}
	h.SetAgentPos(dropPos)
	if ok := h.W.DebugSetAgentVitals(h.DefaultAgentID, 0, 0, 0); !ok {
		t.Fatalf("DebugSetAgentVitals returned false")
	}

	// Next tick should respawn immediately.
	h.StepNoop()
	obs := h.LastObs()

	if obs.Self.HP != 20 || obs.Self.Hunger != 10 || obs.Self.Stamina != 1.0 {
		t.Fatalf("vitals after respawn: hp=%d hunger=%d stamina=%v", obs.Self.HP, obs.Self.Hunger, obs.Self.Stamina)
	}
	if got := invCount(obs.Inventory, "PLANK"); got != 7 {
		t.Fatalf("plank after drop: got %d want %d", got, 7)
	}
	if got := invCount(obs.Inventory, "COAL"); got != 7 {
		t.Fatalf("coal after drop: got %d want %d", got, 7)
	}
	if obs.Self.Pos == dropPos.ToArray() {
		t.Fatalf("expected respawn to move agent to spawn")
	}

	// Dropped items should exist as item entities at the downed position.
	// The agent respawns elsewhere, so validate via snapshot (not OBS.entities range).
	_, snap := h.Snapshot()
	var foundPlank, foundCoal bool
	for _, it := range snap.Items {
		if it.Pos != dropPos.ToArray() {
			continue
		}
		switch it.Item {
		case "PLANK":
			if it.Count != 3 {
				t.Fatalf("dropped PLANK count=%d want 3", it.Count)
			}
			foundPlank = true
		case "COAL":
			if it.Count != 3 {
				t.Fatalf("dropped COAL count=%d want 3", it.Count)
			}
			foundCoal = true
		}
	}
	if !foundPlank || !foundCoal {
		t.Fatalf("expected dropped PLANK and COAL at %v; snapshot.items=%v", dropPos, snap.Items)
	}

	found := false
	for _, e := range obs.Events {
		if e["type"] != "RESPAWN" {
			continue
		}
		if dp, ok := e["drop_pos"].([]interface{}); ok {
			// best-effort validation: [x,y,z]
			if len(dp) == 3 && int(dp[0].(float64)) == dropPos.X && int(dp[2].(float64)) == dropPos.Z {
				found = true
			}
		} else {
			// Some encoders may decode as []int in other contexts; accept presence.
			found = true
		}
		break
	}
	if !found {
		t.Fatalf("expected RESPAWN event; events=%v", obs.Events)
	}
}
