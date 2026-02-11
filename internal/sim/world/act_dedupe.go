package world

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/protocol"
)

const actDedupeTTLTicks = uint64(3000)

type actDedupeKey struct {
	AgentID string
	WorldID string
	ActID   string
}

type actDedupeEntry struct {
	Ack         protocol.AckMsg
	ExpiresTick uint64
}

type actDedupeReq struct {
	AgentID  string
	WorldID  string
	ActID    string
	Proposed protocol.AckMsg
	Resp     chan actDedupeResp
}

type actDedupeResp struct {
	Ack       protocol.AckMsg
	Duplicate bool
	Err       string
}

// RequestCheckOrRememberActAck returns an existing ACK for (agent, world, act_id),
// or stores and returns the proposed one if unseen.
func (w *World) RequestCheckOrRememberActAck(ctx context.Context, agentID, worldID, actID string, proposed protocol.AckMsg) (protocol.AckMsg, bool, error) {
	if w == nil || w.actDedupeReq == nil {
		return protocol.AckMsg{}, false, errors.New("act dedupe not available")
	}
	req := actDedupeReq{
		AgentID:  agentID,
		WorldID:  worldID,
		ActID:    actID,
		Proposed: proposed,
		Resp:     make(chan actDedupeResp, 1),
	}
	select {
	case w.actDedupeReq <- req:
	case <-ctx.Done():
		return protocol.AckMsg{}, false, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return protocol.AckMsg{}, false, errors.New(resp.Err)
		}
		return resp.Ack, resp.Duplicate, nil
	case <-ctx.Done():
		return protocol.AckMsg{}, false, ctx.Err()
	}
}

func (w *World) handleActDedupeReq(req actDedupeReq) {
	resp := actDedupeResp{Ack: req.Proposed}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()
	if req.ActID == "" {
		return
	}
	now := w.tick.Load()
	if req.WorldID == "" {
		req.WorldID = w.cfg.ID
	}
	key := actDedupeKey{AgentID: req.AgentID, WorldID: req.WorldID, ActID: req.ActID}
	if w.actDedupe == nil {
		w.actDedupe = map[actDedupeKey]actDedupeEntry{}
	}
	// Opportunistic cleanup.
	for k, v := range w.actDedupe {
		if now >= v.ExpiresTick {
			delete(w.actDedupe, k)
		}
	}
	if entry, ok := w.actDedupe[key]; ok && now < entry.ExpiresTick {
		resp.Ack = entry.Ack
		resp.Duplicate = true
		return
	}
	w.actDedupe[key] = actDedupeEntry{
		Ack:         req.Proposed,
		ExpiresTick: now + actDedupeTTLTicks,
	}
}
