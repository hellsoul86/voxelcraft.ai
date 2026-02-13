package debug

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestSetAgentPosForces2D(t *testing.T) {
	a := &modelpkg.Agent{}
	if !SetAgentPos(a, modelpkg.Vec3i{X: 3, Y: 9, Z: -4}) {
		t.Fatalf("expected success")
	}
	if a.Pos.Y != 0 || a.Pos.X != 3 || a.Pos.Z != -4 {
		t.Fatalf("unexpected pos: %+v", a.Pos)
	}
}

func TestAddInventoryWithValidation(t *testing.T) {
	a := &modelpkg.Agent{Inventory: map[string]int{"PLANK": 2}}
	allow := func(item string) bool { return item == "PLANK" || item == "STICK" }

	if !AddInventory(a, "STICK", 3, allow) {
		t.Fatalf("expected allowed item")
	}
	if a.Inventory["STICK"] != 3 {
		t.Fatalf("unexpected stick count: %d", a.Inventory["STICK"])
	}

	if AddInventory(a, "INVALID", 1, allow) {
		t.Fatalf("expected invalid item to be rejected")
	}
	if _, ok := a.Inventory["INVALID"]; ok {
		t.Fatalf("invalid item should not be inserted")
	}
}
