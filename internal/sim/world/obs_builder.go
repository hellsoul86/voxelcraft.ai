package world

import (
	"encoding/json"
	"sort"
	"strings"

	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/protocol"
	simenc "voxelcraft.ai/internal/sim/encoding"
	"voxelcraft.ai/internal/sim/tasks"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	boardspkg "voxelcraft.ai/internal/sim/world/feature/observer/boards"
	configpkg "voxelcraft.ai/internal/sim/world/feature/observer/config"
	entitiespkg "voxelcraft.ai/internal/sim/world/feature/observer/entities"
	metapkg "voxelcraft.ai/internal/sim/world/feature/observer/meta"
	observerruntimepkg "voxelcraft.ai/internal/sim/world/feature/observer/runtime"
	streamspkg "voxelcraft.ai/internal/sim/world/feature/observer/stream"
	taskspkg "voxelcraft.ai/internal/sim/world/feature/observer/tasks"
	progresspkg "voxelcraft.ai/internal/sim/world/feature/work/progress"
	"voxelcraft.ai/internal/sim/world/io/obscodec"
)

func (w *World) buildObs(a *Agent, cl *clientState, nowTick uint64) protocol.ObsMsg {
	center := a.Pos
	vox, sensorsNear := w.buildObsVoxels(center, cl)

	land, perms := w.permissionsFor(a.ID, a.Pos)
	if land != nil && land.CurfewEnabled {
		t := w.timeOfDay(nowTick)
		if claimspkg.InWindow(t, land.CurfewStart, land.CurfewEnd) {
			perms["can_build"] = false
			perms["can_break"] = false
		}
	}

	tasksObs := w.buildObsTasks(a, nowTick)
	ents := w.buildObsEntities(a, sensorsNear)

	// Public boards (global, MVP).
	boardInputs := make([]observerruntimepkg.BoardInput, 0, len(w.boards))
	if len(w.boards) > 0 {
		boardIDs := make([]string, 0, len(w.boards))
		for id := range w.boards {
			// For physical boards, only include nearby boards in OBS to keep payloads small.
			if typ, pos, ok := parseContainerID(id); ok && typ == "BULLETIN_BOARD" {
				if Manhattan(pos, a.Pos) > 32 {
					continue
				}
			}
			boardIDs = append(boardIDs, id)
		}
		sort.Strings(boardIDs)
		for _, bid := range boardIDs {
			b := w.boards[bid]
			if b == nil || len(b.Posts) == 0 {
				continue
			}
			posts := make([]observerruntimepkg.BoardPostInput, 0, len(b.Posts))
			for i := 0; i < len(b.Posts); i++ {
				p := b.Posts[i]
				posts = append(posts, observerruntimepkg.BoardPostInput{
					PostID: p.PostID,
					Author: p.Author,
					Title:  p.Title,
					Body:   p.Body,
				})
			}
			boardInputs = append(boardInputs, observerruntimepkg.BoardInput{
				BoardID: bid,
				Posts:   posts,
			})
		}
	}
	publicBoards := observerruntimepkg.BuildPublicBoards(boardInputs, 5, 120)

	localRules := protocol.LocalRulesObs{Permissions: perms}
	if land != nil {
		localRules.LandID = land.LandID
		localRules.Owner = land.Owner
		if land.Owner == a.ID {
			localRules.Role = "OWNER"
		} else if w.isLandMember(a.ID, land) {
			localRules.Role = "MEMBER"
		} else {
			localRules.Role = "VISITOR"
		}
		localRules.Tax = map[string]float64{"market": land.MarketTax}
		localRules.MaintenanceDueTick = land.MaintenanceDueTick
		localRules.MaintenanceStage = land.MaintenanceStage
	} else {
		localRules.Role = "WILD"
		localRules.Tax = map[string]float64{"market": 0.0}
	}

	status := observerruntimepkg.BuildStatus(a.Hunger, a.StaminaMilli, w.weather)

	obs := protocol.ObsMsg{
		Type:            protocol.TypeObs,
		ProtocolVersion: protocol.Version,
		Tick:            nowTick,
		AgentID:         a.ID,
		WorldID:         w.cfg.ID,
		WorldClock:      nowTick,
		World: protocol.WorldObs{
			TimeOfDay:           float64(int(nowTick)%w.cfg.DayTicks) / float64(w.cfg.DayTicks),
			Weather:             w.weather,
			SeasonDay:           w.seasonDay(nowTick),
			Biome:               biomeAt(w.cfg.Seed, a.Pos.X, a.Pos.Z, w.cfg.BiomeRegionSize),
			ActiveEvent:         w.activeEventID,
			ActiveEventEndsTick: w.activeEventEnds,
		},
		Self: protocol.SelfObs{
			Pos:     a.Pos.ToArray(),
			Yaw:     a.Yaw,
			HP:      a.HP,
			Hunger:  a.Hunger,
			Stamina: float64(a.StaminaMilli) / 1000.0,
			Status:  status,
			Reputation: protocol.ReputationObs{
				Trade:  float64(a.RepTrade) / 1000.0,
				Build:  float64(a.RepBuild) / 1000.0,
				Social: float64(a.RepSocial) / 1000.0,
				Law:    float64(a.RepLaw) / 1000.0,
			},
		},
		Inventory: a.InventoryList(),
		Equipment: protocol.EquipmentObs{
			MainHand: a.Equipment.MainHand,
			Armor:    []string{a.Equipment.Armor[0], a.Equipment.Armor[1], a.Equipment.Armor[2], a.Equipment.Armor[3]},
		},
		LocalRules:   localRules,
		Voxels:       vox,
		Entities:     ents,
		Tasks:        tasksObs,
		PublicBoards: publicBoards,
	}
	w.attachObsEventsAndMeta(a, &obs, nowTick)
	return obs
}

