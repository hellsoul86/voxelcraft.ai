package economy

import "testing"

func TestEffectiveMarketTax(t *testing.T) {
	if got := EffectiveMarketTax(0.05, false, "", 0, 0); got != 0 {
		t.Fatalf("expected 0 when not same land, got %f", got)
	}
	if got := EffectiveMarketTax(0.05, true, "", 0, 0); got != 0.05 {
		t.Fatalf("expected 0.05, got %f", got)
	}
	if got := EffectiveMarketTax(0.05, true, "MARKET_WEEK", 10, 100); got != 0.025 {
		t.Fatalf("expected 0.025, got %f", got)
	}
}
