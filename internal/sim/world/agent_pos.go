package world

import (
	"context"
	"errors"
)

type agentPosReq struct {
	AgentID string
	Resp    chan agentPosResp
}

type agentPosResp struct {
	Pos Vec3i
	Err string
}

// RequestAgentPos returns the current position for an agent from the world loop goroutine.
func (w *World) RequestAgentPos(ctx context.Context, agentID string) (Vec3i, error) {
	if w == nil || w.agentPosReq == nil {
		return Vec3i{}, errors.New("agent position query not available")
	}
	req := agentPosReq{
		AgentID: agentID,
		Resp:    make(chan agentPosResp, 1),
	}
	select {
	case w.agentPosReq <- req:
	case <-ctx.Done():
		return Vec3i{}, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return Vec3i{}, errors.New(resp.Err)
		}
		return resp.Pos, nil
	case <-ctx.Done():
		return Vec3i{}, ctx.Err()
	}
}

func (w *World) handleAgentPosReq(req agentPosReq) {
	resp := agentPosResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()
	a := w.agents[req.AgentID]
	if a == nil {
		resp.Err = "agent not found"
		return
	}
	resp.Pos = a.Pos
}