func (w *World) buildObsTasks(a *Agent, nowTick uint64) []protocol.TaskObs {
	var moveIn *taskspkg.MoveInput
	if a.MoveTask != nil {
		moveIn = &taskspkg.MoveInput{
			TaskID:    a.MoveTask.TaskID,
			Kind:      string(a.MoveTask.Kind),
			Target:    taskspkg.Vec3{X: a.MoveTask.Target.X, Y: a.MoveTask.Target.Y, Z: a.MoveTask.Target.Z},
			StartPos:  taskspkg.Vec3{X: a.MoveTask.StartPos.X, Y: a.MoveTask.StartPos.Y, Z: a.MoveTask.StartPos.Z},
			TargetID:  a.MoveTask.TargetID,
			Distance:  a.MoveTask.Distance,
			Tolerance: a.MoveTask.Tolerance,
		}
	}
	var workIn *taskspkg.WorkInput
	if a.WorkTask != nil {
		workIn = &taskspkg.WorkInput{
			TaskID:   a.WorkTask.TaskID,
			Kind:     string(a.WorkTask.Kind),
			Progress: w.workProgressForAgent(a, a.WorkTask),
		}
	}
	return taskspkg.BuildTasks(taskspkg.BuildInput{
		SelfPos: taskspkg.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		Move:    moveIn,
		Work:    workIn,
	}, func(id string) (taskspkg.Vec3, bool) {
		if t, ok := w.followTargetPos(id); ok {
			return taskspkg.Vec3{X: t.X, Y: t.Y, Z: t.Z}, true
		}
		return taskspkg.Vec3{}, false
	})
}

func (w *World) buildObsEntities(a *Agent, sensorsNear []Vec3i) []protocol.EntityObs {
	ents := make([]protocol.EntityObs, 0, 32)
	selfPos := entitiespkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}

	agentInputs := make([]entitiespkg.AgentInput, 0, len(w.agents))
	for _, other := range w.agents {
		agentInputs = append(agentInputs, entitiespkg.AgentInput{
			ID:       other.ID,
			Pos:      entitiespkg.Pos{X: other.Pos.X, Y: other.Pos.Y, Z: other.Pos.Z},
			OrgID:    other.OrgID,
			RepTrade: other.RepTrade,
			RepLaw:   other.RepLaw,
		})
	}
	ents = append(ents, entitiespkg.BuildAgentEntities(a.ID, selfPos, agentInputs, 16)...)

	containers := make([]entitiespkg.SimpleInput, 0, len(w.containers))
	for _, c := range w.containers {
		if !entitiespkg.IsNear(selfPos, entitiespkg.Pos{X: c.Pos.X, Y: c.Pos.Y, Z: c.Pos.Z}, 16) {
			continue
		}
		containers = append(containers, entitiespkg.SimpleInput{
			ID:   c.ID(),
			Type: c.Type,
			Pos:  entitiespkg.Pos{X: c.Pos.X, Y: c.Pos.Y, Z: c.Pos.Z},
		})
	}
	ents = append(ents, entitiespkg.BuildSimpleEntities(containers)...)

	if len(w.boards) > 0 {
		boardEntries := make([]entitiespkg.SimpleInput, 0, len(w.boards))
		for id := range w.boards {
			typ, pos, ok := parseContainerID(id)
			if !ok || typ != "BULLETIN_BOARD" {
				continue
			}
			if !entitiespkg.IsNear(selfPos, entitiespkg.Pos{X: pos.X, Y: pos.Y, Z: pos.Z}, 16) {
				continue
			}
			boardEntries = append(boardEntries, entitiespkg.SimpleInput{
				ID:   id,
				Type: "BULLETIN_BOARD",
				Pos:  entitiespkg.Pos{X: pos.X, Y: pos.Y, Z: pos.Z},
			})
		}
		sort.Slice(boardEntries, func(i, j int) bool { return boardEntries[i].ID < boardEntries[j].ID })
		ents = append(ents, entitiespkg.BuildSimpleEntities(boardEntries)...)
	}
	if len(w.signs) > 0 {
		signs := make([]entitiespkg.SignInput, 0, len(w.signs))
		for _, p := range w.sortedSignPositionsNear(a.Pos, 16) {
			s := w.signs[p]
			text := ""
			if s != nil {
				text = s.Text
			}
			signs = append(signs, entitiespkg.SignInput{
				ID:   signIDAt(p),
				Pos:  entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				Text: text,
			})
		}
		ents = append(ents, entitiespkg.BuildSignEntities(signs)...)
	}
	if len(w.conveyors) > 0 {
		conveyors := make([]entitiespkg.ConveyorInput, 0, len(w.conveyors))
		for _, p := range w.sortedConveyorPositionsNear(a.Pos, 16) {
			m := w.conveyors[p]
			conveyors = append(conveyors, entitiespkg.ConveyorInput{
				ID:     conveyorIDAt(p),
				Pos:    entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				DirTag: conveyorDirTag(m),
			})
		}
		ents = append(ents, entitiespkg.BuildConveyorEntities(conveyors)...)
	}
	if len(w.switches) > 0 {
		switches := make([]entitiespkg.SwitchInput, 0, len(w.switches))
		for _, p := range w.sortedSwitchPositionsNear(a.Pos, 16) {
			switches = append(switches, entitiespkg.SwitchInput{
				ID:  switchIDAt(p),
				Pos: entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				On:  w.switches[p],
			})
		}
		ents = append(ents, entitiespkg.BuildSwitchEntities(switches)...)
	}
	if len(sensorsNear) > 0 {
		sort.Slice(sensorsNear, func(i, j int) bool {
			if sensorsNear[i].X != sensorsNear[j].X {
				return sensorsNear[i].X < sensorsNear[j].X
			}
			if sensorsNear[i].Y != sensorsNear[j].Y {
				return sensorsNear[i].Y < sensorsNear[j].Y
			}
			return sensorsNear[i].Z < sensorsNear[j].Z
		})
		sensors := make([]entitiespkg.SensorInput, 0, len(sensorsNear))
		for _, p := range sensorsNear {
			sensors = append(sensors, entitiespkg.SensorInput{
				ID:  containerID("SENSOR", p),
				Pos: entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				On:  w.sensorOn(p),
			})
		}
		ents = append(ents, entitiespkg.BuildSensorEntities(sensors)...)
	}
	if len(w.items) > 0 {
		items := make([]entitiespkg.ItemInput, 0, len(w.items))
		for _, e := range w.items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			if !entitiespkg.IsNear(selfPos, entitiespkg.Pos{X: e.Pos.X, Y: e.Pos.Y, Z: e.Pos.Z}, 16) {
				continue
			}
			items = append(items, entitiespkg.ItemInput{
				ID:    e.EntityID,
				Pos:   entitiespkg.Pos{X: e.Pos.X, Y: e.Pos.Y, Z: e.Pos.Z},
				Item:  e.Item,
				Count: e.Count,
			})
		}
		ents = append(ents, entitiespkg.BuildItemEntities(items)...)
	}
	return ents
}

