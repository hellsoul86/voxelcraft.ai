package world

import (
	"context"
	"errors"

	transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"
)

func (w *World) RequestAgentPos(ctx context.Context, agentID string) (Vec3i, error) {
	if w == nil {
		return Vec3i{}, errors.New("agent position query not available")
	}
	pos, err := transferruntimepkg.RequestAgentPos(ctx, w.agentPosReq, agentID)
	if err != nil {
		return Vec3i{}, err
	}
	return Vec3i{X: pos[0], Y: pos[1], Z: pos[2]}, nil
}

func (w *World) handleAgentPosReq(req transferruntimepkg.AgentPosReq) {
	resp := transferruntimepkg.HandleAgentPosReq(req, func(agentID string) ([3]int, bool) {
		a := w.agents[agentID]
		if a == nil {
			return [3]int{}, false
		}
		return [3]int{a.Pos.X, a.Pos.Y, a.Pos.Z}, true
	})
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}
