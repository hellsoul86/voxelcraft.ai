package world

import (
	"sort"

	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	conveyruntimepkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	entitiespkg "voxelcraft.ai/internal/sim/world/feature/observer/entities"
	metapkg "voxelcraft.ai/internal/sim/world/feature/observer/meta"
	observerruntimepkg "voxelcraft.ai/internal/sim/world/feature/observer/runtime"
	streamspkg "voxelcraft.ai/internal/sim/world/feature/observer/stream"
	taskspkg "voxelcraft.ai/internal/sim/world/feature/observer/tasks"
	progresspkg "voxelcraft.ai/internal/sim/world/feature/work/progress"
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

	publicBoards := observerruntimepkg.BuildPublicBoardsFromWorld(observerruntimepkg.PublicBoardsFromWorldInput{
		Boards: w.boards,
		Self:   a.Pos,
		ParseContainerID: func(id string) (string, Vec3i, bool) {
			return parseContainerID(id)
		},
		Distance:      Manhattan,
		MaxDistance:   32,
		MaxPosts:      5,
		MaxSummaryLen: 120,
	})

	landID := ""
	owner := ""
	marketTax := 0.0
	maintenanceDue := uint64(0)
	maintenanceStage := 0
	isOwner := false
	isMember := false
	if land != nil {
		landID = land.LandID
		owner = land.Owner
		marketTax = land.MarketTax
		maintenanceDue = land.MaintenanceDueTick
		maintenanceStage = land.MaintenanceStage
		isOwner = land.Owner == a.ID
		isMember = w.isLandMember(a.ID, land)
	}
	localRules := observerruntimepkg.BuildLocalRules(observerruntimepkg.LocalRulesInput{
		Permissions:        perms,
		HasLand:            land != nil,
		LandID:             landID,
		Owner:              owner,
		IsOwner:            isOwner,
		IsMember:           isMember,
		MarketTax:          marketTax,
		MaintenanceDueTick: maintenanceDue,
		MaintenanceStage:   maintenanceStage,
	})

	status := observerruntimepkg.BuildStatus(a.Hunger, a.StaminaMilli, w.weather)

	obs := observerruntimepkg.ComposeObs(observerruntimepkg.ComposeObsInput{
		Tick:    nowTick,
		AgentID: a.ID,
		WorldID: w.cfg.ID,
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
	})
	metapkg.AttachObsEventsAndMeta(a, &obs, nowTick)
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
				DirTag: conveyruntimepkg.DirectionTag(int(m.DX), int(m.DZ)),
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

func (w *World) buildObsVoxels(center Vec3i, cl *clientState) (protocol.VoxelsObs, []Vec3i) {
	r := w.cfg.ObsRadius
	sensorBlock, hasSensor := w.catalogs.Blocks.Index["SENSOR"]
	out := streamspkg.BuildObsVoxels2D(streamspkg.VoxelsBuildInput{
		Center: streamspkg.VoxelPos{X: center.X, Y: center.Y, Z: center.Z},
		Radius: r,

		AirBlock:    w.chunks.gen.Air,
		HasSensor:   hasSensor,
		SensorBlock: sensorBlock,

		DeltaEnabled: cl.DeltaVoxels,
		LastVoxels:   cl.LastVoxels,
	}, func(pos streamspkg.VoxelPos) uint16 {
		return w.chunks.GetBlock(Vec3i{X: pos.X, Y: pos.Y, Z: pos.Z})
	})
	cl.LastVoxels = out.Current

	sensorsNear := make([]Vec3i, 0, len(out.SensorPos))
	for _, p := range out.SensorPos {
		sensorsNear = append(sensorsNear, Vec3i{X: p.X, Y: p.Y, Z: p.Z})
	}
	return out.Voxels, sensorsNear
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
