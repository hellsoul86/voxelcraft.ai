package world

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
)

func (w *World) buildObsTasks(a *Agent, nowTick uint64) []protocol.TaskObs {
	tasksObs := make([]protocol.TaskObs, 0, 2)
	if a.MoveTask != nil {
		mt := a.MoveTask
		target := v3FromTask(mt.Target)
		if mt.Kind == tasks.KindFollow {
			if t, ok := w.followTargetPos(mt.TargetID); ok {
				target = t
			}
			want := int(math.Ceil(mt.Distance))
			if want < 1 {
				want = 1
			}
			d := distXZ(a.Pos, target)
			prog := 0.0
			if d <= want {
				prog = 1.0
			}
			eta := d - want
			if eta < 0 {
				eta = 0
			}
			tasksObs = append(tasksObs, protocol.TaskObs{
				TaskID:   mt.TaskID,
				Kind:     string(mt.Kind),
				Progress: prog,
				Target:   target.ToArray(),
				EtaTicks: eta,
			})
		} else {
			start := v3FromTask(mt.StartPos)
			want := int(math.Ceil(mt.Tolerance))
			if want < 1 {
				want = 1
			}
			distStart := distXZ(start, target)
			distCur := distXZ(a.Pos, target)
			totalEff := distStart - want
			if totalEff < 0 {
				totalEff = 0
			}
			remEff := distCur - want
			if remEff < 0 {
				remEff = 0
			}
			prog := 1.0
			if totalEff > 0 {
				prog = float64(totalEff-remEff) / float64(totalEff)
				if prog < 0 {
					prog = 0
				} else if prog > 1 {
					prog = 1
				}
			}
			eta := remEff
			tasksObs = append(tasksObs, protocol.TaskObs{
				TaskID:   mt.TaskID,
				Kind:     string(mt.Kind),
				Progress: prog,
				Target:   target.ToArray(),
				EtaTicks: eta,
			})
		}
	}
	if a.WorkTask != nil {
		tasksObs = append(tasksObs, protocol.TaskObs{
			TaskID:   a.WorkTask.TaskID,
			Kind:     string(a.WorkTask.Kind),
			Progress: w.workProgressForAgent(a, a.WorkTask),
		})
	}
	return tasksObs
}

func (w *World) buildObsEntities(a *Agent, sensorsNear []Vec3i) []protocol.EntityObs {
	ents := make([]protocol.EntityObs, 0, 16)
	for _, other := range w.agents {
		if other.ID == a.ID {
			continue
		}
		if Manhattan(other.Pos, a.Pos) <= 16 {
			tags := []string{}
			if other.OrgID != "" {
				tags = append(tags, "org:"+other.OrgID)
			}
			if other.RepLaw > 0 && other.RepLaw < 200 {
				tags = append(tags, "wanted")
			}
			ents = append(ents, protocol.EntityObs{
				ID:             other.ID,
				Type:           "AGENT",
				Pos:            other.Pos.ToArray(),
				Tags:           tags,
				ReputationHint: float64(other.RepTrade) / 1000.0,
			})
		}
	}
	for _, c := range w.containers {
		if Manhattan(c.Pos, a.Pos) <= 16 {
			ents = append(ents, protocol.EntityObs{ID: c.ID(), Type: c.Type, Pos: c.Pos.ToArray()})
		}
	}
	if len(w.boards) > 0 {
		boardIDs := make([]string, 0, len(w.boards))
		for id := range w.boards {
			typ, pos, ok := parseContainerID(id)
			if !ok || typ != "BULLETIN_BOARD" {
				continue
			}
			if Manhattan(pos, a.Pos) > 16 {
				continue
			}
			boardIDs = append(boardIDs, id)
		}
		sort.Strings(boardIDs)
		for _, id := range boardIDs {
			typ, pos, ok := parseContainerID(id)
			if !ok || typ != "BULLETIN_BOARD" {
				continue
			}
			ents = append(ents, protocol.EntityObs{ID: id, Type: "BULLETIN_BOARD", Pos: pos.ToArray()})
		}
	}
	if len(w.signs) > 0 {
		for _, p := range w.sortedSignPositionsNear(a.Pos, 16) {
			s := w.signs[p]
			tags := []string{}
			if s != nil && strings.TrimSpace(s.Text) != "" {
				tags = append(tags, "has_text")
			}
			ents = append(ents, protocol.EntityObs{ID: signIDAt(p), Type: "SIGN", Pos: p.ToArray(), Tags: tags})
		}
	}
	if len(w.conveyors) > 0 {
		for _, p := range w.sortedConveyorPositionsNear(a.Pos, 16) {
			m := w.conveyors[p]
			tags := []string{"dir:" + conveyorDirTag(m)}
			ents = append(ents, protocol.EntityObs{ID: conveyorIDAt(p), Type: "CONVEYOR", Pos: p.ToArray(), Tags: tags})
		}
	}
	if len(w.switches) > 0 {
		for _, p := range w.sortedSwitchPositionsNear(a.Pos, 16) {
			state := "off"
			if w.switches[p] {
				state = "on"
			}
			ents = append(ents, protocol.EntityObs{ID: switchIDAt(p), Type: "SWITCH", Pos: p.ToArray(), Tags: []string{"state:" + state}})
		}
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
		for _, p := range sensorsNear {
			state := "off"
			if w.sensorOn(p) {
				state = "on"
			}
			ents = append(ents, protocol.EntityObs{ID: containerID("SENSOR", p), Type: "SENSOR", Pos: p.ToArray(), Tags: []string{"state:" + state}})
		}
	}
	if len(w.items) > 0 {
		itemIDs := make([]string, 0, len(w.items))
		for id, e := range w.items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			if Manhattan(e.Pos, a.Pos) > 16 {
				continue
			}
			itemIDs = append(itemIDs, id)
		}
		sort.Strings(itemIDs)
		for _, id := range itemIDs {
			e := w.items[id]
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			ents = append(ents, protocol.EntityObs{
				ID:    e.EntityID,
				Type:  "ITEM",
				Pos:   e.Pos.ToArray(),
				Item:  e.Item,
				Count: e.Count,
			})
		}
	}
	return ents
}

func (w *World) attachObsEventsAndMeta(a *Agent, obs *protocol.ObsMsg, nowTick uint64) {
	ev := a.TakeEvents()
	if len(ev) > 0 {
		a.EventCursor += uint64(len(ev))
	}
	obs.Events = ev
	obs.EventsCursor = a.EventCursor
	obs.ObsID = fmt.Sprintf("%s:%d:%d", a.ID, nowTick, a.EventCursor)

	if a.Fun.Novelty != 0 || a.Fun.Creation != 0 || a.Fun.Social != 0 || a.Fun.Influence != 0 || a.Fun.Narrative != 0 || a.Fun.RiskRescue != 0 {
		obs.FunScore = &protocol.FunScoreObs{
			Novelty:    a.Fun.Novelty,
			Creation:   a.Fun.Creation,
			Social:     a.Fun.Social,
			Influence:  a.Fun.Influence,
			Narrative:  a.Fun.Narrative,
			RiskRescue: a.Fun.RiskRescue,
		}
	}

	if len(a.PendingMemory) > 0 {
		obs.Memory = a.PendingMemory
		a.PendingMemory = nil
	}
}
