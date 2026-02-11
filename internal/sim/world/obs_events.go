package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	featureobserver "voxelcraft.ai/internal/sim/world/feature/observer"
)

func (w *World) buildObsTasks(a *Agent, nowTick uint64) []protocol.TaskObs {
	var moveIn *featureobserver.MoveTaskInput
	if a.MoveTask != nil {
		moveIn = &featureobserver.MoveTaskInput{
			TaskID:    a.MoveTask.TaskID,
			Kind:      string(a.MoveTask.Kind),
			Target:    featureobserver.TaskVec3{X: a.MoveTask.Target.X, Y: a.MoveTask.Target.Y, Z: a.MoveTask.Target.Z},
			StartPos:  featureobserver.TaskVec3{X: a.MoveTask.StartPos.X, Y: a.MoveTask.StartPos.Y, Z: a.MoveTask.StartPos.Z},
			TargetID:  a.MoveTask.TargetID,
			Distance:  a.MoveTask.Distance,
			Tolerance: a.MoveTask.Tolerance,
		}
	}
	var workIn *featureobserver.WorkTaskInput
	if a.WorkTask != nil {
		workIn = &featureobserver.WorkTaskInput{
			TaskID:   a.WorkTask.TaskID,
			Kind:     string(a.WorkTask.Kind),
			Progress: w.workProgressForAgent(a, a.WorkTask),
		}
	}
	return featureobserver.BuildTasks(featureobserver.BuildTasksInput{
		SelfPos: featureobserver.TaskVec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		Move:    moveIn,
		Work:    workIn,
	}, func(id string) (featureobserver.TaskVec3, bool) {
		if t, ok := w.followTargetPos(id); ok {
			return featureobserver.TaskVec3{X: t.X, Y: t.Y, Z: t.Z}, true
		}
		return featureobserver.TaskVec3{}, false
	})
}

func (w *World) buildObsEntities(a *Agent, sensorsNear []Vec3i) []protocol.EntityObs {
	ents := make([]protocol.EntityObs, 0, 32)
	selfPos := featureobserver.EntityPos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}

	agentInputs := make([]featureobserver.AgentEntityInput, 0, len(w.agents))
	for _, other := range w.agents {
		agentInputs = append(agentInputs, featureobserver.AgentEntityInput{
			ID:       other.ID,
			Pos:      featureobserver.EntityPos{X: other.Pos.X, Y: other.Pos.Y, Z: other.Pos.Z},
			OrgID:    other.OrgID,
			RepTrade: other.RepTrade,
			RepLaw:   other.RepLaw,
		})
	}
	ents = append(ents, featureobserver.BuildAgentEntities(a.ID, selfPos, agentInputs, 16)...)

	containers := make([]featureobserver.SimpleEntityInput, 0, len(w.containers))
	for _, c := range w.containers {
		if !featureobserver.IsNear(selfPos, featureobserver.EntityPos{X: c.Pos.X, Y: c.Pos.Y, Z: c.Pos.Z}, 16) {
			continue
		}
		containers = append(containers, featureobserver.SimpleEntityInput{
			ID:   c.ID(),
			Type: c.Type,
			Pos:  featureobserver.EntityPos{X: c.Pos.X, Y: c.Pos.Y, Z: c.Pos.Z},
		})
	}
	ents = append(ents, featureobserver.BuildSimpleEntities(containers)...)

	if len(w.boards) > 0 {
		boardEntries := make([]featureobserver.SimpleEntityInput, 0, len(w.boards))
		for id := range w.boards {
			typ, pos, ok := parseContainerID(id)
			if !ok || typ != "BULLETIN_BOARD" {
				continue
			}
			if !featureobserver.IsNear(selfPos, featureobserver.EntityPos{X: pos.X, Y: pos.Y, Z: pos.Z}, 16) {
				continue
			}
			boardEntries = append(boardEntries, featureobserver.SimpleEntityInput{
				ID:   id,
				Type: "BULLETIN_BOARD",
				Pos:  featureobserver.EntityPos{X: pos.X, Y: pos.Y, Z: pos.Z},
			})
		}
		sort.Slice(boardEntries, func(i, j int) bool { return boardEntries[i].ID < boardEntries[j].ID })
		ents = append(ents, featureobserver.BuildSimpleEntities(boardEntries)...)
	}
	if len(w.signs) > 0 {
		signs := make([]featureobserver.SignEntityInput, 0, len(w.signs))
		for _, p := range w.sortedSignPositionsNear(a.Pos, 16) {
			s := w.signs[p]
			text := ""
			if s != nil {
				text = s.Text
			}
			signs = append(signs, featureobserver.SignEntityInput{
				ID:   signIDAt(p),
				Pos:  featureobserver.EntityPos{X: p.X, Y: p.Y, Z: p.Z},
				Text: text,
			})
		}
		ents = append(ents, featureobserver.BuildSignEntities(signs)...)
	}
	if len(w.conveyors) > 0 {
		conveyors := make([]featureobserver.ConveyorEntityInput, 0, len(w.conveyors))
		for _, p := range w.sortedConveyorPositionsNear(a.Pos, 16) {
			m := w.conveyors[p]
			conveyors = append(conveyors, featureobserver.ConveyorEntityInput{
				ID:     conveyorIDAt(p),
				Pos:    featureobserver.EntityPos{X: p.X, Y: p.Y, Z: p.Z},
				DirTag: conveyorDirTag(m),
			})
		}
		ents = append(ents, featureobserver.BuildConveyorEntities(conveyors)...)
	}
	if len(w.switches) > 0 {
		switches := make([]featureobserver.SwitchEntityInput, 0, len(w.switches))
		for _, p := range w.sortedSwitchPositionsNear(a.Pos, 16) {
			switches = append(switches, featureobserver.SwitchEntityInput{
				ID:  switchIDAt(p),
				Pos: featureobserver.EntityPos{X: p.X, Y: p.Y, Z: p.Z},
				On:  w.switches[p],
			})
		}
		ents = append(ents, featureobserver.BuildSwitchEntities(switches)...)
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
		sensors := make([]featureobserver.SensorEntityInput, 0, len(sensorsNear))
		for _, p := range sensorsNear {
			sensors = append(sensors, featureobserver.SensorEntityInput{
				ID:  containerID("SENSOR", p),
				Pos: featureobserver.EntityPos{X: p.X, Y: p.Y, Z: p.Z},
				On:  w.sensorOn(p),
			})
		}
		ents = append(ents, featureobserver.BuildSensorEntities(sensors)...)
	}
	if len(w.items) > 0 {
		items := make([]featureobserver.ItemEntityInput, 0, len(w.items))
		for _, e := range w.items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			if !featureobserver.IsNear(selfPos, featureobserver.EntityPos{X: e.Pos.X, Y: e.Pos.Y, Z: e.Pos.Z}, 16) {
				continue
			}
			items = append(items, featureobserver.ItemEntityInput{
				ID:    e.EntityID,
				Pos:   featureobserver.EntityPos{X: e.Pos.X, Y: e.Pos.Y, Z: e.Pos.Z},
				Item:  e.Item,
				Count: e.Count,
			})
		}
		ents = append(ents, featureobserver.BuildItemEntities(items)...)
	}
	return ents
}

func (w *World) attachObsEventsAndMeta(a *Agent, obs *protocol.ObsMsg, nowTick uint64) {
	ev := a.TakeEvents()
	obs.Events = ev
	obs.EventsCursor = a.EventCursor
	obs.ObsID = featureobserver.ObsID(a.ID, nowTick, a.EventCursor)
	obs.FunScore = featureobserver.FunScorePtr(
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
