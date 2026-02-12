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
