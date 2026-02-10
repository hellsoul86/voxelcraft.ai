package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestInstant_EatRestoresStats(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "eater", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	a.Inventory["BERRIES"] = 2
	a.HP = 10
	a.Hunger = 0
	a.StaminaMilli = 0

	w.applyInstant(a, protocol.InstantReq{ID: "I1", Type: "EAT", ItemID: "BERRIES", Count: 1}, 0)

	if got := a.Inventory["BERRIES"]; got != 1 {
		t.Fatalf("berries: got %d want %d", got, 1)
	}
	if got := a.HP; got != 12 {
		t.Fatalf("hp: got %d want %d", got, 12)
	}
	if got := a.Hunger; got != 4 {
		t.Fatalf("hunger: got %d want %d", got, 4)
	}
	if got := a.StaminaMilli; got != 100 {
		t.Fatalf("stamina: got %d want %d", got, 100)
	}
}

func TestRespawn_OnHPZero_DropsItemsAndResetsVitals(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "victim", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	a.Inventory = map[string]int{"PLANK": 10, "COAL": 10}
	a.Pos = Vec3i{X: 123, Y: 40, Z: -321}
	dropPos := a.Pos
	a.HP = 1
	a.Hunger = 0
	a.StaminaMilli = 0

	// Starvation at tick 0 will reduce HP to 0 and trigger respawn.
	w.weather = "CLEAR"
	w.systemEnvironment(0)

	if a.HP != 20 || a.Hunger != 10 || a.StaminaMilli != 1000 {
		t.Fatalf("vitals after respawn: hp=%d hunger=%d stamina=%d", a.HP, a.Hunger, a.StaminaMilli)
	}
	if got := a.Inventory["PLANK"]; got != 7 {
		t.Fatalf("plank after drop: got %d want %d", got, 7)
	}
	if got := a.Inventory["COAL"]; got != 7 {
		t.Fatalf("coal after drop: got %d want %d", got, 7)
	}
	if a.Pos == (Vec3i{X: 123, Y: 40, Z: -321}) {
		t.Fatalf("expected respawn to move agent to spawn")
	}

	// Dropped items should exist as item entities at the downed position.
	if len(w.items) == 0 {
		t.Fatalf("expected item entities for dropped inventory")
	}
	var foundPlank, foundCoal bool
	for _, e := range w.items {
		if e == nil || e.Pos != dropPos {
			continue
		}
		switch e.Item {
		case "PLANK":
			if e.Count != 3 {
				t.Fatalf("dropped PLANK count=%d want 3", e.Count)
			}
			foundPlank = true
		case "COAL":
			if e.Count != 3 {
				t.Fatalf("dropped COAL count=%d want 3", e.Count)
			}
			foundCoal = true
		}
	}
	if !foundPlank || !foundCoal {
		t.Fatalf("expected dropped PLANK and COAL at %v", dropPos)
	}

	found := false
	for _, e := range a.Events {
		if e["type"] == "RESPAWN" {
			if dp, ok := e["drop_pos"].([3]int); ok {
				if dp != dropPos.ToArray() {
					t.Fatalf("RESPAWN drop_pos mismatch: got %v want %v", dp, dropPos.ToArray())
				}
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected RESPAWN event")
	}
}