func (w *World) attachObsEventsAndMeta(a *Agent, obs *protocol.ObsMsg, nowTick uint64) {
	ev := a.TakeEvents()
	obs.Events = ev
	obs.EventsCursor = a.EventCursor
	obs.ObsID = metapkg.ObsID(a.ID, nowTick, a.EventCursor)
	obs.FunScore = metapkg.FunScorePtr(
		a.Fun.Novelty,
		a.Fun.Creation,
		a.Fun.Social,
		a.Fun.Influence,
		a.Fun.Narrative,
		a.Fun.RiskRescue,
	)

	if len(a.PendingMemory) > 0 {
		obs.Memory = a.PendingMemory
		a.PendingMemory = nil
	}
}

func (w *World) buildObsVoxels(center Vec3i, cl *clientState) (protocol.VoxelsObs, []Vec3i) {
	r := w.cfg.ObsRadius
	sensorBlock, hasSensor := w.catalogs.Blocks.Index["SENSOR"]
	sensorsNear := make([]Vec3i, 0, 4)
	dim := 2*r + 1
	plane := dim * dim
	total := plane * dim
	curr := make([]uint16, total)

	// 2D world optimization: only the y==0 slice can be non-AIR; all other y are read-as-AIR.
	// Keep the same scan order (dy outer, dz middle, dx inner) so DELTA ops remain stable.
	air := w.chunks.gen.Air
	if air != 0 {
		for i := range curr {
			curr[i] = air
		}
	}
	// Fill only the slice where world Y equals 0.
	dy0 := -center.Y
	if dy0 >= -r && dy0 <= r {
		layerOff := (dy0 + r) * plane
		for dz := -r; dz <= r; dz++ {
			rowOff := layerOff + (dz+r)*dim
			for dx := -r; dx <= r; dx++ {
				p := Vec3i{X: center.X + dx, Y: 0, Z: center.Z + dz}
				b := w.chunks.GetBlock(p)
				curr[rowOff+(dx+r)] = b
				if hasSensor && b == sensorBlock {
					sensorsNear = append(sensorsNear, p)
				}
			}
		}
	}

	vox := protocol.VoxelsObs{
		Center:   center.ToArray(),
		Radius:   r,
		Encoding: "RLE",
	}

	if cl.DeltaVoxels && cl.LastVoxels != nil && len(cl.LastVoxels) == len(curr) {
		ops := obscodec.BuildDeltaOps(cl.LastVoxels, curr, r)
		if len(ops) > 0 && len(ops) < len(curr)/2 {
			vox.Encoding = "DELTA"
			vox.Ops = ops
		} else {
			vox.Data = simenc.EncodeRLE(curr)
		}
	} else {
		vox.Data = simenc.EncodeRLE(curr)
	}
	cl.LastVoxels = curr

	return vox, sensorsNear
}

