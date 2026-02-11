package world

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"

	"voxelcraft.ai/internal/observerproto"
)

func (w *World) stepObserverChunksForClient(nowTick uint64, c *observerClient, connected []ChunkKey, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}

	wantKeys := computeWantedChunks(connected, c.cfg.chunkRadius, c.cfg.maxChunks)
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	// Track wanted chunks, enqueue full surfaces as needed (rate-limited).
	fullBudget := observerMaxFullChunksPerTick
	canSend := true
	for _, k := range wantKeys {
		st := c.chunks[k]
		if st == nil {
			st = &observerChunk{
				key:            k,
				lastWantedTick: nowTick,
				needsFull:      true,
			}
			c.chunks[k] = st
		} else {
			st.lastWantedTick = nowTick
		}

		if canSend && st.needsFull && fullBudget > 0 {
			if st.surface == nil {
				st.surface = w.computeChunkSurface(k.CX, k.CZ)
			}
			if w.sendChunkSurface(c, st) {
				st.sentFull = true
				st.needsFull = false
				fullBudget--
			} else {
				// Channel is likely full; don't spend more CPU on sends this tick.
				canSend = false
			}
		}
	}

	// Apply audits -> patch (or force resync).
	patches := map[ChunkKey]map[int]observerproto.ChunkPatchCell{}
	for _, e := range audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx := e.Pos[0]
		wz := e.Pos[2]
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		st := c.chunks[key]
		if st == nil {
			continue
		}
		// If we don't have a baseline surface yet, just ignore PATCH; a future full send will catch up.
		if st.surface == nil {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lz*16 + lx
		newCell := w.computeSurfaceCellAt(wx, wz)
		old := st.surface[idx]
		if old == newCell {
			continue
		}
		st.surface[idx] = newCell
		if st.needsFull {
			continue
		}
		m := patches[key]
		if m == nil {
			m = map[int]observerproto.ChunkPatchCell{}
			patches[key] = m
		}
		m[idx] = observerproto.ChunkPatchCell{X: lx, Z: lz, Block: newCell.b, Y: int(newCell.y)}
	}

	for key, m := range patches {
		st := c.chunks[key]
		if st == nil || st.needsFull {
			continue
		}
		cells := make([]observerproto.ChunkPatchCell, 0, len(m))
		for _, cell := range m {
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Z != cells[j].Z {
				return cells[i].Z < cells[j].Z
			}
			return cells[i].X < cells[j].X
		})
		msg := observerproto.ChunkPatchMsg{
			Type:            "CHUNK_PATCH",
			ProtocolVersion: observerproto.Version,
			CX:              key.CX,
			CZ:              key.CZ,
			Cells:           cells,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if !trySend(c.dataOut, b) {
			// Force a full resync next tick if PATCH is dropped.
			st.needsFull = true
		}
	}

	// Evictions.
	if len(c.chunks) > 0 {
		var evictKeys []ChunkKey
		for k, st := range c.chunks {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if nowTick-st.lastWantedTick < observerEvictAfterTicks {
				continue
			}
			if !st.sentFull {
				evictKeys = append(evictKeys, k)
				continue
			}

			msg := observerproto.ChunkEvictMsg{
				Type:            "CHUNK_EVICT",
				ProtocolVersion: observerproto.Version,
				CX:              k.CX,
				CZ:              k.CZ,
			}
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if trySend(c.dataOut, b) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(c.chunks, k)
		}
	}
}

