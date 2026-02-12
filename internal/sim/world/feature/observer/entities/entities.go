package entities

import (
	"sort"
	"strings"

	"voxelcraft.ai/internal/protocol"
)

type Pos struct {
	X int
	Y int
	Z int
}

func (p Pos) ToArray() [3]int { return [3]int{p.X, p.Y, p.Z} }

func IsNear(center, other Pos, dist int) bool {
	dx := center.X - other.X
	if dx < 0 {
		dx = -dx
	}
	dy := center.Y - other.Y
	if dy < 0 {
		dy = -dy
	}
	dz := center.Z - other.Z
	if dz < 0 {
		dz = -dz
	}
	return dx+dy+dz <= dist
}

type AgentInput struct {
	ID       string
	Pos      Pos
	OrgID    string
	RepTrade int
	RepLaw   int
}

func BuildAgentEntities(selfID string, selfPos Pos, agents []AgentInput, dist int) []protocol.EntityObs {
	out := make([]protocol.EntityObs, 0, len(agents))
	for _, other := range agents {
		if other.ID == selfID || !IsNear(selfPos, other.Pos, dist) {
			continue
		}
		tags := make([]string, 0, 2)
		if other.OrgID != "" {
			tags = append(tags, "org:"+other.OrgID)
		}
		if other.RepLaw > 0 && other.RepLaw < 200 {
			tags = append(tags, "wanted")
		}
		out = append(out, protocol.EntityObs{
			ID:             other.ID,
			Type:           "AGENT",
			Pos:            other.Pos.ToArray(),
			Tags:           tags,
			ReputationHint: float64(other.RepTrade) / 1000.0,
		})
	}
	return out
}

type SimpleInput struct {
	ID   string
	Type string
	Pos  Pos
}

func BuildSimpleEntities(in []SimpleInput) []protocol.EntityObs {
	out := make([]protocol.EntityObs, 0, len(in))
	for _, e := range in {
		out = append(out, protocol.EntityObs{ID: e.ID, Type: e.Type, Pos: e.Pos.ToArray()})
	}
	return out
}

type SignInput struct {
	ID   string
	Pos  Pos
	Text string
}

func BuildSignEntities(in []SignInput) []protocol.EntityObs {
	out := make([]protocol.EntityObs, 0, len(in))
	for _, s := range in {
		tags := []string{}
		if strings.TrimSpace(s.Text) != "" {
			tags = append(tags, "has_text")
		}
		out = append(out, protocol.EntityObs{ID: s.ID, Type: "SIGN", Pos: s.Pos.ToArray(), Tags: tags})
	}
	return out
}

type ConveyorInput struct {
	ID     string
	Pos    Pos
	DirTag string
}

func BuildConveyorEntities(in []ConveyorInput) []protocol.EntityObs {
	out := make([]protocol.EntityObs, 0, len(in))
	for _, c := range in {
		out = append(out, protocol.EntityObs{ID: c.ID, Type: "CONVEYOR", Pos: c.Pos.ToArray(), Tags: []string{"dir:" + c.DirTag}})
	}
	return out
}

type SwitchInput struct {
	ID  string
	Pos Pos
	On  bool
}

func BuildSwitchEntities(in []SwitchInput) []protocol.EntityObs {
	out := make([]protocol.EntityObs, 0, len(in))
	for _, s := range in {
		state := "off"
		if s.On {
			state = "on"
		}
		out = append(out, protocol.EntityObs{ID: s.ID, Type: "SWITCH", Pos: s.Pos.ToArray(), Tags: []string{"state:" + state}})
	}
	return out
}

type SensorInput struct {
	ID  string
	Pos Pos
	On  bool
}

func BuildSensorEntities(in []SensorInput) []protocol.EntityObs {
	out := make([]protocol.EntityObs, 0, len(in))
	for _, s := range in {
		state := "off"
		if s.On {
			state = "on"
		}
		out = append(out, protocol.EntityObs{ID: s.ID, Type: "SENSOR", Pos: s.Pos.ToArray(), Tags: []string{"state:" + state}})
	}
	return out
}

type ItemInput struct {
	ID    string
	Pos   Pos
	Item  string
	Count int
}

func BuildItemEntities(in []ItemInput) []protocol.EntityObs {
	sort.Slice(in, func(i, j int) bool { return in[i].ID < in[j].ID })
	out := make([]protocol.EntityObs, 0, len(in))
	for _, it := range in {
		out = append(out, protocol.EntityObs{
			ID:    it.ID,
			Type:  "ITEM",
			Pos:   it.Pos.ToArray(),
			Item:  it.Item,
			Count: it.Count,
		})
	}
	return out
}
