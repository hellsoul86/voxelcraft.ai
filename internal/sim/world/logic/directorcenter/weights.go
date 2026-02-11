package directorcenter

import "sort"

func SampleWeighted(weights map[string]float64, roll uint64) string {
	if len(weights) == 0 {
		return ""
	}
	ids := make([]string, 0, len(weights))
	var total float64
	for id, w := range weights {
		if w > 0 {
			ids = append(ids, id)
			total += w
		}
	}
	if total <= 0 || len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)

	// Deterministic pick in [0,total).
	r := float64(roll%1_000_000_000) / 1_000_000_000.0
	target := r * total

	var acc float64
	for _, id := range ids {
		acc += weights[id]
		if target <= acc {
			return id
		}
	}
	return ids[len(ids)-1]
}
