package world

import "voxelcraft.ai/internal/persistence/snapshot"

func (w *World) ExportSnapshot(nowTick uint64) snapshot.SnapshotV1 {
	return w.exportSnapshot(nowTick)
}
