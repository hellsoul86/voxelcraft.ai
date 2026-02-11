package world

import (
	"encoding/json"
	"sort"

	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/world/io/obscodec"
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

func encodePAL16Y8(surface []surfaceCell) string {
	blocks := make([]uint16, len(surface))
	ys := make([]byte, len(surface))
	for i, c := range surface {
		blocks[i] = c.b
		ys[i] = c.y
	}
	return obscodec.EncodePAL16Y8(blocks, ys)
}
