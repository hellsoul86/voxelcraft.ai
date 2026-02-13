package runtime

import "testing"

func TestPickAvailableItem(t *testing.T) {
	inv := map[string]int{"B": 1, "A": 2}
	got := PickAvailableItem(inv, func(item string) int {
		if item == "A" {
			return 0
		}
		return 1
	})
	if got != "B" {
		t.Fatalf("PickAvailableItem=%q, want B", got)
	}
}

func TestSortedLiveItemIDs(t *testing.T) {
	got := SortedLiveItemIDs(map[string]ItemEntry{
		"z": {ID: "z", Item: "COAL", Count: 1},
		"a": {ID: "a", Item: "COAL", Count: 0},
		"b": {ID: "b", Item: "IRON", Count: 2},
	})
	if len(got) != 2 || got[0] != "b" || got[1] != "z" {
		t.Fatalf("SortedLiveItemIDs=%v", got)
	}
}

func TestDirectionTag(t *testing.T) {
	if got := DirectionTag(1, 0); got != "+X" {
		t.Fatalf("DirectionTag +X=%q", got)
	}
	if got := DirectionTag(0, -1); got != "-Z" {
		t.Fatalf("DirectionTag -Z=%q", got)
	}
}

func TestYawToDir(t *testing.T) {
	dx, dz := YawToDir(90)
	if dx != 1 || dz != 0 {
		t.Fatalf("YawToDir(90)=(%d,%d)", dx, dz)
	}
	dx, dz = YawToDir(-10)
	if dx != 0 || dz != 1 {
		t.Fatalf("YawToDir(-10)=(%d,%d)", dx, dz)
	}
}
