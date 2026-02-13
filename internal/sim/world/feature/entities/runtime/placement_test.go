package runtime

import "testing"

func TestEffectsForPlacedBlock(t *testing.T) {
	cases := []struct {
		block string
		check func(PlacementEffects) bool
	}{
		{"CHEST", func(e PlacementEffects) bool { return e.ContainerType == "CHEST" }},
		{"BULLETIN_BOARD", func(e PlacementEffects) bool { return e.EnsureBoard }},
		{"SIGN", func(e PlacementEffects) bool { return e.EnsureSign }},
		{"CONVEYOR", func(e PlacementEffects) bool { return e.EnsureConveyor && e.ConveyorDX == 1 && e.ConveyorDZ == 0 }},
		{"SWITCH", func(e PlacementEffects) bool { return e.EnsureSwitch && !e.SwitchOn }},
		{"PLANK", func(e PlacementEffects) bool { return e == (PlacementEffects{}) }},
	}
	for _, tc := range cases {
		got := EffectsForPlacedBlock(tc.block)
		if !tc.check(got) {
			t.Fatalf("unexpected effects for %s: %+v", tc.block, got)
		}
	}
}
