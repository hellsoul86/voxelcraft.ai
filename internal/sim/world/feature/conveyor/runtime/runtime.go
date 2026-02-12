package runtime

import "sort"

type Pos struct {
	X int
	Y int
	Z int
}

type ItemEntry struct {
	ID    string
	Item  string
	Count int
}

func SortedLiveItemIDs(items map[string]ItemEntry) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for id, e := range items {
		if id == "" || e.Item == "" || e.Count <= 0 {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func SortedPositions[T any](m map[Pos]T) []Pos {
	if len(m) == 0 {
		return nil
	}
	out := make([]Pos, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].Z < out[j].Z
	})
	return out
}

func HasLiveItem(ids []string, load func(string) (ItemEntry, bool)) bool {
	if load == nil {
		return false
	}
	for _, id := range ids {
		e, ok := load(id)
		if !ok {
			continue
		}
		if e.Item != "" && e.Count > 0 {
			return true
		}
	}
	return false
}

func PickAvailableItem(inventory map[string]int, availableCount func(string) int) string {
	if len(inventory) == 0 || availableCount == nil {
		return ""
	}
	keys := make([]string, 0, len(inventory))
	for item, n := range inventory {
		if item == "" || n <= 0 {
			continue
		}
		if availableCount(item) <= 0 {
			continue
		}
		keys = append(keys, item)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return keys[0]
}

func SensorNeighborOffsets() []Pos {
	return []Pos{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: -1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: 0, Y: -1, Z: 0},
		{X: 0, Y: 0, Z: 1},
		{X: 0, Y: 0, Z: -1},
	}
}
