package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	entitiespkg "voxelcraft.ai/internal/sim/world/feature/observer/entities"
	metapkg "voxelcraft.ai/internal/sim/world/feature/observer/meta"
	taskspkg "voxelcraft.ai/internal/sim/world/feature/observer/tasks"
)

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
