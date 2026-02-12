package value

import "testing"

func TestTradeValueDeterministicAndPositive(t *testing.T) {
	itemsA := map[string]int{"PLANK": 5, "IRON_INGOT": 1}
	itemsB := map[string]int{"IRON_INGOT": 1, "PLANK": 5}
	va := TradeValue(itemsA, ItemTradeValue)
	vb := TradeValue(itemsB, ItemTradeValue)
	if va <= 0 || vb <= 0 {
		t.Fatalf("expected positive values: va=%d vb=%d", va, vb)
	}
	if va != vb {
		t.Fatalf("expected deterministic map-order invariant value: va=%d vb=%d", va, vb)
	}
}

func TestTradeMutualBenefit(t *testing.T) {
	fairOffer := TradeValue(map[string]int{"PLANK": 5}, ItemTradeValue)
	fairRequest := TradeValue(map[string]int{"IRON_INGOT": 1}, ItemTradeValue)
	if !TradeMutualBenefit(fairOffer, fairRequest) {
		t.Fatalf("expected fair trade to be mutual benefit")
	}

	unfairOffer := TradeValue(map[string]int{"PLANK": 1}, ItemTradeValue)
	unfairRequest := TradeValue(map[string]int{"CRYSTAL_SHARD": 1}, ItemTradeValue)
	if TradeMutualBenefit(unfairOffer, unfairRequest) {
		t.Fatalf("expected unfair trade to be rejected")
	}
}
