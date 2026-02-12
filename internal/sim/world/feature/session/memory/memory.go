package memory

func OverMemoryBudget(mem map[string]string, key, val string, budget int) bool {
	total := 0
	for k, v := range mem {
		if k == key {
			continue
		}
		total += len(k) + len(v)
	}
	total += len(key) + len(val)
	return total > budget
}
