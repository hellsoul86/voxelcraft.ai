package value

import "sort"

const TradeFairMinPct = 50

func ItemTradeValue(item string) int64 {
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
		return 1
	}
}

func TradeValue(items map[string]int, itemValue func(string) int64) int64 {
	if len(items) == 0 || itemValue == nil {
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
		total += itemValue(k) * int64(n)
	}
	return total
}

func TradeMutualBenefit(offerValue, requestValue int64) bool {
	if offerValue <= 0 || requestValue <= 0 {
		return false
	}
	minv := offerValue
	maxv := requestValue
	if minv > maxv {
		minv, maxv = maxv, minv
	}
	return minv*100 >= maxv*TradeFairMinPct
}
