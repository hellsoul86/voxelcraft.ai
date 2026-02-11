package world

import "voxelcraft.ai/internal/sim/world/logic/mathx"

func (w *World) spawnCrystalRift(nowTick uint64, center Vec3i) {
	ore, ok := w.catalogs.Blocks.Index["CRYSTAL_ORE"]
	if !ok {
		return
	}
	// 2D world: spawn a compact surface cluster on y=0.
	c := Vec3i{X: center.X, Y: 0, Z: center.Z}
	for dz := -2; dz <= 2; dz++ {
		for dx := -2; dx <= 2; dx++ {
			p := Vec3i{X: c.X + dx, Y: 0, Z: c.Z + dz}
			from := w.chunks.GetBlock(p)
			w.chunks.SetBlock(p, ore)
			w.auditSetBlock(nowTick, "WORLD", p, from, ore, "EVENT:CRYSTAL_RIFT")
		}
	}
}

func (w *World) spawnDeepVein(nowTick uint64, center Vec3i) {
	iron, ok1 := w.catalogs.Blocks.Index["IRON_ORE"]
	copper, ok2 := w.catalogs.Blocks.Index["COPPER_ORE"]
	if !ok1 || !ok2 {
		return
	}
	// 2D world: spawn a mixed ore patch on y=0.
	c := Vec3i{X: center.X, Y: 0, Z: center.Z}
	for dz := -3; dz <= 3; dz++ {
		for dx := -3; dx <= 3; dx++ {
			p := Vec3i{X: c.X + dx, Y: 0, Z: c.Z + dz}
			to := iron
			if (dx+dz)&1 == 0 {
				to = copper
			}
			from := w.chunks.GetBlock(p)
			w.chunks.SetBlock(p, to)
			w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:DEEP_VEIN")
		}
	}
}

func (w *World) spawnRuinsGate(nowTick uint64, center Vec3i) {
	brick, okB := w.catalogs.Blocks.Index["BRICK"]
	chest, okC := w.catalogs.Blocks.Index["CHEST"]
	if !okB || !okC {
		return
	}

	// Build a small ring with a loot chest in the center.
	p0 := Vec3i{X: center.X, Y: 0, Z: center.Z}

	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			p := Vec3i{X: p0.X + dx, Y: p0.Y, Z: p0.Z + dz}
			from := w.chunks.GetBlock(p)
			to := brick
			if dx == 0 && dz == 0 {
				to = chest
			}
			w.chunks.SetBlock(p, to)
			w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:RUINS_GATE")
			if dx == 0 && dz == 0 {
				c := w.ensureContainer(p, "CHEST")
				c.Inventory["CRYSTAL_SHARD"] += 2
				c.Inventory["IRON_INGOT"] += 4
				c.Inventory["COPPER_INGOT"] += 4
			}
		}
	}

	// Use the chest position as the event center marker.
	w.activeEventCenter = p0
}

func (w *World) spawnEventNoticeBoard(nowTick uint64, center Vec3i, eventID string, headline string, body string) {
	board, okB := w.catalogs.Blocks.Index["BULLETIN_BOARD"]
	sign, okS := w.catalogs.Blocks.Index["SIGN"]
	if !okB || !okS {
		return
	}

	boardPos := Vec3i{X: center.X, Y: 0, Z: center.Z}
	signPos := Vec3i{X: center.X + 1, Y: 0, Z: center.Z}

	from := w.chunks.GetBlock(boardPos)
	w.chunks.SetBlock(boardPos, board)
	w.auditSetBlock(nowTick, "WORLD", boardPos, from, board, "EVENT:"+eventID)
	w.ensureBoard(boardPos)
	if b := w.boards[boardIDAt(boardPos)]; b != nil {
		postID := w.newPostID()
		b.Posts = append(b.Posts, BoardPost{
			PostID: postID,
			Author: "WORLD",
			Title:  headline,
			Body:   body,
			Tick:   nowTick,
		})
	}

	from2 := w.chunks.GetBlock(signPos)
	w.chunks.SetBlock(signPos, sign)
	w.auditSetBlock(nowTick, "WORLD", signPos, from2, sign, "EVENT:"+eventID)
	s := w.ensureSign(signPos)
	s.Text = headline
	s.UpdatedTick = nowTick
	s.UpdatedBy = "WORLD"
}

func (w *World) spawnFloodWarning(nowTick uint64, center Vec3i) {
	water, ok := w.catalogs.Blocks.Index["WATER"]
	if !ok {
		return
	}
	for dz := -2; dz <= 2; dz++ {
		for dx := -2; dx <= 2; dx++ {
			p := Vec3i{X: center.X + dx, Y: 0, Z: center.Z + dz}
			from := w.chunks.GetBlock(p)
			w.chunks.SetBlock(p, water)
			w.auditSetBlock(nowTick, "WORLD", p, from, water, "EVENT:FLOOD_WARNING")
		}
	}
}

func (w *World) spawnBlightZone(nowTick uint64, center Vec3i) {
	gravel, ok := w.catalogs.Blocks.Index["GRAVEL"]
	if !ok {
		return
	}
	for dz := -3; dz <= 3; dz++ {
		for dx := -3; dx <= 3; dx++ {
			if mathx.AbsInt(dx)+mathx.AbsInt(dz) > 4 {
				continue
			}
			p := Vec3i{X: center.X + dx, Y: 0, Z: center.Z + dz}
			from := w.chunks.GetBlock(p)
			w.chunks.SetBlock(p, gravel)
			w.auditSetBlock(nowTick, "WORLD", p, from, gravel, "EVENT:BLIGHT_ZONE")
		}
	}
}

func (w *World) spawnBanditCamp(nowTick uint64, center Vec3i) {
	brick, okB := w.catalogs.Blocks.Index["BRICK"]
	chest, okC := w.catalogs.Blocks.Index["CHEST"]
	sign, okS := w.catalogs.Blocks.Index["SIGN"]
	if !okB || !okC || !okS {
		return
	}

	p0 := Vec3i{X: center.X, Y: 0, Z: center.Z}

	// Build a simple camp ring with a loot chest in the center.
	for dz := -2; dz <= 2; dz++ {
		for dx := -2; dx <= 2; dx++ {
			p := Vec3i{X: p0.X + dx, Y: p0.Y, Z: p0.Z + dz}
			from := w.chunks.GetBlock(p)
			to := w.chunks.gen.Air
			if dx == 0 && dz == 0 {
				to = chest
			} else if mathx.AbsInt(dx) == 2 || mathx.AbsInt(dz) == 2 {
				to = brick
			}
			w.chunks.SetBlock(p, to)
			w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:BANDIT_CAMP")
			if dx == 0 && dz == 0 {
				c := w.ensureContainer(p, "CHEST")
				c.Inventory["IRON_INGOT"] += 6
				c.Inventory["COPPER_INGOT"] += 4
				c.Inventory["CRYSTAL_SHARD"] += 1
				c.Inventory["BREAD"] += 2
			}
		}
	}

	// Sign marker.
	sp := Vec3i{X: p0.X + 3, Y: p0.Y, Z: p0.Z}
	fromS := w.chunks.GetBlock(sp)
	w.chunks.SetBlock(sp, sign)
	w.auditSetBlock(nowTick, "WORLD", sp, fromS, sign, "EVENT:BANDIT_CAMP")
	s := w.ensureSign(sp)
	s.Text = "BANDIT CAMP"
	s.UpdatedTick = nowTick
	s.UpdatedBy = "WORLD"

	// Use chest position as the event center marker.
	w.activeEventCenter = p0
}
