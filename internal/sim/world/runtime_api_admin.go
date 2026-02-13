package world

import (
	"context"
	"errors"

	adminhandlerspkg "voxelcraft.ai/internal/sim/world/feature/admin/handlers"
	adminrequestspkg "voxelcraft.ai/internal/sim/world/feature/admin/requests"
)

type adminSnapshotReq = adminrequestspkg.SnapshotReq
type adminSnapshotResp = adminrequestspkg.SnapshotResp

type adminResetReq = adminrequestspkg.ResetReq
type adminResetResp = adminrequestspkg.ResetResp

// RequestSnapshot asks the world loop goroutine to enqueue a snapshot.
// It is safe to call from other goroutines (e.g. HTTP handlers).
func (w *World) RequestSnapshot(ctx context.Context) (tick uint64, err error) {
	if w == nil {
		return 0, errors.New("admin snapshot not available")
	}
	return adminrequestspkg.RequestSnapshot(ctx, w.admin)
}

func (w *World) handleAdminSnapshotRequests(reqs []adminSnapshotReq) {
	if w == nil || len(reqs) == 0 {
		return
	}
	result := adminhandlerspkg.HandleSnapshot(adminhandlerspkg.SnapshotInput{
		CurrentTick: w.tick.Load(),
		HasSink:     w.snapshotSink != nil,
		Enqueue: func(snapshotTick uint64) bool {
			snap := w.ExportSnapshot(snapshotTick)
			select {
			case w.snapshotSink <- snap:
				return true
			default:
				return false
			}
		},
	})
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- adminSnapshotResp{Tick: result.Tick, Err: result.Err}:
		default:
			// Client timed out; don't block the sim loop.
		}
	}
}

// RequestReset asks the world loop goroutine to perform an immediate world reset.
// It is safe to call from other goroutines (e.g. admin HTTP handlers).
func (w *World) RequestReset(ctx context.Context) (tick uint64, err error) {
	if w == nil {
		return 0, errors.New("admin reset not available")
	}
	return adminrequestspkg.RequestReset(ctx, w.adminReset)
}

func (w *World) handleAdminResetRequests(reqs []adminResetReq) {
	if w == nil || len(reqs) == 0 {
		return
	}
	result := adminhandlerspkg.HandleReset(adminhandlerspkg.ResetInput{
		CurrentTick: w.tick.Load(),
		HasSink:     w.snapshotSink != nil,
		Enqueue: func(archiveTick uint64) bool {
			snap := w.ExportSnapshot(archiveTick)
			select {
			case w.snapshotSink <- snap:
				return true
			default:
				return false
			}
		},
		OnReset: func(curTick uint64, archiveTick uint64) {
			newSeason := w.seasonIndex(curTick) + 1
			w.resetWorldForNewSeason(curTick, newSeason, archiveTick)
			w.auditEvent(curTick, "SYSTEM", "WORLD_RESET", Vec3i{}, "ADMIN_RESET", map[string]any{
				"world_id":     w.cfg.ID,
				"archive_tick": archiveTick,
				"new_seed":     w.cfg.Seed,
			})
		},
	})
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- adminResetResp{Tick: result.Tick, Err: result.Err}:
		default:
		}
	}
}
