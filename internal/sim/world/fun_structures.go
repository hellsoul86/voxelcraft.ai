package world

import (
	"math"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"

	"voxelcraft.ai/internal/sim/catalogs"
)

func (w *World) registerStructure(nowTick uint64, builderID string, blueprintID string, anchor Vec3i, rotation int) {
	w.funInit()
	bp, ok := w.catalogs.Blueprints.ByID[blueprintID]
	if !ok {
		return
	}
	rot := blueprint.NormalizeRotation(rotation)

	id := fmtStructureID(builderID, nowTick, blueprintID, anchor)

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
	rot := blueprint.NormalizeRotation(rotation)
	positions := make([]Vec3i, 0, len(bp.Blocks))
	index := map[Vec3i]int{}
	for i, b := range bp.Blocks {
		off := blueprint.RotateOffset(b.Pos, rot)
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
