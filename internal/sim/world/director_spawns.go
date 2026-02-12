package world

import (
	spawnspkg "voxelcraft.ai/internal/sim/world/feature/director/spawns"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

func (w *World) spawnCrystalRift(nowTick uint64, center Vec3i) {
	ore, ok := w.catalogs.Blocks.Index["CRYSTAL_ORE"]
	if !ok {
		return
	}
	// 2D world: spawn a compact surface cluster on y=0.
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 2) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, ore)
		w.auditSetBlock(nowTick, "WORLD", p, from, ore, "EVENT:CRYSTAL_RIFT")
	}
}

func (w *World) spawnDeepVein(nowTick uint64, center Vec3i) {
	iron, ok1 := w.catalogs.Blocks.Index["IRON_ORE"]
	copper, ok2 := w.catalogs.Blocks.Index["COPPER_ORE"]
	if !ok1 || !ok2 {
		return
	}
	// 2D world: spawn a mixed ore patch on y=0.
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 3) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		to := iron
		if spawnspkg.DeepVeinIsCopper(pp.X-center.X, pp.Z-center.Z) {
			to = copper
		}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, to)
		w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:DEEP_VEIN")
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

	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: p0.X, Y: p0.Y, Z: p0.Z}, 1) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		to := brick
		if pp.X == p0.X && pp.Z == p0.Z {
			to = chest
		}
		w.chunks.SetBlock(p, to)
		w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:RUINS_GATE")
		if pp.X == p0.X && pp.Z == p0.Z {
			c := w.ensureContainer(p, "CHEST")
			c.Inventory["CRYSTAL_SHARD"] += 2
			c.Inventory["IRON_INGOT"] += 4
			c.Inventory["COPPER_INGOT"] += 4
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
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 2) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, water)
		w.auditSetBlock(nowTick, "WORLD", p, from, water, "EVENT:FLOOD_WARNING")
	}
}

func (w *World) spawnBlightZone(nowTick uint64, center Vec3i) {
	gravel, ok := w.catalogs.Blocks.Index["GRAVEL"]
	if !ok {
		return
	}
	for _, pp := range spawnspkg.Diamond(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 4) {
		dx := pp.X - center.X
		dz := pp.Z - center.Z
		if mathx.AbsInt(dx) > 3 || mathx.AbsInt(dz) > 3 {
			continue // keep legacy footprint
		}
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, gravel)
		w.auditSetBlock(nowTick, "WORLD", p, from, gravel, "EVENT:BLIGHT_ZONE")
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
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: p0.X, Y: p0.Y, Z: p0.Z}, 2) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		dx := pp.X - p0.X
		dz := pp.Z - p0.Z
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
