package world

import (
	"context"
	"errors"
)

type adminSnapshotReq struct {
	Resp chan adminSnapshotResp
}

type adminSnapshotResp struct {
	Tick uint64
	Err  string
}

// RequestSnapshot asks the world loop goroutine to enqueue a snapshot.
// It is safe to call from other goroutines (e.g. HTTP handlers).
func (w *World) RequestSnapshot(ctx context.Context) (tick uint64, err error) {
	if w == nil || w.admin == nil {
		return 0, errors.New("admin snapshot not available")
	}
	resp := make(chan adminSnapshotResp, 1)
	req := adminSnapshotReq{Resp: resp}

	select {
	case w.admin <- req:
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

func (w *World) handleAdminSnapshotRequests(reqs []adminSnapshotReq) {
	if w == nil || len(reqs) == 0 {
		return
	}
	cur := w.tick.Load()
	snapTick := uint64(0)
	if cur > 0 {
		snapTick = cur - 1
	}

	errStr := ""
	if w.snapshotSink == nil {
		errStr = "snapshot sink not configured"
	} else {
		snap := w.ExportSnapshot(snapTick)
		select {
		case w.snapshotSink <- snap:
		default:
			errStr = "snapshot sink backpressure"
		}
	}

	resp := adminSnapshotResp{Tick: snapTick, Err: errStr}
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- resp:
		default:
			// Client timed out; don't block the sim loop.
		}
	}
}
