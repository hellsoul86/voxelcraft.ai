package world

import (
	"math"
	"sort"
	"strconv"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

type FunScore struct {
	Novelty    int
	Creation   int
	Social     int
	Influence  int
	Narrative  int
	RiskRescue int
}

type funDecayWindow struct {
	StartTick uint64
	Count     int
}

type Structure struct {
	StructureID string
	BlueprintID string
	BuilderID   string
	Anchor      Vec3i
	Rotation    int
	Min         Vec3i
	Max         Vec3i

	CompletedTick uint64
	AwardDueTick  uint64
	Awarded       bool

	// Usage: agent_id -> last tick seen inside the structure.
	UsedBy map[string]uint64

	// Influence: last day index we awarded influence for.
	LastInfluenceDay int
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

func (w *World) registerStructure(nowTick uint64, builderID string, blueprintID string, anchor Vec3i, rotation int) {
	w.funInit()
	bp, ok := w.catalogs.Blueprints.ByID[blueprintID]
	if !ok {
		return
	}
	rot := normalizeRotation(rotation)

	id := fmtStructureID(builderID, nowTick, blueprintID, anchor)

	// Compute actual bounds from rotated block positions (rotation affects Min/Max).
	min := Vec3i{X: anchor.X, Y: anchor.Y, Z: anchor.Z}
	max := Vec3i{X: anchor.X, Y: anchor.Y, Z: anchor.Z}
	if len(bp.Blocks) > 0 {
		first := true
		for _, b := range bp.Blocks {
			off := rotateOffset(b.Pos, rot)
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

func fmtStructureID(builderID string, nowTick uint64, blueprintID string, anchor Vec3i) string {
	// Deterministic, stable id (no counters) so snapshots and replays match.
	return "STRUCT_" + builderID + "_" + itoaU64(nowTick) + "_" + blueprintID + "_" + itoaI(anchor.X) + "_" + itoaI(anchor.Y) + "_" + itoaI(anchor.Z)
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
	if s == nil || len(s.UsedBy) == 0 {
		return 0
	}
	cutoff := uint64(0)
	if nowTick > window {
		cutoff = nowTick - window
	}
	n := 0
	for aid, last := range s.UsedBy {
		if aid == "" || aid == s.BuilderID {
			continue
		}
		if last >= cutoff {
			n++
		}
	}
	return n
}

func (w *World) structureCreationScore(bp *catalogs.BlueprintDef, s *Structure, nowTick uint64) int {
	if bp == nil || s == nil {
		return 0
	}
	unique := map[string]bool{}
	hasStorage := false
	hasLight := false
	hasWorkshop := false
	hasGov := false

	for _, b := range bp.Blocks {
		unique[b.Block] = true
		switch b.Block {
		case "CHEST":
			hasStorage = true
		case "TORCH":
			hasLight = true
		case "CRAFTING_BENCH", "FURNACE":
			hasWorkshop = true
		case "BULLETIN_BOARD", "CONTRACT_TERMINAL", "CLAIM_TOTEM", "SIGN":
			hasGov = true
		}
	}

	base := 5
	complexity := int(math.Round(math.Log(1+float64(len(unique))) * 2))
	modules := 0
	if hasStorage {
		modules += 2
	}
	if hasLight {
		modules += 2
	}
	if hasWorkshop {
		modules += 2
	}
	if hasGov {
		modules += 2
	}

	stable := w.structureStable(bp, s.Anchor, s.Rotation)
	stability := 0
	if stable {
		stability = 3
	}

	users := w.structureUniqueUsers(s, nowTick, uint64(w.cfg.DayTicks))
	usageBonus := minInt(10, 2*users)

	return base + complexity + modules + stability + usageBonus
}

func (w *World) structureStable(bp *catalogs.BlueprintDef, anchor Vec3i, rotation int) bool {
	if bp == nil || len(bp.Blocks) == 0 {
		return true
	}
	rot := normalizeRotation(rotation)
	positions := make([]Vec3i, 0, len(bp.Blocks))
	index := map[Vec3i]int{}
	for i, b := range bp.Blocks {
		off := rotateOffset(b.Pos, rot)
		p := Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
		positions = append(positions, p)
		index[p] = i
	}
	visited := make([]bool, len(positions))
	queue := make([]int, 0, len(positions))

	// Seed BFS with blocks that have ground support (non-air below).
	for i, p := range positions {
		if p.Y <= 1 {
			visited[i] = true
			queue = append(queue, i)
			continue
		}
		below := Vec3i{X: p.X, Y: p.Y - 1, Z: p.Z}
		if _, ok := index[below]; ok {
			// Supported by structure; not ground.
			continue
		}
		if w.chunks.GetBlock(below) != w.chunks.gen.Air {
			visited[i] = true
			queue = append(queue, i)
		}
	}

	dirs := []Vec3i{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		p := positions[i]
		for _, d := range dirs {
			np := Vec3i{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
			ni, ok := index[np]
			if !ok || visited[ni] {
				continue
			}
			visited[ni] = true
			queue = append(queue, ni)
		}
	}

	count := 0
	for _, v := range visited {
		if v {
			count++
		}
	}
	if count == 0 {
		return false
	}
	return float64(count)/float64(len(visited)) >= 0.95
}

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

func (w *World) funDecay(a *Agent, key string, base int, nowTick uint64) int {
	if a == nil || base <= 0 {
		return 0
	}
	dw := a.funDecay[key]
	if dw == nil {
		dw = &funDecayWindow{StartTick: nowTick}
		a.funDecay[key] = dw
	}
	window := uint64(w.cfg.FunDecayWindowTicks)
	if window == 0 {
		window = 3000
	}
	if nowTick-dw.StartTick >= window {
		dw.StartTick = nowTick
		dw.Count = 0
	}
	dw.Count++
	baseMult := w.cfg.FunDecayBase
	if baseMult <= 0 || baseMult > 1.0 {
		baseMult = 0.70
	}
	mult := math.Pow(baseMult, float64(dw.Count-1))
	delta := int(math.Round(float64(base) * mult))
	if delta <= 0 {
		return 0
	}
	return delta
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

func socialFunFactor(a *Agent) float64 {
	// Map reputation 0..1000 to 0.5..1.0 multiplier (default rep=500 -> 1.0).
	rep := a.RepTrade
	if rep >= 500 {
		return 1.0
	}
	if rep <= 0 {
		return 0.5
	}
	return 0.5 + 0.5*(float64(rep)/500.0)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func itoaU64(v uint64) string { return strconv.FormatUint(v, 10) }
func itoaI(v int) string      { return strconv.Itoa(v) }
