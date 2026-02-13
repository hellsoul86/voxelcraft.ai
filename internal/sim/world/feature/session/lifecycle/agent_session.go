package lifecycle

import (
	"strings"

	resumepkg "voxelcraft.ai/internal/sim/world/feature/session/resume"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type BuildJoinedAgentInput struct {
	AgentID      string
	Name         string
	WorldID      string
	Spawn        modelpkg.Vec3i
	StarterItems map[string]int
}

func BuildJoinedAgent(in BuildJoinedAgentInput) *modelpkg.Agent {
	a := &modelpkg.Agent{
		ID:             in.AgentID,
		Name:           strings.TrimSpace(in.Name),
		Pos:            modelpkg.Vec3i{X: in.Spawn.X, Y: 0, Z: in.Spawn.Z},
		Yaw:            0,
		CurrentWorldID: in.WorldID,
	}
	a.InitDefaults()
	if in.StarterItems != nil {
		keys := resumepkg.SortedIDs(in.StarterItems)
		for _, item := range keys {
			n := in.StarterItems[item]
			if item == "" || n <= 0 {
				continue
			}
			a.Inventory[item] += n
		}
	}
	return a
}

type AttachResult struct {
	Agent    *modelpkg.Agent
	NewToken string
	OK       bool
}

func AttachByToken(token string, agents map[string]*modelpkg.Agent, worldID string, nowUnixNano int64) AttachResult {
	if strings.TrimSpace(token) == "" || len(agents) == 0 {
		return AttachResult{}
	}
	candidates := make([]resumepkg.Candidate, 0, len(agents))
	for id, a := range agents {
		if a == nil {
			continue
		}
		candidates = append(candidates, resumepkg.Candidate{ID: id, ResumeToken: a.ResumeToken})
	}
	aid := resumepkg.FindResumeAgentID(candidates, strings.TrimSpace(token))
	a := agents[aid]
	if a == nil {
		return AttachResult{}
	}
	a.CurrentWorldID = worldID
	newToken := NewResumeToken(worldID, nowUnixNano)
	a.ResumeToken = newToken
	return AttachResult{
		Agent:    a,
		NewToken: newToken,
		OK:       true,
	}
}
