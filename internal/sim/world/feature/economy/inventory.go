package economy

func DeductItems(inv map[string]int, cost map[string]int) {
	for item, c := range cost {
		if item == "" || c <= 0 {
			continue
		}
		inv[item] -= c
		if inv[item] <= 0 {
			delete(inv, item)
		}
	}
}
