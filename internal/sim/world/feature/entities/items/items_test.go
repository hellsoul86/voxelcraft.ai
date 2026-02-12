package items

import "testing"

func TestFindMergeTarget(t *testing.T) {
	m := map[string]Entry{
		"I1": {ID: "I1", Item: "COAL", Count: 2},
		"I2": {ID: "I2", Item: "IRON_ORE", Count: 1},
	}
	id, ok := FindMergeTarget([]string{"I2", "I1"}, "COAL", func(s string) (Entry, bool) {
		e, ok := m[s]
		return e, ok
	})
	if !ok || id != "I1" {
		t.Fatalf("expected merge target I1, got %q ok=%v", id, ok)
	}
}

func TestRemoveID(t *testing.T) {
	got := RemoveID([]string{"A", "B", "C"}, "B")
	if len(got) != 2 || got[0] != "A" || got[1] != "C" {
		t.Fatalf("unexpected ids: %#v", got)
	}
}

func TestSortedExpired(t *testing.T) {
	m := map[string]Entry{
		"I2": {ID: "I2", ExpiresTick: 10},
		"I1": {ID: "I1", ExpiresTick: 11},
	}
	exp := SortedExpired([]string{"I1", "I2"}, func(s string) (Entry, bool) {
		e, ok := m[s]
		return e, ok
	}, 10)
	if len(exp) != 1 || exp[0] != "I2" {
		t.Fatalf("unexpected expired ids: %#v", exp)
	}
}
