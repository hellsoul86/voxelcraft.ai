package items

import "sort"

type Entry struct {
	ID          string
	Item        string
	Count       int
	ExpiresTick uint64
}

func FindMergeTarget(ids []string, item string, load func(string) (Entry, bool)) (string, bool) {
	if load == nil || item == "" {
		return "", false
	}
	for _, id := range ids {
		e, ok := load(id)
		if !ok {
			continue
		}
		if e.Item == item && e.Count > 0 {
			return e.ID, true
		}
	}
	return "", false
}

func RemoveID(ids []string, id string) []string {
	for i := 0; i < len(ids); i++ {
		if ids[i] != id {
			continue
		}
		copy(ids[i:], ids[i+1:])
		return ids[:len(ids)-1]
	}
	return ids
}

func SortedExpired(ids []string, load func(string) (Entry, bool), nowTick uint64) []string {
	out := make([]string, 0)
	if load == nil {
		return out
	}
	for _, id := range ids {
		e, ok := load(id)
		if !ok {
			continue
		}
		if e.ExpiresTick != 0 && nowTick >= e.ExpiresTick {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
