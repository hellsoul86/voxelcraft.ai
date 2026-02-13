package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	statspkg "voxelcraft.ai/internal/sim/world/feature/director/stats"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
	genpkg "voxelcraft.ai/internal/sim/world/terrain/gen"
)

func (w *World) funOnBiome(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	b := genpkg.BiomeAt(w.cfg.Seed, a.Pos.X, a.Pos.Z, w.cfg.BiomeRegionSize)
	if b == "" {
		return
	}
	if !a.MarkBiomeSeen(b) {
		return
	}
	w.addFun(a, nowTick, "NOVELTY", "biome:"+b, 10)
}

func (w *World) funOnRecipe(a *Agent, recipeID string, tier int, nowTick uint64) {
	if a == nil || recipeID == "" {
		return
	}
	if !a.MarkRecipeSeen(recipeID) {
		return
	}
	pts := 3
	switch tier {
	case 0, 1:
		pts = 3
	case 2:
		pts = 5
	default:
		pts = 8
	}
	w.addFun(a, nowTick, "NOVELTY", "recipe:"+recipeID, pts)
}

func (w *World) funOnWorldEventParticipation(a *Agent, eventID string, nowTick uint64) {
	if a == nil || eventID == "" {
		return
	}
	if !a.MarkEventSeen(eventID) {
		return
	}
	w.addFun(a, nowTick, "NOVELTY", "event:"+eventID, 5)
}

