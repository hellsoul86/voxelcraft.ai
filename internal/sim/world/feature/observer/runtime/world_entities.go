package runtime

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	conveyruntimepkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
	entitiespkg "voxelcraft.ai/internal/sim/world/feature/observer/entities"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type BuildEntitiesFromWorldInput struct {
	SelfID  string
	SelfPos modelpkg.Vec3i

	Agents     map[string]*modelpkg.Agent
	Containers map[modelpkg.Vec3i]*modelpkg.Container
	Boards     map[string]*modelpkg.Board
	Signs      map[modelpkg.Vec3i]*modelpkg.Sign
	Conveyors  map[modelpkg.Vec3i]modelpkg.ConveyorMeta
	Switches   map[modelpkg.Vec3i]bool
	Items      map[string]*modelpkg.ItemEntity

	SensorsNear []modelpkg.Vec3i

	ParseContainerID            func(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	SortedSignPositionsNear     func(pos modelpkg.Vec3i, dist int) []modelpkg.Vec3i
	SortedConveyorPositionsNear func(pos modelpkg.Vec3i, dist int) []modelpkg.Vec3i
	SortedSwitchPositionsNear   func(pos modelpkg.Vec3i, dist int) []modelpkg.Vec3i
	SensorOn                    func(pos modelpkg.Vec3i) bool

	ContainerID  func(typ string, pos modelpkg.Vec3i) string
	SignIDAt     func(pos modelpkg.Vec3i) string
	ConveyorIDAt func(pos modelpkg.Vec3i) string
	SwitchIDAt   func(pos modelpkg.Vec3i) string

	Distance int
}

func BuildEntitiesFromWorld(in BuildEntitiesFromWorldInput) []protocol.EntityObs {
	dist := in.Distance
	if dist <= 0 {
		dist = 16
	}

	selfPos := entitiespkg.Pos{X: in.SelfPos.X, Y: in.SelfPos.Y, Z: in.SelfPos.Z}
	ents := make([]protocol.EntityObs, 0, 32)

	agentInputs := make([]entitiespkg.AgentInput, 0, len(in.Agents))
	for _, other := range in.Agents {
		if other == nil {
			continue
		}
		agentInputs = append(agentInputs, entitiespkg.AgentInput{
			ID:       other.ID,
			Pos:      entitiespkg.Pos{X: other.Pos.X, Y: other.Pos.Y, Z: other.Pos.Z},
			OrgID:    other.OrgID,
			RepTrade: other.RepTrade,
			RepLaw:   other.RepLaw,
		})
	}
	ents = append(ents, entitiespkg.BuildAgentEntities(in.SelfID, selfPos, agentInputs, dist)...)

	containers := make([]entitiespkg.SimpleInput, 0, len(in.Containers))
	for _, c := range in.Containers {
		if c == nil {
			continue
		}
		pos := entitiespkg.Pos{X: c.Pos.X, Y: c.Pos.Y, Z: c.Pos.Z}
		if !entitiespkg.IsNear(selfPos, pos, dist) {
			continue
		}
		containers = append(containers, entitiespkg.SimpleInput{
			ID:   c.ID(),
			Type: c.Type,
			Pos:  pos,
		})
	}
	ents = append(ents, entitiespkg.BuildSimpleEntities(containers)...)

	if len(in.Boards) > 0 && in.ParseContainerID != nil {
		boardEntries := make([]entitiespkg.SimpleInput, 0, len(in.Boards))
		for id := range in.Boards {
			typ, pos, ok := in.ParseContainerID(id)
			if !ok || typ != "BULLETIN_BOARD" {
				continue
			}
			bpos := entitiespkg.Pos{X: pos.X, Y: pos.Y, Z: pos.Z}
			if !entitiespkg.IsNear(selfPos, bpos, dist) {
				continue
			}
			boardEntries = append(boardEntries, entitiespkg.SimpleInput{
				ID:   id,
				Type: "BULLETIN_BOARD",
				Pos:  bpos,
			})
		}
		sort.Slice(boardEntries, func(i, j int) bool { return boardEntries[i].ID < boardEntries[j].ID })
		ents = append(ents, entitiespkg.BuildSimpleEntities(boardEntries)...)
	}

	if len(in.Signs) > 0 && in.SortedSignPositionsNear != nil && in.SignIDAt != nil {
		signs := make([]entitiespkg.SignInput, 0, len(in.Signs))
		for _, p := range in.SortedSignPositionsNear(in.SelfPos, dist) {
			s := in.Signs[p]
			text := ""
			if s != nil {
				text = s.Text
			}
			signs = append(signs, entitiespkg.SignInput{
				ID:   in.SignIDAt(p),
				Pos:  entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				Text: text,
			})
		}
		ents = append(ents, entitiespkg.BuildSignEntities(signs)...)
	}

	if len(in.Conveyors) > 0 && in.SortedConveyorPositionsNear != nil && in.ConveyorIDAt != nil {
		conveyors := make([]entitiespkg.ConveyorInput, 0, len(in.Conveyors))
		for _, p := range in.SortedConveyorPositionsNear(in.SelfPos, dist) {
			m := in.Conveyors[p]
			conveyors = append(conveyors, entitiespkg.ConveyorInput{
				ID:     in.ConveyorIDAt(p),
				Pos:    entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				DirTag: conveyruntimepkg.DirectionTag(int(m.DX), int(m.DZ)),
			})
		}
		ents = append(ents, entitiespkg.BuildConveyorEntities(conveyors)...)
	}

	if len(in.Switches) > 0 && in.SortedSwitchPositionsNear != nil && in.SwitchIDAt != nil {
		switches := make([]entitiespkg.SwitchInput, 0, len(in.Switches))
		for _, p := range in.SortedSwitchPositionsNear(in.SelfPos, dist) {
			switches = append(switches, entitiespkg.SwitchInput{
				ID:  in.SwitchIDAt(p),
				Pos: entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				On:  in.Switches[p],
			})
		}
		ents = append(ents, entitiespkg.BuildSwitchEntities(switches)...)
	}

	if len(in.SensorsNear) > 0 && in.ContainerID != nil && in.SensorOn != nil {
		sensorsNear := append([]modelpkg.Vec3i(nil), in.SensorsNear...)
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
				ID:  in.ContainerID("SENSOR", p),
				Pos: entitiespkg.Pos{X: p.X, Y: p.Y, Z: p.Z},
				On:  in.SensorOn(p),
			})
		}
		ents = append(ents, entitiespkg.BuildSensorEntities(sensors)...)
	}

	if len(in.Items) > 0 {
		items := make([]entitiespkg.ItemInput, 0, len(in.Items))
		for _, e := range in.Items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			pos := entitiespkg.Pos{X: e.Pos.X, Y: e.Pos.Y, Z: e.Pos.Z}
			if !entitiespkg.IsNear(selfPos, pos, dist) {
				continue
			}
			items = append(items, entitiespkg.ItemInput{
				ID:    e.EntityID,
				Pos:   pos,
				Item:  e.Item,
				Count: e.Count,
			})
		}
		ents = append(ents, entitiespkg.BuildItemEntities(items)...)
	}

	return ents
}
