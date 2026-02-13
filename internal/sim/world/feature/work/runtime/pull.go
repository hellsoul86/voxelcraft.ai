package runtime

import (
	"fmt"
	"sort"

	"voxelcraft.ai/internal/sim/catalogs"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type StorageCandidate struct {
	Pos  modelpkg.Vec3i
	Type string
	Dist int
	Ref  *modelpkg.Container
}

type StorageCandidateInput struct {
	Anchor              modelpkg.Vec3i
	AnchorLandID        string
	AgentID             string
	Containers          map[modelpkg.Vec3i]*modelpkg.Container
	AutoPullRange       int
	LandIDAt            func(pos modelpkg.Vec3i) (landID string, hasLand bool)
	CanWithdraw         func(agentID string, pos modelpkg.Vec3i) bool
	Manhattan           func(a, b modelpkg.Vec3i) int
	SupportedContainers map[string]struct{}
}

func BuildStorageCandidates(in StorageCandidateInput) []StorageCandidate {
	if in.LandIDAt == nil || in.CanWithdraw == nil || in.Manhattan == nil {
		return nil
	}
	supported := in.SupportedContainers
	if len(supported) == 0 {
		supported = map[string]struct{}{"CHEST": {}, "CONTRACT_TERMINAL": {}}
	}
	out := make([]StorageCandidate, 0, 8)
	for pos, c := range in.Containers {
		if c == nil {
			continue
		}
		if _, ok := supported[c.Type]; !ok {
			continue
		}
		d := in.Manhattan(pos, in.Anchor)
		if in.AutoPullRange > 0 && d > in.AutoPullRange {
			continue
		}

		landID, hasLand := in.LandIDAt(pos)
		if in.AnchorLandID == "" {
			if hasLand {
				continue
			}
		} else {
			if !hasLand || landID != in.AnchorLandID {
				continue
			}
		}

		if !in.CanWithdraw(in.AgentID, pos) {
			continue
		}
		out = append(out, StorageCandidate{Pos: pos, Type: c.Type, Dist: d, Ref: c})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Dist != out[j].Dist {
			return out[i].Dist < out[j].Dist
		}
		if out[i].Pos.X != out[j].Pos.X {
			return out[i].Pos.X < out[j].Pos.X
		}
		if out[i].Pos.Y != out[j].Pos.Y {
			return out[i].Pos.Y < out[j].Pos.Y
		}
		if out[i].Pos.Z != out[j].Pos.Z {
			return out[i].Pos.Z < out[j].Pos.Z
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func EnsureBlueprintMaterials(inv map[string]int, cands []StorageCandidate, cost []catalogs.ItemCount) (bool, string) {
	if len(cost) == 0 {
		return true, ""
	}
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
		have := inv[item]
		if have >= required {
			continue
		}
		deficit := required - have
		avail := 0
		for _, cand := range cands {
			if cand.Ref == nil {
				continue
			}
			avail += cand.Ref.AvailableCount(item)
			if avail >= deficit {
				break
			}
		}
		if avail < deficit {
			return false, fmt.Sprintf("missing %s x%d", item, deficit-avail)
		}
	}

	for _, item := range items {
		required := need[item]
		for inv[item] < required {
			deficit := required - inv[item]
			tookAny := false
			for _, cand := range cands {
				if cand.Ref == nil {
					continue
				}
				avail := cand.Ref.AvailableCount(item)
				if avail <= 0 {
					continue
				}
				take := avail
				if take > deficit {
					take = deficit
				}
				cand.Ref.Inventory[item] -= take
				if cand.Ref.Inventory[item] <= 0 {
					delete(cand.Ref.Inventory, item)
				}
				inv[item] += take
				deficit -= take
				tookAny = true
				if deficit <= 0 {
					break
				}
			}
			if !tookAny {
				return false, "missing materials"
			}
		}
	}
	return true, ""
}
