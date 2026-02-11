package world

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/protocol"
)

type injectEventReq struct {
	AgentID string
	Event   protocol.Event
}

func (w *World) RequestInjectEvent(ctx context.Context, agentID string, ev protocol.Event) error {
	if w == nil || w.injectEvent == nil {
		return errors.New("inject event not available")
	}
	req := injectEventReq{AgentID: agentID, Event: ev}
	select {
	case w.injectEvent <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
