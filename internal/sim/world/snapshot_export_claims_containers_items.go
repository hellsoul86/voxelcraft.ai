package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) exportSnapshotClaims() []snapshot.ClaimV1 {
	claimIDs := make([]string, 0, len(w.claims))
	for id := range w.claims {
		claimIDs = append(claimIDs, id)
	}
	sort.Strings(claimIDs)
	claimSnaps := make([]snapshot.ClaimV1, 0, len(claimIDs))
	for _, id := range claimIDs {
		c := w.claims[id]
		members := []string{}
		for mid, ok := range c.Members {
			if ok {
				members = append(members, mid)
			}
		}
		sort.Strings(members)
		claimSnaps = append(claimSnaps, snapshot.ClaimV1{
			LandID:    c.LandID,
			Owner:     c.Owner,
			ClaimType: c.ClaimType,
			Anchor:    c.Anchor.ToArray(),
			Radius:    c.Radius,
			Flags: snapshot.ClaimFlagsV1{
				AllowBuild:  c.Flags.AllowBuild,
				AllowBreak:  c.Flags.AllowBreak,
				AllowDamage: c.Flags.AllowDamage,
				AllowTrade:  c.Flags.AllowTrade,
			},
			Members:            members,
			MarketTax:          c.MarketTax,
			CurfewEnabled:      c.CurfewEnabled,
			CurfewStart:        c.CurfewStart,
			CurfewEnd:          c.CurfewEnd,
			FineBreakEnabled:   c.FineBreakEnabled,
			FineBreakItem:      c.FineBreakItem,
			FineBreakPerBlock:  c.FineBreakPerBlock,
			AccessPassEnabled:  c.AccessPassEnabled,
			AccessTicketItem:   c.AccessTicketItem,
			AccessTicketCost:   c.AccessTicketCost,
			MaintenanceDueTick: c.MaintenanceDueTick,
			MaintenanceStage:   c.MaintenanceStage,
		})
	}
	return claimSnaps
}

func (w *World) exportSnapshotContainers() []snapshot.ContainerV1 {
	contPos := make([]Vec3i, 0, len(w.containers))
	for p := range w.containers {
		contPos = append(contPos, p)
	}
	sort.Slice(contPos, func(i, j int) bool {
		if contPos[i].X != contPos[j].X {
			return contPos[i].X < contPos[j].X
		}
		if contPos[i].Y != contPos[j].Y {
			return contPos[i].Y < contPos[j].Y
		}
		return contPos[i].Z < contPos[j].Z
	})
	containerSnaps := make([]snapshot.ContainerV1, 0, len(contPos))
	for _, p := range contPos {
		c := w.containers[p]
		inv := map[string]int{}
		for k, v := range c.Inventory {
			if v != 0 {
				inv[k] = v
			}
		}
		res := map[string]int{}
		for k, v := range c.Reserved {
			if v != 0 {
				res[k] = v
			}
		}
		owed := map[string]map[string]int{}
		for aid, m := range c.Owed {
			m2 := map[string]int{}
			for k, v := range m {
				if v != 0 {
					m2[k] = v
				}
			}
			if len(m2) > 0 {
				owed[aid] = m2
			}
		}
		containerSnaps = append(containerSnaps, snapshot.ContainerV1{
			Type:      c.Type,
			Pos:       c.Pos.ToArray(),
			Inventory: inv,
			Reserved:  res,
			Owed:      owed,
		})
	}
	return containerSnaps
}

func (w *World) exportSnapshotItems(nowTick uint64) []snapshot.ItemEntityV1 {
	itemIDs := make([]string, 0, len(w.items))
	for id, e := range w.items {
		if e == nil || e.Item == "" || e.Count <= 0 {
			continue
		}
		if e.ExpiresTick != 0 && nowTick >= e.ExpiresTick {
			continue
		}
		itemIDs = append(itemIDs, id)
	}
	sort.Strings(itemIDs)
	itemSnaps := make([]snapshot.ItemEntityV1, 0, len(itemIDs))
	for _, id := range itemIDs {
		e := w.items[id]
		if e == nil || e.Item == "" || e.Count <= 0 {
			continue
		}
		if e.ExpiresTick != 0 && nowTick >= e.ExpiresTick {
			continue
		}
		itemSnaps = append(itemSnaps, snapshot.ItemEntityV1{
			EntityID:    e.EntityID,
			Pos:         e.Pos.ToArray(),
			Item:        e.Item,
			Count:       e.Count,
			CreatedTick: e.CreatedTick,
			ExpiresTick: e.ExpiresTick,
		})
	}
	return itemSnaps
}
