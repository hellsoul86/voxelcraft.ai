package handlers

import tickspkg "voxelcraft.ai/internal/sim/world/feature/admin/ticks"

type SnapshotInput struct {
	CurrentTick uint64
	HasSink     bool
	Enqueue     func(snapshotTick uint64) bool
}

type ResetInput struct {
	CurrentTick uint64
	HasSink     bool
	Enqueue     func(archiveTick uint64) bool
	OnReset     func(curTick uint64, archiveTick uint64)
}

type SnapshotResp struct {
	Tick uint64
	Err  string
}

type ResetResp struct {
	Tick uint64
	Err  string
}

func HandleSnapshot(input SnapshotInput) SnapshotResp {
	snapTick := tickspkg.SnapshotTick(input.CurrentTick)
	resp := SnapshotResp{Tick: snapTick}
	if !input.HasSink {
		resp.Err = "snapshot sink not configured"
		return resp
	}
	if input.Enqueue == nil || !input.Enqueue(snapTick) {
		resp.Err = "snapshot sink backpressure"
	}
	return resp
}

func HandleReset(input ResetInput) ResetResp {
	archiveTick := tickspkg.ArchiveTick(input.CurrentTick)
	resp := ResetResp{Tick: input.CurrentTick}
	if input.HasSink {
		if input.Enqueue == nil || !input.Enqueue(archiveTick) {
			resp.Err = "snapshot sink backpressure"
			return resp
		}
	}
	if input.OnReset != nil {
		input.OnReset(input.CurrentTick, archiveTick)
	}
	return resp
}
