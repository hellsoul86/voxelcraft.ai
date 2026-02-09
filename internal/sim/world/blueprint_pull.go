package world

import (
	"fmt"
	"sort"

	"voxelcraft.ai/internal/sim/catalogs"
)

const blueprintAutoPullRange = 32

type containerCand struct {
	pos  Vec3i
	typ  string
	dist int
	c    *Container
}

func (w *World) blueprintStorageCandidates(agentID string, anchor Vec3i) []containerCand {
	var anchorLandID string
	if land := w.landAt(anchor); land != nil {
		anchorLandID = land.LandID
	}

	cands := make([]containerCand, 0, 8)
	for pos, c := range w.containers {
		if c == nil {
			continue
		}
		switch c.Type {
		case "CHEST", "CONTRACT_TERMINAL":
		default:
			continue
		}
		d := Manhattan(pos, anchor)
		if d > blueprintAutoPullRange {
			continue
		}

		land := w.landAt(pos)
		if anchorLandID == "" {
			if land != nil {
				continue
			}
		} else {
			if land == nil || land.LandID != anchorLandID {
				continue
			}
		}

		if !w.canWithdrawFromContainer(agentID, pos) {
			continue
		}
		cands = append(cands, containerCand{pos: pos, typ: c.Type, dist: d, c: c})
	}

	sort.Slice(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		if cands[i].pos.X != cands[j].pos.X {
			return cands[i].pos.X < cands[j].pos.X
		}
		if cands[i].pos.Y != cands[j].pos.Y {
			return cands[i].pos.Y < cands[j].pos.Y
		}
		if cands[i].pos.Z != cands[j].pos.Z {
			return cands[i].pos.Z < cands[j].pos.Z
		}
		return cands[i].typ < cands[j].typ
	})
	return cands
}

func (w *World) blueprintEnsureMaterials(a *Agent, anchor Vec3i, cost []catalogs.ItemCount, nowTick uint64) (ok bool, errMsg string) {
	if a == nil || len(cost) == 0 {
		return true, ""
	}

	cands := w.blueprintStorageCandidates(a.ID, anchor)

	// Preflight: ensure availability without mutating state.
	need := map[string]int{}
	for _, it := range cost {
		if it.Item == "" || it.Count <= 0 {
			continue
		}
		need[it.Item] += it.Count
	}
	items := make([]string, 0, len(need))
	for item := range need {
		items = append(items, item)
	}
	sort.Strings(items)

	for _, item := range items {
		required := need[item]
		have := a.Inventory[item]
		if have >= required {
			continue
		}
		deficit := required - have
		avail := 0
		for _, cand := range cands {
			avail += cand.c.availableCount(item)
			if avail >= deficit {
				break
			}
		}
		if avail < deficit {
			return false, fmt.Sprintf("missing %s x%d", item, deficit-avail)
		}
	}

	// Pull deficits deterministically.
	for _, item := range items {
		required := need[item]
		for a.Inventory[item] < required {
			deficit := required - a.Inventory[item]
			tookAny := false
			for _, cand := range cands {
				avail := cand.c.availableCount(item)
				if avail <= 0 {
					continue
				}
				take := avail
				if take > deficit {
					take = deficit
				}
				cand.c.Inventory[item] -= take
				if cand.c.Inventory[item] <= 0 {
					delete(cand.c.Inventory, item)
				}
				a.Inventory[item] += take
				deficit -= take
				tookAny = true
				if deficit <= 0 {
					break
				}
			}
			if !tookAny {
				// Should not happen due to preflight.
				return false, "missing materials"
			}
		}
	}
	return true, ""
}