func (w *World) observerMoveTaskState(a *Agent, nowTick uint64) *observerproto.TaskState {
	if w == nil || a == nil || a.MoveTask == nil {
		return nil
	}
	mt := a.MoveTask
	return taskspkg.BuildMoveTaskState(
		taskspkg.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		taskspkg.MoveStateInput{
			Kind:      string(mt.Kind),
			Target:    taskspkg.Vec3{X: mt.Target.X, Y: mt.Target.Y, Z: mt.Target.Z},
			StartPos:  taskspkg.Vec3{X: mt.StartPos.X, Y: mt.StartPos.Y, Z: mt.StartPos.Z},
			TargetID:  mt.TargetID,
			Distance:  mt.Distance,
			Tolerance: mt.Tolerance,
		},
		func(id string) (taskspkg.Vec3, bool) {
			t, ok := w.followTargetPos(id)
			return taskspkg.Vec3{X: t.X, Y: t.Y, Z: t.Z}, ok
		},
	)
}

func (w *World) observerWorkTaskState(a *Agent) *observerproto.TaskState {
	if a == nil || a.WorkTask == nil {
		return nil
	}
	wt := a.WorkTask
	return taskspkg.BuildWorkTaskState(string(wt.Kind), w.workProgressForAgent(a, wt))
}

func (w *World) workProgressForAgent(a *Agent, wt *tasks.WorkTask) float64 {
	if a == nil || wt == nil {
		return 0
	}
	switch wt.Kind {
	case tasks.KindMine:
		pos := v3FromTask(wt.BlockPos)
		blockName := w.blockName(w.chunks.GetBlock(pos))
		return progresspkg.MineProgress(wt.WorkTicks, blockName, a.Inventory)
	case tasks.KindCraft:
		rec, ok := w.catalogs.Recipes.ByID[wt.RecipeID]
		if !ok {
			return 0
		}
		return progresspkg.TimedProgress(wt.WorkTicks, rec.TimeTicks)
	case tasks.KindSmelt:
		rec, ok := w.smeltByInput[wt.ItemID]
		if !ok {
			return 0
		}
		return progresspkg.TimedProgress(wt.WorkTicks, rec.TimeTicks)
	case tasks.KindBuildBlueprint:
		bp, ok := w.catalogs.Blueprints.ByID[wt.BlueprintID]
		if !ok {
			return 0
		}
		return progresspkg.BlueprintProgress(wt.BuildIndex, len(bp.Blocks))
	default:
		return 0
	}
}

func (w *World) Config() WorldConfig {
	if w == nil {
		return WorldConfig{}
	}
	cfg := w.cfg
	if cfg.MaintenanceCost != nil {
		m := make(map[string]int, len(cfg.MaintenanceCost))
		for k, v := range cfg.MaintenanceCost {
			m[k] = v
		}
		cfg.MaintenanceCost = m
	}
	return cfg
}

func (w *World) BlockPalette() []string {
	if w == nil || w.catalogs == nil {
		return nil
	}
	p := w.catalogs.Blocks.Palette
	out := make([]string, len(p))
	copy(out, p)
	return out
}

func (w *World) newPostID() string {
	return boardspkg.NewPostID(w.nextPostNum.Add(1))
}

func boardIDAt(pos Vec3i) string {
	return containerID("BULLETIN_BOARD", pos)
}

func (w *World) ensureBoard(pos Vec3i) *Board {
	id := boardIDAt(pos)
	b := w.boards[id]
	if b != nil {
		return b
	}
	b = &Board{BoardID: id}
	w.boards[id] = b
	return b
}

func (w *World) removeBoard(pos Vec3i) {
	delete(w.boards, boardIDAt(pos))
}

func (w *World) handleObserverJoin(req ObserverJoinRequest) {
	if w == nil || req.SessionID == "" || req.TickOut == nil || req.DataOut == nil {
		return
	}

	cfg := observerCfg{
		chunkRadius:    clampInt(configpkg.ClampChunkRadius(req.ChunkRadius), 1, 32, 6),
		maxChunks:      clampInt(configpkg.ClampMaxChunks(req.MaxChunks), 1, 16384, 1024),
		focusAgentID:   configpkg.NormalizeFocusAgentID(req.FocusAgentID),
		voxelRadius:    clampInt(configpkg.ClampVoxelRadius(req.VoxelRadius), 0, 8, 0),
		voxelMaxChunks: clampInt(configpkg.ClampVoxelMaxChunks(req.VoxelMaxChunks), 1, 2048, 256),
	}

	// Replace existing session id if any (defensive).
	if old := w.observers[req.SessionID]; old != nil {
		close(old.tickOut)
		close(old.dataOut)
	}

	w.observers[req.SessionID] = &observerClient{
		id:          req.SessionID,
		tickOut:     req.TickOut,
		dataOut:     req.DataOut,
		cfg:         cfg,
		chunks:      map[ChunkKey]*observerChunk{},
		voxelChunks: map[ChunkKey]*observerVoxelChunk{},
	}
}

