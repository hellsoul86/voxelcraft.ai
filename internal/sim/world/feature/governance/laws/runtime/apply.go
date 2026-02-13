package runtime

import (
	"fmt"

	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func ApplyTemplateToLand(law *lawspkg.Law, land *modelpkg.LandClaim) error {
	if law == nil {
		return fmt.Errorf("nil law")
	}
	if land == nil {
		return fmt.Errorf("land not found")
	}
	out, err := lawspkg.ApplyLawTemplate(law.TemplateID, law.Params, lawspkg.LandState{
		MarketTax:         land.MarketTax,
		CurfewEnabled:     land.CurfewEnabled,
		CurfewStart:       land.CurfewStart,
		CurfewEnd:         land.CurfewEnd,
		FineBreakEnabled:  land.FineBreakEnabled,
		FineBreakItem:     land.FineBreakItem,
		FineBreakPerBlock: land.FineBreakPerBlock,
		AccessPassEnabled: land.AccessPassEnabled,
		AccessTicketItem:  land.AccessTicketItem,
		AccessTicketCost:  land.AccessTicketCost,
	})
	if err != nil {
		return err
	}
	land.MarketTax = out.MarketTax
	land.CurfewEnabled = out.CurfewEnabled
	land.CurfewStart = out.CurfewStart
	land.CurfewEnd = out.CurfewEnd
	land.FineBreakEnabled = out.FineBreakEnabled
	land.FineBreakItem = out.FineBreakItem
	land.FineBreakPerBlock = out.FineBreakPerBlock
	land.AccessPassEnabled = out.AccessPassEnabled
	land.AccessTicketItem = out.AccessTicketItem
	land.AccessTicketCost = out.AccessTicketCost
	return nil
}
