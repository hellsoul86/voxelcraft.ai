package world

import (
	"math"
	"sort"
)

func (w *World) digestAgents(h hashWriter, tmp *[8]byte) {
	agents := w.sortedAgents()
	for _, a := range agents {
		h.Write([]byte(a.ID))
		h.Write([]byte(a.Name))
		h.Write([]byte(a.OrgID))
		digestWriteI64(h, tmp, int64(a.Pos.X))
		digestWriteI64(h, tmp, int64(a.Pos.Y))
		digestWriteI64(h, tmp, int64(a.Pos.Z))
		digestWriteI64(h, tmp, int64(a.Yaw))
		digestWriteU64(h, tmp, uint64(a.HP))
		digestWriteU64(h, tmp, uint64(a.Hunger))
		digestWriteU64(h, tmp, uint64(a.StaminaMilli))
		digestWriteU64(h, tmp, uint64(a.RepTrade))
		digestWriteU64(h, tmp, uint64(a.RepBuild))
		digestWriteU64(h, tmp, uint64(a.RepSocial))
		digestWriteU64(h, tmp, uint64(a.RepLaw))
		digestWriteU64(h, tmp, uint64(a.Fun.Novelty))
		digestWriteU64(h, tmp, uint64(a.Fun.Creation))
		digestWriteU64(h, tmp, uint64(a.Fun.Social))
		digestWriteU64(h, tmp, uint64(a.Fun.Influence))
		digestWriteU64(h, tmp, uint64(a.Fun.Narrative))
		digestWriteU64(h, tmp, uint64(a.Fun.RiskRescue))

		// Fun novelty memory (seen biome/recipes/events) and anti-exploit state.
		biomes := make([]string, 0, len(a.seenBiomes))
		for b, ok := range a.seenBiomes {
			if ok {
				biomes = append(biomes, b)
			}
		}
		sort.Strings(biomes)
		digestWriteU64(h, tmp, uint64(len(biomes)))
		for _, b := range biomes {
			h.Write([]byte(b))
		}
		recipes := make([]string, 0, len(a.seenRecipes))
		for r, ok := range a.seenRecipes {
			if ok {
				recipes = append(recipes, r)
			}
		}
		sort.Strings(recipes)
		digestWriteU64(h, tmp, uint64(len(recipes)))
		for _, r := range recipes {
			h.Write([]byte(r))
		}
		events := make([]string, 0, len(a.seenEvents))
		for e, ok := range a.seenEvents {
			if ok {
				events = append(events, e)
			}
		}
		sort.Strings(events)
		digestWriteU64(h, tmp, uint64(len(events)))
		for _, e := range events {
			h.Write([]byte(e))
		}
		decayKeys := make([]string, 0, len(a.funDecay))
		for k, dw := range a.funDecay {
			if dw != nil {
				decayKeys = append(decayKeys, k)
			}
		}
		sort.Strings(decayKeys)
		digestWriteU64(h, tmp, uint64(len(decayKeys)))
		for _, k := range decayKeys {
			dw := a.funDecay[k]
			h.Write([]byte(k))
			digestWriteU64(h, tmp, dw.StartTick)
			digestWriteU64(h, tmp, uint64(dw.Count))
		}
		h.Write([]byte(a.Equipment.MainHand))
		for i := 0; i < 4; i++ {
			h.Write([]byte(a.Equipment.Armor[i]))
		}

		// Tasks (affects future simulation state; include in digest).
		h.Write([]byte{boolByte(a.MoveTask != nil)})
		if a.MoveTask != nil {
			mt := a.MoveTask
			h.Write([]byte(mt.TaskID))
			h.Write([]byte(string(mt.Kind)))
			digestWriteI64(h, tmp, int64(mt.Target.X))
			digestWriteI64(h, tmp, int64(mt.Target.Y))
			digestWriteI64(h, tmp, int64(mt.Target.Z))
			digestWriteU64(h, tmp, math.Float64bits(mt.Tolerance))
			h.Write([]byte(mt.TargetID))
			digestWriteU64(h, tmp, math.Float64bits(mt.Distance))
			digestWriteI64(h, tmp, int64(mt.StartPos.X))
			digestWriteI64(h, tmp, int64(mt.StartPos.Y))
			digestWriteI64(h, tmp, int64(mt.StartPos.Z))
			digestWriteU64(h, tmp, mt.StartedTick)
		}
		h.Write([]byte{boolByte(a.WorkTask != nil)})
		if a.WorkTask != nil {
			wt := a.WorkTask
			h.Write([]byte(wt.TaskID))
			h.Write([]byte(string(wt.Kind)))
			digestWriteI64(h, tmp, int64(wt.BlockPos.X))
			digestWriteI64(h, tmp, int64(wt.BlockPos.Y))
			digestWriteI64(h, tmp, int64(wt.BlockPos.Z))
			h.Write([]byte(wt.RecipeID))
			h.Write([]byte(wt.ItemID))
			digestWriteU64(h, tmp, uint64(wt.Count))
			h.Write([]byte(wt.BlueprintID))
			digestWriteI64(h, tmp, int64(wt.Anchor.X))
			digestWriteI64(h, tmp, int64(wt.Anchor.Y))
			digestWriteI64(h, tmp, int64(wt.Anchor.Z))
			digestWriteU64(h, tmp, uint64(wt.Rotation))
			digestWriteU64(h, tmp, uint64(wt.BuildIndex))
			h.Write([]byte(wt.TargetID))
			h.Write([]byte(wt.SrcContainer))
			h.Write([]byte(wt.DstContainer))
			digestWriteU64(h, tmp, wt.StartedTick)
			digestWriteU64(h, tmp, uint64(wt.WorkTicks))
		}

		// Inventory (sorted).
		inv := a.InventoryList()
		for _, it := range inv {
			h.Write([]byte(it.Item))
			digestWriteU64(h, tmp, uint64(it.Count))
		}
	}
}
