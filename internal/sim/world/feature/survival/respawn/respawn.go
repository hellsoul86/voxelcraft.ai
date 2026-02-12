package respawn

import "sort"

func ComputeInventoryLoss(inventory map[string]int) map[string]int {
	lost := map[string]int{}
	if len(inventory) == 0 {
		return lost
	}

	keys := make([]string, 0, len(inventory))
	for k, n := range inventory {
		if k != "" && n > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, k := range keys {
		n := inventory[k]
		d := (n * 3) / 10
		if d <= 0 {
			continue
		}
		inventory[k] -= d
		if inventory[k] <= 0 {
			delete(inventory, k)
		}
		lost[k] = d
	}

	// Ensure at least one item is lost when inventory is non-empty.
	if len(lost) == 0 {
		for _, k := range keys {
			if inventory[k] > 0 {
				inventory[k]--
				if inventory[k] <= 0 {
					delete(inventory, k)
				}
				lost[k] = 1
				break
			}
		}
	}

	return lost
}

func AgentNumber(agentID string) int {
	if len(agentID) < 2 || agentID[0] != 'A' {
		return 0
	}
	n := 0
	for i := 1; i < len(agentID); i++ {
		c := agentID[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