func (w *World) funOnTrade(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	base := 2
	// Low reputation makes social fun less valuable (anti-scam signal).
	base = statspkg.ScaleByFactor(base, statspkg.SocialFunFactor(a.RepTrade))
	if base <= 0 {
		return
	}
	w.addFun(a, nowTick, "SOCIAL", "trade", a.FunDecayDelta("social:trade", base, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
}

func (w *World) funOnContractComplete(a *Agent, nowTick uint64, kind string) {
	if a == nil {
		return
	}
	base := 5
	if kind == "BUILD" {
		base = 7
	}
	base = statspkg.ScaleByFactor(base, statspkg.SocialFunFactor(a.RepTrade))
	if base <= 0 {
		return
	}
	w.addFun(a, nowTick, "SOCIAL", "contract", a.FunDecayDelta("social:contract", base, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	if w.activeEventID != "" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_success", a.FunDecayDelta("narrative:event_success", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
	if w.weather == "STORM" || w.weather == "COLD" {
		w.addFun(a, nowTick, "RISK_RESCUE", "hazard_success", a.FunDecayDelta("risk:hazard_success", 8, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
}

func (w *World) funOnBlueprintComplete(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	if w.activeEventID != "" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_build", a.FunDecayDelta("narrative:event_build", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
	if w.weather == "STORM" || w.weather == "COLD" {
		w.addFun(a, nowTick, "RISK_RESCUE", "hazard_build", a.FunDecayDelta("risk:hazard_build", 8, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
}

func (w *World) funOnLawActive(proposer *Agent, nowTick uint64) {
	if proposer == nil {
		return
	}
	w.addFun(proposer, nowTick, "INFLUENCE", "law_adopted", proposer.FunDecayDelta("influence:law_adopted", 4, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	w.addFun(proposer, nowTick, "NARRATIVE", "law_adopted", proposer.FunDecayDelta("narrative:law_adopted", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(proposer, w.activeEventID, nowTick)
		w.addFun(proposer, nowTick, "NARRATIVE", "civic_vote_law", proposer.FunDecayDelta("narrative:civic_vote_law", 6, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
}

func (w *World) funOnVote(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	base := 2
	key := "narrative:vote"
	reason := "vote"
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		base = 4
		key = "narrative:civic_vote_vote"
		reason = "civic_vote_vote"
	}
	w.addFun(a, nowTick, "NARRATIVE", reason, a.FunDecayDelta(key, base, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
}

func (w *World) addFun(a *Agent, nowTick uint64, dim string, reason string, delta int) {
	if a == nil || delta == 0 {
		return
	}
	total := 0
	switch dim {
	case "NOVELTY":
		a.Fun.Novelty += delta
		total = a.Fun.Novelty
	case "CREATION":
		a.Fun.Creation += delta
		total = a.Fun.Creation
	case "SOCIAL":
		a.Fun.Social += delta
		total = a.Fun.Social
	case "INFLUENCE":
		a.Fun.Influence += delta
		total = a.Fun.Influence
	case "NARRATIVE":
		a.Fun.Narrative += delta
		total = a.Fun.Narrative
	case "RISK_RESCUE":
		a.Fun.RiskRescue += delta
		total = a.Fun.RiskRescue
	default:
		return
	}

	a.AddEvent(protocol.Event{
		"t":      nowTick,
		"type":   "FUN",
		"dim":    dim,
		"delta":  delta,
		"total":  total,
		"reason": reason,
	})
}

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
				w.addFun(builder, nowTick, "CREATION", "structure", builder.FunDecayDelta("creation:structure", creationPts, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
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
			pts := statspkg.InfluenceUsagePoints(users)
			if pts > 0 {
				w.addFun(builder, nowTick, "INFLUENCE", "infra_usage_day", builder.FunDecayDelta("influence:infra_usage_day", pts, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
			}
		}
	}
}

func (w *World) registerStructure(nowTick uint64, builderID string, blueprintID string, anchor Vec3i, rotation int) {
	w.funInit()
	bp, ok := w.catalogs.Blueprints.ByID[blueprintID]
	if !ok {
		return
	}
	rot := blueprint.NormalizeRotation(rotation)

	id := statspkg.StructureID(builderID, nowTick, blueprintID, anchor.X, anchor.Y, anchor.Z)

	// Compute actual bounds from rotated block positions (rotation affects Min/Max).
	min := Vec3i{X: anchor.X, Y: anchor.Y, Z: anchor.Z}
	max := Vec3i{X: anchor.X, Y: anchor.Y, Z: anchor.Z}
	if len(bp.Blocks) > 0 {
		first := true
		for _, b := range bp.Blocks {
			off := blueprint.RotateOffset(b.Pos, rot)
			p := Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
			if first {
				min, max = p, p
				first = false
				continue
			}
			if p.X < min.X {
				min.X = p.X
			}
			if p.Y < min.Y {
				min.Y = p.Y
			}
			if p.Z < min.Z {
				min.Z = p.Z
			}
			if p.X > max.X {
				max.X = p.X
			}
			if p.Y > max.Y {
				max.Y = p.Y
			}
			if p.Z > max.Z {
				max.Z = p.Z
			}
		}
	}
	w.structures[id] = &Structure{
		StructureID:   id,
		BlueprintID:   blueprintID,
		BuilderID:     builderID,
		Anchor:        anchor,
		Rotation:      rot,
		Min:           min,
		Max:           max,
		CompletedTick: nowTick,
		AwardDueTick:  nowTick + uint64(w.cfg.StructureSurvivalTicks),
		UsedBy:        map[string]uint64{},
	}
}

func (w *World) recordStructureUsage(agentID string, pos Vec3i, nowTick uint64) {
	if len(w.structures) == 0 || agentID == "" {
		return
	}
	for _, s := range w.structures {
		if s == nil {
			continue
		}
		if pos.X < s.Min.X || pos.X > s.Max.X || pos.Y < s.Min.Y || pos.Y > s.Max.Y || pos.Z < s.Min.Z || pos.Z > s.Max.Z {
			continue
		}
		if s.UsedBy == nil {
			s.UsedBy = map[string]uint64{}
		}
		s.UsedBy[agentID] = nowTick
	}
}

func (w *World) structureUniqueUsers(s *Structure, nowTick uint64, window uint64) int {
	if s == nil {
		return 0
	}
	return statspkg.StructureUniqueUsers(s.UsedBy, s.BuilderID, nowTick, window)
}

func (w *World) structureCreationScore(bp *catalogs.BlueprintDef, s *Structure, nowTick uint64) int {
	if bp == nil || s == nil {
		return 0
	}
	features := statspkg.ExtractBlueprintFeatures(bp.Blocks)

	stable := w.structureStable(bp, s.Anchor, s.Rotation)
	users := w.structureUniqueUsers(s, nowTick, uint64(w.cfg.DayTicks))
	return statspkg.CreationScore(statspkg.CreationScoreInput{
		UniqueBlockTypes: features.UniqueBlockTypes,
		HasStorage:       features.HasStorage,
		HasLight:         features.HasLight,
		HasWorkshop:      features.HasWorkshop,
		HasGovernance:    features.HasGovernance,
		Stable:           stable,
		Users:            users,
	})
}

func (w *World) structureStable(bp *catalogs.BlueprintDef, anchor Vec3i, rotation int) bool {
	if bp == nil || len(bp.Blocks) == 0 {
		return true
	}
	rot := blueprint.NormalizeRotation(rotation)
	positions := make([]statspkg.Vec3, 0, len(bp.Blocks))
	for _, b := range bp.Blocks {
		off := blueprint.RotateOffset(b.Pos, rot)
		p := statspkg.Vec3{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
		positions = append(positions, p)
	}
	return statspkg.IsStructureStable(positions, func(x, y, z int) bool {
		return w.chunks.GetBlock(Vec3i{X: x, Y: y, Z: z}) != w.chunks.gen.Air
	})
}
