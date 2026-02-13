package runtime

import (
	"testing"

	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestApplyTemplateToLandMarketTax(t *testing.T) {
	law := &lawspkg.Law{
		TemplateID: "MARKET_TAX",
		Params:     map[string]string{"market_tax": "0.12"},
	}
	land := &modelpkg.LandClaim{}
	if err := ApplyTemplateToLand(law, land); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if land.MarketTax != 0.12 {
		t.Fatalf("expected market tax 0.12, got %v", land.MarketTax)
	}
}

func TestApplyTemplateToLandNilGuards(t *testing.T) {
	if err := ApplyTemplateToLand(nil, &modelpkg.LandClaim{}); err == nil {
		t.Fatalf("expected nil law error")
	}
	if err := ApplyTemplateToLand(&lawspkg.Law{}, nil); err == nil {
		t.Fatalf("expected nil land error")
	}
}
