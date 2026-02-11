package world

import (
	"context"
	"errors"
)

type adminResetReq struct {
	Resp chan adminResetResp
}

type adminResetResp struct {
	Tick uint64
	Err  string
}

// RequestReset asks the world loop goroutine to perform an immediate world reset.
// It is safe to call from other goroutines (e.g. admin HTTP handlers).
func (w *World) RequestReset(ctx context.Context) (tick uint64, err error) {
	if w == nil || w.adminReset == nil {
		return 0, errors.New("admin reset not available")
	}
	resp := make(chan adminResetResp, 1)
	req := adminResetReq{Resp: resp}

	select {
	case w.adminReset <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	select {
	case r := <-resp:
		if r.Err != "" {
			return r.Tick, errors.New(r.Err)
		}
		return r.Tick, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (w *World) handleAdminResetRequests(reqs []adminResetReq) {
	if w == nil || len(reqs) == 0 {
		return
	}

	cur := w.tick.Load()
	archiveTick := uint64(0)
	if cur > 0 {
		archiveTick = cur - 1
	}

	errStr := ""
	if w.snapshotSink != nil {
		snap := w.ExportSnapshot(archiveTick)
		select {
		case w.snapshotSink <- snap:
		default:
			errStr = "snapshot sink backpressure"
		}
	}
	if errStr == "" {
		newSeason := w.seasonIndex(cur) + 1
		w.resetWorldForNewSeason(cur, newSeason, archiveTick)
		w.auditEvent(cur, "SYSTEM", "WORLD_RESET", Vec3i{}, "ADMIN_RESET", map[string]any{
			"world_id":     w.cfg.ID,
			"archive_tick": archiveTick,
			"new_seed":     w.cfg.Seed,
		})
	}

	resp := adminResetResp{Tick: cur, Err: errStr}
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- resp:
		default:
		}
	}
}
