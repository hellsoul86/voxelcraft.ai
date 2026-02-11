package world

import (
	"math"
	"sort"
)

func (w *World) funInit() {
	if w.structures == nil {
		w.structures = map[string]*Structure{}
	}
}

func (w *World) systemFun(nowTick uint64) {
	w.funInit()

	// Award delayed creation scores.
	if len(w.structures) > 0 {
		ids := make([]string, 0, len(w.structures))
		for id := range w.structures {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			s := w.structures[id]
			if s == nil || s.Awarded || nowTick < s.AwardDueTick {
				continue
			}

			// Validate that the structure still exists and matches the blueprint.
			if !w.checkBlueprintPlaced(s.BlueprintID, s.Anchor, s.Rotation) {
				delete(w.structures, id)
				continue
			}

			builder := w.agents[s.BuilderID]
			if builder == nil {
				delete(w.structures, id)
				continue
			}

			bp, ok := w.catalogs.Blueprints.ByID[s.BlueprintID]
			if !ok {
				delete(w.structures, id)
				continue
			}

			creationPts := w.structureCreationScore(&bp, s, nowTick)
			if creationPts > 0 {
				w.addFun(builder, nowTick, "CREATION", "structure", w.funDecay(builder, "creation:structure", creationPts, nowTick))
			}

			s.Awarded = true
		}
	}

	// Influence: award per in-game day boundary.
	if w.cfg.DayTicks > 0 && nowTick != 0 && nowTick%uint64(w.cfg.DayTicks) == 0 {
		day := int(nowTick / uint64(w.cfg.DayTicks))
		ids := make([]string, 0, len(w.structures))
		for id := range w.structures {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			s := w.structures[id]
			if s == nil {
				continue
			}
			if !w.checkBlueprintPlaced(s.BlueprintID, s.Anchor, s.Rotation) {
				delete(w.structures, id)
				continue
			}
			if s.LastInfluenceDay == day {
				continue
			}
			s.LastInfluenceDay = day
			builder := w.agents[s.BuilderID]
			if builder == nil {
				continue
			}
			users := w.structureUniqueUsers(s, nowTick, uint64(w.cfg.DayTicks))
			if users <= 0 {
				continue
			}
			pts := int(math.Round(minFloat(15, 3*math.Sqrt(float64(users)))))
			if pts > 0 {
				w.addFun(builder, nowTick, "INFLUENCE", "infra_usage_day", w.funDecay(builder, "influence:infra_usage_day", pts, nowTick))
			}
		}
	}
}
