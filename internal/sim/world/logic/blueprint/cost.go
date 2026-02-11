package blueprint

type ItemCount struct {
	Item  string
	Count int
}

func RemainingCost(cost []ItemCount, alreadyCorrect map[string]int) []ItemCount {
	if len(cost) == 0 {
		return nil
	}
	out := make([]ItemCount, 0, len(cost))
	for _, c := range cost {
		if c.Item == "" || c.Count <= 0 {
			continue
		}
		n := c.Count
		if k := alreadyCorrect[c.Item]; k > 0 {
			if k >= n {
				n = 0
			} else {
				n -= k
			}
		}
		if n > 0 {
			out = append(out, ItemCount{Item: c.Item, Count: n})
		}
	}
	return out
}

func FullySatisfied(correct, total int) bool {
	return total > 0 && correct == total
}