func (w *World) stepObserverVoxelChunksForClient(nowTick uint64, c *observerClient, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}

	focusID := strings.TrimSpace(c.cfg.focusAgentID)
	radius := c.cfg.voxelRadius
	if focusID == "" || radius <= 0 {
		// If voxels are disabled, evict any previously-sent voxel chunks immediately.
		if len(c.voxelChunks) > 0 {
			for k, st := range c.voxelChunks {
				if st != nil && st.sentFull {
					msg := observerproto.ChunkVoxelsEvictMsg{
						Type:            "CHUNK_VOXELS_EVICT",
						ProtocolVersion: observerproto.Version,
						CX:              k.CX,
						CZ:              k.CZ,
					}
					if b, err := json.Marshal(msg); err == nil {
						_ = trySend(c.dataOut, b)
					}
				}
				delete(c.voxelChunks, k)
			}
		}
		return
	}

	a := w.agents[focusID]
	if a == nil {
		return
	}

	center := ChunkKey{CX: floorDiv(a.Pos.X, 16), CZ: floorDiv(a.Pos.Z, 16)}
	wantKeys := computeWantedChunks([]ChunkKey{center}, radius, c.cfg.voxelMaxChunks)
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	// Track wanted chunks and enqueue full voxel chunks as needed (rate-limited).
	fullBudget := observerMaxFullVoxelChunksPerTick
	canSend := true
	for _, k := range wantKeys {
		st := c.voxelChunks[k]
		if st == nil {
			st = &observerVoxelChunk{
				key:            k,
				lastWantedTick: nowTick,
				needsFull:      true,
			}
			c.voxelChunks[k] = st
		} else {
			st.lastWantedTick = nowTick
		}

		if canSend && st.needsFull && fullBudget > 0 {
			if st.blocks == nil {
				st.blocks = w.computeChunkVoxels(k.CX, k.CZ)
			}
			if w.sendChunkVoxels(c, st) {
				st.sentFull = true
				st.needsFull = false
				fullBudget--
			} else {
				// Channel is likely full; don't spend more CPU on sends this tick.
				canSend = false
			}
		}
	}

	// Apply audits -> voxel patch (or force resync).
	patches := map[ChunkKey]map[int]observerproto.ChunkVoxelPatchCell{}
	for _, e := range audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx, wy, wz := e.Pos[0], e.Pos[1], e.Pos[2]
		// 2D world: only y==0 exists.
		if wy != 0 {
			continue
		}
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		if _, ok := wantSet[key]; !ok {
			continue
		}
		st := c.voxelChunks[key]
		if st == nil || st.blocks == nil {
			continue
		}
		if st.needsFull {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lx + lz*16
		if idx < 0 || idx >= len(st.blocks) {
			continue
		}
		if st.blocks[idx] == e.To {
			continue
		}
		st.blocks[idx] = e.To

		m := patches[key]
		if m == nil {
			m = map[int]observerproto.ChunkVoxelPatchCell{}
			patches[key] = m
		}
		m[idx] = observerproto.ChunkVoxelPatchCell{X: lx, Y: 0, Z: lz, Block: e.To}
	}

	for key, m := range patches {
		st := c.voxelChunks[key]
		if st == nil || st.needsFull {
			continue
		}
		cells := make([]observerproto.ChunkVoxelPatchCell, 0, len(m))
		for _, cell := range m {
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Y != cells[j].Y {
				return cells[i].Y < cells[j].Y
			}
			if cells[i].Z != cells[j].Z {
				return cells[i].Z < cells[j].Z
			}
			return cells[i].X < cells[j].X
		})
		msg := observerproto.ChunkVoxelPatchMsg{
			Type:            "CHUNK_VOXEL_PATCH",
			ProtocolVersion: observerproto.Version,
			CX:              key.CX,
			CZ:              key.CZ,
			Cells:           cells,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if !trySend(c.dataOut, b) {
			// Force a full resync next tick if PATCH is dropped.
			st.needsFull = true
		}
	}

	// Evictions.
	if len(c.voxelChunks) > 0 {
		var evictKeys []ChunkKey
		for k, st := range c.voxelChunks {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if nowTick-st.lastWantedTick < observerVoxelEvictAfterTicks {
				continue
			}
			if !st.sentFull {
				evictKeys = append(evictKeys, k)
				continue
			}

			msg := observerproto.ChunkVoxelsEvictMsg{
				Type:            "CHUNK_VOXELS_EVICT",
				ProtocolVersion: observerproto.Version,
				CX:              k.CX,
				CZ:              k.CZ,
			}
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if trySend(c.dataOut, b) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(c.voxelChunks, k)
		}
	}
}

func (w *World) sendChunkSurface(c *observerClient, st *observerChunk) bool {
	if w == nil || c == nil || st == nil || st.surface == nil {
		return false
	}
	msg := observerproto.ChunkSurfaceMsg{
		Type:            "CHUNK_SURFACE",
		ProtocolVersion: observerproto.Version,
		CX:              st.key.CX,
		CZ:              st.key.CZ,
		Encoding:        "PAL16_Y8",
		Data:            encodePAL16Y8(st.surface),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return trySend(c.dataOut, b)
}

func (w *World) sendChunkVoxels(c *observerClient, st *observerVoxelChunk) bool {
	if w == nil || c == nil || st == nil || st.blocks == nil {
		return false
	}
	msg := observerproto.ChunkVoxelsMsg{
		Type:            "CHUNK_VOXELS",
		ProtocolVersion: observerproto.Version,
		CX:              st.key.CX,
		CZ:              st.key.CZ,
		Encoding:        "PAL16_U16LE_YZX",
		Data:            encodePAL16U16LE(st.blocks),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return trySend(c.dataOut, b)
}

func encodePAL16Y8(surface []surfaceCell) string {
	buf := make([]byte, 16*16*3)
	for i, c := range surface {
		off := i * 3
		buf[off] = byte(c.b)
		buf[off+1] = byte(c.b >> 8)
		buf[off+2] = c.y
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func encodePAL16U16LE(blocks []uint16) string {
	buf := make([]byte, len(blocks)*2)
	for i, v := range blocks {
		off := i * 2
		buf[off] = byte(v)
		buf[off+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
