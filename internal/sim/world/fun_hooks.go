package world

import (
	"math"

	"voxelcraft.ai/internal/protocol"
)

func (w *World) funOnBiome(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	b := biomeAt(w.cfg.Seed, a.Pos.X, a.Pos.Z, w.cfg.BiomeRegionSize)
	if b == "" {
		return
	}
	if a.seenBiomes[b] {
		return
	}
	a.seenBiomes[b] = true
	w.addFun(a, nowTick, "NOVELTY", "biome:"+b, 10)
}

func (w *World) funOnRecipe(a *Agent, recipeID string, tier int, nowTick uint64) {
	if a == nil || recipeID == "" {
		return
	}
	if a.seenRecipes[recipeID] {
		return
	}
	a.seenRecipes[recipeID] = true
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
	if a.seenEvents[eventID] {
		return
	}
	a.seenEvents[eventID] = true
	w.addFun(a, nowTick, "NOVELTY", "event:"+eventID, 5)
}

func (w *World) funOnTrade(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	base := 2
	// Low reputation makes social fun less valuable (anti-scam signal).
	factor := socialFunFactor(a)
	base = int(math.Round(float64(base) * factor))
	if base <= 0 {
		return
	}
	w.addFun(a, nowTick, "SOCIAL", "trade", w.funDecay(a, "social:trade", base, nowTick))
}

func (w *World) funOnContractComplete(a *Agent, nowTick uint64, kind string) {
	if a == nil {
		return
	}
	base := 5
	if kind == "BUILD" {
		base = 7
	}
	factor := socialFunFactor(a)
	base = int(math.Round(float64(base) * factor))
	if base <= 0 {
		return
	}
	w.addFun(a, nowTick, "SOCIAL", "contract", w.funDecay(a, "social:contract", base, nowTick))
	if w.activeEventID != "" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_success", w.funDecay(a, "narrative:event_success", 5, nowTick))
	}
	if w.weather == "STORM" || w.weather == "COLD" {
		w.addFun(a, nowTick, "RISK_RESCUE", "hazard_success", w.funDecay(a, "risk:hazard_success", 8, nowTick))
	}
}

func (w *World) funOnBlueprintComplete(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	if w.activeEventID != "" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_build", w.funDecay(a, "narrative:event_build", 5, nowTick))
	}
	if w.weather == "STORM" || w.weather == "COLD" {
		w.addFun(a, nowTick, "RISK_RESCUE", "hazard_build", w.funDecay(a, "risk:hazard_build", 8, nowTick))
	}
}

func (w *World) funOnLawActive(proposer *Agent, nowTick uint64) {
	if proposer == nil {
		return
	}
	w.addFun(proposer, nowTick, "INFLUENCE", "law_adopted", w.funDecay(proposer, "influence:law_adopted", 4, nowTick))
	w.addFun(proposer, nowTick, "NARRATIVE", "law_adopted", w.funDecay(proposer, "narrative:law_adopted", 5, nowTick))
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(proposer, w.activeEventID, nowTick)
		w.addFun(proposer, nowTick, "NARRATIVE", "civic_vote_law", w.funDecay(proposer, "narrative:civic_vote_law", 6, nowTick))
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
	w.addFun(a, nowTick, "NARRATIVE", reason, w.funDecay(a, key, base, nowTick))
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