func (w *World) handleObserverSubscribe(req ObserverSubscribeRequest) {
	c := w.observers[req.SessionID]
	if c == nil {
		return
	}
	c.cfg.chunkRadius = clampInt(req.ChunkRadius, 1, 32, c.cfg.chunkRadius)
	c.cfg.maxChunks = clampInt(req.MaxChunks, 1, 16384, c.cfg.maxChunks)

	// Voxel streaming config. Allow disabling by sending voxel_radius=0.
	c.cfg.focusAgentID = configpkg.NormalizeFocusAgentID(req.FocusAgentID)
	c.cfg.voxelRadius = configpkg.ClampVoxelRadius(req.VoxelRadius)
	if req.VoxelMaxChunks > 0 {
		c.cfg.voxelMaxChunks = clampInt(configpkg.ClampVoxelMaxChunks(req.VoxelMaxChunks), 1, 2048, c.cfg.voxelMaxChunks)
	}
}

func (w *World) handleObserverLeave(sessionID string) {
	if sessionID == "" {
		return
	}
	c := w.observers[sessionID]
	if c == nil {
		return
	}
	delete(w.observers, sessionID)
	close(c.tickOut)
	close(c.dataOut)
}

func (w *World) computeChunkSurface(cx, cz int) []surfaceCell {
	ch := w.chunkForSurface(cx, cz)
	air := w.chunks.gen.Air
	boundaryR := w.chunks.gen.BoundaryR
	out := make([]surfaceCell, 16*16)
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			wx := cx*16 + x
			wz := cz*16 + z
			if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
				out[z*16+x] = surfaceCell{b: air, y: 0}
				continue
			}
			b := air
			v := ch.Blocks[x+z*16]
			if v != air {
				b = v
			}
			out[z*16+x] = surfaceCell{b: b, y: 0}
		}
	}
	return out
}

func (w *World) computeChunkVoxels(cx, cz int) []uint16 {
	ch := w.chunkForVoxels(cx, cz)
	if ch == nil || ch.Blocks == nil {
		return nil
	}
	out := make([]uint16, len(ch.Blocks))
	copy(out, ch.Blocks)

	// Respect world boundary the same way ChunkStore.GetBlock does.
	if w.chunks != nil && w.chunks.gen.BoundaryR > 0 {
		air := w.chunks.gen.Air
		br := w.chunks.gen.BoundaryR
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				wx := cx*16 + x
				wz := cz*16 + z
				if wx < -br || wx > br || wz < -br || wz > br {
					out[x+z*16] = air
				}
			}
		}
	}
	return out
}

func (w *World) computeSurfaceCellAt(wx, wz int) surfaceCell {
	air := w.chunks.gen.Air
	boundaryR := w.chunks.gen.BoundaryR
	if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
		return surfaceCell{b: air, y: 0}
	}
	cx := floorDiv(wx, 16)
	cz := floorDiv(wz, 16)
	lx := mod(wx, 16)
	lz := mod(wz, 16)
	ch := w.chunkForSurface(cx, cz)
	v := ch.Blocks[lx+lz*16]
	if v != air {
		return surfaceCell{b: v, y: 0}
	}
	return surfaceCell{b: air, y: 0}
}

func (w *World) chunkForSurface(cx, cz int) *Chunk {
	if w == nil || w.chunks == nil {
		return &Chunk{CX: cx, CZ: cz, Blocks: nil}
	}
	k := ChunkKey{CX: cx, CZ: cz}
	if ch, ok := w.chunks.chunks[k]; ok && ch != nil {
		return ch
	}
	// Generate an ephemeral chunk without mutating the world's loaded chunk set. This ensures
	// observer clients cannot affect simulation state/digests by "viewing" far-away terrain.
	tmp := &Chunk{
		CX:     cx,
		CZ:     cz,
		Blocks: make([]uint16, 16*16),
	}
	w.chunks.generateChunk(tmp)
	return tmp
}

func (w *World) chunkForVoxels(cx, cz int) *Chunk {
	// Same semantics as chunkForSurface; kept separate to make intent explicit.
	return w.chunkForSurface(cx, cz)
}

func trySend(ch chan []byte, b []byte) bool {
	select {
	case ch <- b:
		return true
	default:
		return false
	}
}

func clampInt(v, min, max, def int) int { return streamspkg.ClampInt(v, min, max, def) }

