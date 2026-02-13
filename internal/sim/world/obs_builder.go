package world

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	metapkg "voxelcraft.ai/internal/sim/world/feature/observer/meta"
	observerruntimepkg "voxelcraft.ai/internal/sim/world/feature/observer/runtime"
	streamspkg "voxelcraft.ai/internal/sim/world/feature/observer/stream"
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
	return observerruntimepkg.BuildTasksFromWorld(observerruntimepkg.BuildTasksFromWorldInput{
		Agent:    a,
		Progress: w.workProgressForAgent(a, a.WorkTask),
	}, func(id string) (Vec3i, bool) {
		return w.followTargetPos(id)
	})
}

func (w *World) buildObsEntities(a *Agent, sensorsNear []Vec3i) []protocol.EntityObs {
	return observerruntimepkg.BuildEntitiesFromWorld(observerruntimepkg.BuildEntitiesFromWorldInput{
		SelfID:                      a.ID,
		SelfPos:                     a.Pos,
		Agents:                      w.agents,
		Containers:                  w.containers,
		Boards:                      w.boards,
		Signs:                       w.signs,
		Conveyors:                   w.conveyors,
		Switches:                    w.switches,
		Items:                       w.items,
		SensorsNear:                 sensorsNear,
		ParseContainerID:            parseContainerID,
		SortedSignPositionsNear:     w.sortedSignPositionsNear,
		SortedConveyorPositionsNear: w.sortedConveyorPositionsNear,
		SortedSwitchPositionsNear:   w.sortedSwitchPositionsNear,
		SensorOn:                    w.sensorOn,
		ContainerID:                 containerID,
		SignIDAt:                    signIDAt,
		ConveyorIDAt:                conveyorIDAt,
		SwitchIDAt:                  switchIDAt,
		Distance:                    16,
	})
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
