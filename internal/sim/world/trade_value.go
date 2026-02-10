package world

import "sort"

const tradeFairMinPct = 50 // mutual benefit if values are within 2x of each other

func (w *World) itemTradeValue(item string) int64 {
	// Deterministic, coarse valuation table used only for anti-exploit gating of fun/reputation.
	// It intentionally does NOT affect trade execution.
	switch item {
	case "PLANK":
		return 1
	case "LOG":
		return 2
	case "STONE", "GRAVEL", "DIRT", "SAND", "BRICK", "GLASS", "METAL_PLATE":
		return 1
	case "STICK":
		return 1
	case "COAL":
		return 2
	case "IRON_ORE", "COPPER_ORE":
		return 3
	case "IRON_INGOT":
		return 5
	case "COPPER_INGOT":
		return 4
	case "CRYSTAL_SHARD":
		return 20
	case "WIRE":
		return 1
	case "BATTERY":
		return 30
	case "CONVEYOR":
		return 36
	case "SENSOR":
		return 52
	case "CLAIM_TOTEM":
		return 26
	case "CONTRACT_TERMINAL":
		return 80

	case "BERRIES":
		return 1
	case "BREAD":
		return 3
	case "RAW_MEAT":
		return 1
	case "COOKED_MEAT":
		return 5

	default:
		// Unknown or unpriced: treat as low value so extreme trades don't farm social rewards.
		return 1
	}
}

func (w *World) tradeValue(items map[string]int) int64 {
	if w == nil || len(items) == 0 {
		return 0
	}
	keys := make([]string, 0, len(items))
	for k, n := range items {
		if k != "" && n > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var total int64
	for _, k := range keys {
		n := items[k]
		if n <= 0 {
			continue
		}
		total += w.itemTradeValue(k) * int64(n)
	}
	return total
}

func (w *World) tradeMutualBenefit(offer, request map[string]int) (ok bool, offerValue, requestValue int64) {
	offerValue = w.tradeValue(offer)
	requestValue = w.tradeValue(request)
	if offerValue <= 0 || requestValue <= 0 {
		return false, offerValue, requestValue
	}
	minv := offerValue
	maxv := requestValue
	if minv > maxv {
		minv, maxv = maxv, minv
	}
	// min/max >= tradeFairMinPct%
	if minv*100 < maxv*tradeFairMinPct {
		return false, offerValue, requestValue
	}
	return true, offerValue, requestValue
}
