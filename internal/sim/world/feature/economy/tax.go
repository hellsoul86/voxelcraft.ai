package economy

func EffectiveMarketTax(baseRate float64, sameLand bool, activeEventID string, nowTick, activeEventEnds uint64) float64 {
	if !sameLand || baseRate <= 0 {
		return 0
	}
	taxRate := baseRate
	if activeEventID == "MARKET_WEEK" && nowTick < activeEventEnds {
		taxRate *= 0.5
	}
	if taxRate < 0 {
		return 0
	}
	if taxRate > 1 {
		return 1
	}
	return taxRate
}