func (w *World) stepObserverChunksForClient(nowTick uint64, c *observerClient, connected []ChunkKey, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}

	in := make([]streamspkg.ChunkKey, 0, len(connected))
	for _, k := range connected {
		in = append(in, streamspkg.ChunkKey{CX: k.CX, CZ: k.CZ})
	}
	rawWanted := streamspkg.ComputeWantedChunks(in, c.cfg.chunkRadius, c.cfg.maxChunks)
	wantKeys := make([]ChunkKey, 0, len(rawWanted))
	for _, k := range rawWanted {
		wantKeys = append(wantKeys, ChunkKey{CX: k.CX, CZ: k.CZ})
	}
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	// Track wanted chunks, enqueue full surfaces as needed (rate-limited).
	fullBudget := observerMaxFullChunksPerTick
	canSend := true
	for _, k := range wantKeys {
		st := c.chunks[k]
		if st == nil {
			st = &observerChunk{
				key:            k,
				lastWantedTick: nowTick,
				needsFull:      true,
			}
			c.chunks[k] = st
		} else {
			st.lastWantedTick = nowTick
		}

		if canSend && st.needsFull && fullBudget > 0 {
			if st.surface == nil {
				st.surface = w.computeChunkSurface(k.CX, k.CZ)
			}
			if w.sendChunkSurface(c, st) {
				st.sentFull = true
				st.needsFull = false
				fullBudget--
			} else {
				// Channel is likely full; don't spend more CPU on sends this tick.
				canSend = false
			}
		}
	}

	// Apply audits -> patch (or force resync).
	patches := map[ChunkKey]map[int]observerproto.ChunkPatchCell{}
	for _, e := range audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx := e.Pos[0]
		wz := e.Pos[2]
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		st := c.chunks[key]
		if st == nil {
			continue
		}
		// If we don't have a baseline surface yet, just ignore PATCH; a future full send will catch up.
		if st.surface == nil {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lz*16 + lx
		newCell := w.computeSurfaceCellAt(wx, wz)
		old := st.surface[idx]
		if old == newCell {
			continue
		}
		st.surface[idx] = newCell
		if st.needsFull {
			continue
		}
		m := patches[key]
		if m == nil {
			m = map[int]observerproto.ChunkPatchCell{}
			patches[key] = m
		}
		m[idx] = observerproto.ChunkPatchCell{X: lx, Z: lz, Block: newCell.b, Y: int(newCell.y)}
	}

	for key, m := range patches {
		st := c.chunks[key]
		if st == nil || st.needsFull {
			continue
		}
		cells := make([]observerproto.ChunkPatchCell, 0, len(m))
		for _, cell := range m {
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Z != cells[j].Z {
				return cells[i].Z < cells[j].Z
			}
			return cells[i].X < cells[j].X
		})
		msg := observerproto.ChunkPatchMsg{
			Type:            "CHUNK_PATCH",
			ProtocolVersion: observerproto.Version,
			CX:              key.CX,
			CZ:              key.CZ,
			Cells:           cells,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if !trySend(c.dataOut, b) {
			// Force a full resync next tick if PATCH is dropped.
			st.needsFull = true
		}
	}

	// Evictions.
	if len(c.chunks) > 0 {
		var evictKeys []ChunkKey
		for k, st := range c.chunks {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if nowTick-st.lastWantedTick < observerEvictAfterTicks {
				continue
			}
			if !st.sentFull {
				evictKeys = append(evictKeys, k)
				continue
			}

			msg := observerproto.ChunkEvictMsg{
				Type:            "CHUNK_EVICT",
				ProtocolVersion: observerproto.Version,
				CX:              k.CX,
				CZ:              k.CZ,
			}
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if trySend(c.dataOut, b) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(c.chunks, k)
		}
	}
}

