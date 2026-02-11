package world

import (
	"context"
	"fmt"
	"time"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
)

func (w *World) Run(ctx context.Context) error {
	interval := time.Second / time.Duration(w.cfg.TickRateHz)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var pendingActions []ActionEnvelope
	var pendingJoins []JoinRequest
	var pendingLeaves []string
	var pendingAdmin []adminSnapshotReq
	var pendingAdminReset []adminResetReq
	var pendingTransferOut []transferOutReq
	var pendingTransferIn []transferInReq
	var pendingInjectEvents []injectEventReq

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stop:
			return nil
		case req := <-w.join:
			pendingJoins = append(pendingJoins, req)
		case req := <-w.attach:
			w.handleAttach(req)
		case id := <-w.leave:
			pendingLeaves = append(pendingLeaves, id)
		case req := <-w.observerJoin:
			w.handleObserverJoin(req)
		case req := <-w.observerSub:
			w.handleObserverSubscribe(req)
		case id := <-w.observerLeave:
			w.handleObserverLeave(id)
		case req := <-w.admin:
			pendingAdmin = append(pendingAdmin, req)
		case req := <-w.adminReset:
			pendingAdminReset = append(pendingAdminReset, req)
		case req := <-w.agentPosReq:
			w.handleAgentPosReq(req)
		case req := <-w.eventsReq:
			w.handleEventsReq(req)
		case req := <-w.actDedupeReq:
			w.handleActDedupeReq(req)
		case req := <-w.orgMetaReq:
			w.handleOrgMetaReq(req)
		case req := <-w.orgMetaUpsert:
			w.handleOrgMetaUpsertReq(req)
		case req := <-w.transferOut:
			pendingTransferOut = append(pendingTransferOut, req)
		case req := <-w.transferIn:
			pendingTransferIn = append(pendingTransferIn, req)
		case req := <-w.injectEvent:
			pendingInjectEvents = append(pendingInjectEvents, req)
		case env := <-w.inbox:
			pendingActions = append(pendingActions, env)
		case <-ticker.C:
			w.stepInternal(pendingJoins, pendingLeaves, pendingActions, pendingTransferOut, pendingTransferIn, pendingInjectEvents)
			w.handleAdminSnapshotRequests(pendingAdmin)
			w.handleAdminResetRequests(pendingAdminReset)
			pendingJoins = pendingJoins[:0]
			pendingLeaves = pendingLeaves[:0]
			pendingActions = pendingActions[:0]
			pendingAdmin = pendingAdmin[:0]
			pendingAdminReset = pendingAdminReset[:0]
			pendingTransferOut = pendingTransferOut[:0]
			pendingTransferIn = pendingTransferIn[:0]
			pendingInjectEvents = pendingInjectEvents[:0]
		}
	}
}

func (w *World) Stop() { close(w.stop) }

func (w *World) handleLeave(agentID string) {
	delete(w.clients, agentID)
}

func (w *World) step(joins []JoinRequest, leaves []string, actions []ActionEnvelope) {
	w.stepInternal(joins, leaves, actions, nil, nil, nil)
}

// StepOnce advances the world by a single tick using the same ordering semantics as the server.
// It is primarily intended for deterministic replays/tests.
func (w *World) StepOnce(joins []JoinRequest, leaves []string, actions []ActionEnvelope) (tick uint64, digest string) {
	tick = w.tick.Load()
	w.step(joins, leaves, actions)
	return tick, w.stateDigest(tick)
}

func (w *World) ID() string {
	if w == nil {
		return ""
	}
	return w.cfg.ID
}

func (w *World) TickRateHz() int {
	if w == nil {
		return 0
	}
	return w.cfg.TickRateHz
}

func (w *World) followTargetPos(targetID string) (Vec3i, bool) {
	if targetID == "" {
		return Vec3i{}, false
	}
	if a := w.agents[targetID]; a != nil {
		return a.Pos, true
	}
	if c := w.getContainerByID(targetID); c != nil {
		return c.Pos, true
	}
	return Vec3i{}, false
}

func (w *World) newTaskID() string {
	n := w.nextTaskNum.Add(1)
	return fmt.Sprintf("T%06d", n)
}

func sendLatest(ch chan []byte, b []byte) {
	select {
	case ch <- b:
		return
	default:
	}
	// Drop one.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- b:
	default:
	}
}

func (w *World) broadcastChat(tick uint64, from *Agent, channel string, text string) {
	for _, a := range w.agents {
		switch channel {
		case "LOCAL":
			if Manhattan(a.Pos, from.Pos) > 32 {
				continue
			}
		case "CITY":
			if from.OrgID == "" || !w.isOrgMember(a.ID, from.OrgID) {
				continue
			}
		}
		a.AddEvent(protocol.Event{
			"t":       tick,
			"type":    "CHAT",
			"from":    from.ID,
			"channel": channel,
			"text":    text,
		})
	}
}

func v3FromTask(v tasks.Vec3i) Vec3i {
	return Vec3i{X: v.X, Y: v.Y, Z: v.Z}
}

func v3ToTask(v Vec3i) tasks.Vec3i {
	return tasks.Vec3i{X: v.X, Y: v.Y, Z: v.Z}
}