func (w *World) sendChunkSurface(c *observerClient, st *observerChunk) bool {
	if w == nil || c == nil || st == nil || st.surface == nil {
		return false
	}
	msg := observerproto.ChunkSurfaceMsg{
		Type:            "CHUNK_SURFACE",
		ProtocolVersion: observerproto.Version,
		CX:              st.key.CX,
		CZ:              st.key.CZ,
		Encoding:        "PAL16_Y8",
		Data:            encodePAL16Y8(st.surface),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return trySend(c.dataOut, b)
}

func encodePAL16Y8(surface []surfaceCell) string {
	blocks := make([]uint16, len(surface))
	ys := make([]byte, len(surface))
	for i, c := range surface {
		blocks[i] = c.b
		ys[i] = c.y
	}
	return obscodec.EncodePAL16Y8(blocks, ys)
}

// ObserverJoinRequest registers a read-only observer session that receives:
// - chunk surface tiles (dataOut)
// - per-tick global state (tickOut)
//
// All observer state is maintained by the world loop goroutine.
type ObserverJoinRequest struct {
	SessionID string
	TickOut   chan []byte
	DataOut   chan []byte

	ChunkRadius int
	MaxChunks   int

	// Optional: stream 3D voxels around a focused agent.
	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
}

// ObserverSubscribeRequest updates an existing observer session subscription settings.
type ObserverSubscribeRequest struct {
	SessionID string

	ChunkRadius int
	MaxChunks   int

	// Optional: stream 3D voxels around a focused agent.
	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
}

type observerClient struct {
	id      string
	tickOut chan []byte
	dataOut chan []byte

	cfg observerCfg

	// Chunks tracked for this observer (may be pending full send).
	chunks map[ChunkKey]*observerChunk

	// Voxel chunks tracked for this observer (for 3D rendering).
	voxelChunks map[ChunkKey]*observerVoxelChunk
}

type observerCfg struct {
	chunkRadius int
	maxChunks   int

	focusAgentID   string
	voxelRadius    int
	voxelMaxChunks int
}

type observerChunk struct {
	key ChunkKey

	// lastWantedTick is updated whenever the chunk is in the current wanted set.
	lastWantedTick uint64

	// sentFull indicates we have enqueued at least one CHUNK_SURFACE for this chunk.
	sentFull bool

	// needsFull forces a CHUNK_SURFACE resend (e.g. after a dropped PATCH).
	needsFull bool

	// surface is the last known surface state for the chunk, used for PATCH diffs.
	// Populated lazily when we send (or attempt to send) a full surface.
	surface []surfaceCell // len=256
}

type surfaceCell struct {
	b uint16
	y uint8
}

type observerVoxelChunk struct {
	key ChunkKey

	lastWantedTick uint64

	sentFull  bool
	needsFull bool

	// blocks is the last known chunk block array, used for patch updates.
	// Populated lazily when we send (or attempt to send) a full chunk.
	blocks []uint16 // len = 16*16*height
}

const (
	observerEvictAfterTicks      = 50
	observerMaxFullChunksPerTick = 32

	observerVoxelEvictAfterTicks      = 10
	observerMaxFullVoxelChunksPerTick = 8
)

func (w *World) stepObservers(nowTick uint64, joins []RecordedJoin, leaves []string, actions []RecordedAction) {
	if w == nil || len(w.observers) == 0 {
		return
	}

	// Snapshot agent list (sorted for stable output).
	agentsSorted := w.sortedAgents()
	agentStates := make([]observerproto.AgentState, 0, len(agentsSorted))

	connectedChunks := make([]ChunkKey, 0, len(agentsSorted))
	for _, a := range agentsSorted {
		if a == nil {
			continue
		}
		connected := w.clients[a.ID] != nil
		if connected {
			cx := floorDiv(a.Pos.X, 16)
			cz := floorDiv(a.Pos.Z, 16)
			connectedChunks = append(connectedChunks, ChunkKey{CX: cx, CZ: cz})
		}

		st := observerproto.AgentState{
			ID:           a.ID,
			Name:         a.Name,
			Connected:    connected,
			OrgID:        a.OrgID,
			Pos:          a.Pos.ToArray(),
			Yaw:          a.Yaw,
			HP:           a.HP,
			Hunger:       a.Hunger,
			StaminaMilli: a.StaminaMilli,
		}
		if a.MoveTask != nil {
			st.MoveTask = w.observerMoveTaskState(a, nowTick)
		}
		if a.WorkTask != nil {
			st.WorkTask = w.observerWorkTaskState(a)
		}
		agentStates = append(agentStates, st)
	}

	joinsOut := make([]observerproto.JoinInfo, 0, len(joins))
	for _, j := range joins {
		joinsOut = append(joinsOut, observerproto.JoinInfo{AgentID: j.AgentID, Name: j.Name})
	}

	actionsOut := make([]observerproto.RecordedAction, 0, len(actions))
	for _, a := range actions {
		actionsOut = append(actionsOut, observerproto.RecordedAction{AgentID: a.AgentID, Act: a.Act})
	}

	auditsOut := make([]observerproto.AuditEntry, 0, len(w.obsAuditsThisTick))
	for _, e := range w.obsAuditsThisTick {
		auditsOut = append(auditsOut, observerproto.AuditEntry{
			Tick:   e.Tick,
			Actor:  e.Actor,
			Action: e.Action,
			Pos:    e.Pos,
			From:   e.From,
			To:     e.To,
			Reason: e.Reason,
		})
	}

	// Chunk surfaces / patches are per-observer (depends on subscription settings).
	for _, c := range w.observers {
		w.stepObserverChunksForClient(nowTick, c, connectedChunks, w.obsAuditsThisTick)
		w.stepObserverVoxelChunksForClient(nowTick, c, w.obsAuditsThisTick)
	}

	msg := observerproto.TickMsg{
		Type:                "TICK",
		ProtocolVersion:     observerproto.Version,
		Tick:                nowTick,
		TimeOfDay:           w.timeOfDay(nowTick),
		Weather:             w.weather,
		ActiveEventID:       w.activeEventID,
		ActiveEventEndsTick: w.activeEventEnds,
		Agents:              agentStates,
		Joins:               joinsOut,
		Leaves:              leaves,
		Actions:             actionsOut,
		Audits:              auditsOut,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for _, c := range w.observers {
		sendLatest(c.tickOut, b)
	}
}

func (w *World) stepObserverVoxelChunksForClient(nowTick uint64, c *observerClient, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}

	focusID := strings.TrimSpace(c.cfg.focusAgentID)
	radius := c.cfg.voxelRadius
	if focusID == "" || radius <= 0 {
		// If voxels are disabled, evict any previously-sent voxel chunks immediately.
		if len(c.voxelChunks) > 0 {
			for k, st := range c.voxelChunks {
				if st != nil && st.sentFull {
					msg := observerproto.ChunkVoxelsEvictMsg{
						Type:            "CHUNK_VOXELS_EVICT",
						ProtocolVersion: observerproto.Version,
						CX:              k.CX,
						CZ:              k.CZ,
					}
					if b, err := json.Marshal(msg); err == nil {
						_ = trySend(c.dataOut, b)
					}
				}
				delete(c.voxelChunks, k)
			}
		}
		return
	}

	a := w.agents[focusID]
	if a == nil {
		return
	}

	center := ChunkKey{CX: floorDiv(a.Pos.X, 16), CZ: floorDiv(a.Pos.Z, 16)}
	rawWanted := streamspkg.ComputeWantedChunks([]streamspkg.ChunkKey{{CX: center.CX, CZ: center.CZ}}, radius, c.cfg.voxelMaxChunks)
	wantKeys := make([]ChunkKey, 0, len(rawWanted))
	for _, k := range rawWanted {
		wantKeys = append(wantKeys, ChunkKey{CX: k.CX, CZ: k.CZ})
	}
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	// Track wanted chunks and enqueue full voxel chunks as needed (rate-limited).
	fullBudget := observerMaxFullVoxelChunksPerTick
	canSend := true
	for _, k := range wantKeys {
		st := c.voxelChunks[k]
		if st == nil {
			st = &observerVoxelChunk{
				key:            k,
				lastWantedTick: nowTick,
				needsFull:      true,
			}
			c.voxelChunks[k] = st
		} else {
			st.lastWantedTick = nowTick
		}

		if canSend && st.needsFull && fullBudget > 0 {
			if st.blocks == nil {
				st.blocks = w.computeChunkVoxels(k.CX, k.CZ)
			}
			if w.sendChunkVoxels(c, st) {
				st.sentFull = true
				st.needsFull = false
				fullBudget--
			} else {
				// Channel is likely full; don't spend more CPU on sends this tick.
				canSend = false
			}
		}
	}

	// Apply audits -> voxel patch (or force resync).
	patches := map[ChunkKey]map[int]observerproto.ChunkVoxelPatchCell{}
	for _, e := range audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx, wy, wz := e.Pos[0], e.Pos[1], e.Pos[2]
		// 2D world: only y==0 exists.
		if wy != 0 {
			continue
		}
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		if _, ok := wantSet[key]; !ok {
			continue
		}
		st := c.voxelChunks[key]
		if st == nil || st.blocks == nil {
			continue
		}
		if st.needsFull {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lx + lz*16
		if idx < 0 || idx >= len(st.blocks) {
			continue
		}
		if st.blocks[idx] == e.To {
			continue
		}
		st.blocks[idx] = e.To

		m := patches[key]
		if m == nil {
			m = map[int]observerproto.ChunkVoxelPatchCell{}
			patches[key] = m
		}
		m[idx] = observerproto.ChunkVoxelPatchCell{X: lx, Y: 0, Z: lz, Block: e.To}
	}

	for key, m := range patches {
		st := c.voxelChunks[key]
		if st == nil || st.needsFull {
			continue
		}
		cells := make([]observerproto.ChunkVoxelPatchCell, 0, len(m))
		for _, cell := range m {
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Y != cells[j].Y {
				return cells[i].Y < cells[j].Y
			}
			if cells[i].Z != cells[j].Z {
				return cells[i].Z < cells[j].Z
			}
			return cells[i].X < cells[j].X
		})
		msg := observerproto.ChunkVoxelPatchMsg{
			Type:            "CHUNK_VOXEL_PATCH",
			ProtocolVersion: observerproto.Version,
			CX:              key.CX,
			CZ:              key.CZ,
			Cells:           cells,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if !trySend(c.dataOut, b) {
			// Force a full resync next tick if PATCH is dropped.
			st.needsFull = true
		}
	}

	// Evictions.
	if len(c.voxelChunks) > 0 {
		var evictKeys []ChunkKey
		for k, st := range c.voxelChunks {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if nowTick-st.lastWantedTick < observerVoxelEvictAfterTicks {
				continue
			}
			if !st.sentFull {
				evictKeys = append(evictKeys, k)
				continue
			}

			msg := observerproto.ChunkVoxelsEvictMsg{
				Type:            "CHUNK_VOXELS_EVICT",
				ProtocolVersion: observerproto.Version,
				CX:              k.CX,
				CZ:              k.CZ,
			}
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if trySend(c.dataOut, b) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(c.voxelChunks, k)
		}
	}
}

func (w *World) sendChunkVoxels(c *observerClient, st *observerVoxelChunk) bool {
	if w == nil || c == nil || st == nil || st.blocks == nil {
		return false
	}
	msg := observerproto.ChunkVoxelsMsg{
		Type:            "CHUNK_VOXELS",
		ProtocolVersion: observerproto.Version,
		CX:              st.key.CX,
		CZ:              st.key.CZ,
		Encoding:        "PAL16_U16LE_YZX",
		Data:            encodePAL16U16LE(st.blocks),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return trySend(c.dataOut, b)
}

func encodePAL16U16LE(blocks []uint16) string {
	return obscodec.EncodePAL16U16LE(blocks)
}
