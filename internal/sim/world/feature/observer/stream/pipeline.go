package stream

import (
	"sort"

	chunkspkg "voxelcraft.ai/internal/sim/world/feature/observer/chunks"
)

type AuditEntry struct {
	Action string
	Pos    [3]int
	To     uint16
}

type ChunkPatchCell struct {
	X     int
	Z     int
	Y     int
	Block uint16
}

type VoxelPatchCell struct {
	X     int
	Y     int
	Z     int
	Block uint16
}

type ChunkRuntimeState struct {
	LastWantedTick uint64
	SentFull       bool
	NeedsFull      bool
	Surface        []chunkspkg.SurfaceCell
}

type VoxelRuntimeState struct {
	LastWantedTick uint64
	SentFull       bool
	NeedsFull      bool
	Blocks         []uint16
}

type ChunkStepInput struct {
	NowTick         uint64
	Connected       []ChunkKey
	Radius          int
	MaxChunks       int
	MaxFullPerTick  int
	EvictAfterTicks uint64
	Audits          []AuditEntry
}

type ChunkStepDeps struct {
	ComputeSurface   func(cx, cz int) []chunkspkg.SurfaceCell
	ComputeSurfaceAt func(wx, wz int) chunkspkg.SurfaceCell
	SendFull         func(key ChunkKey, surface []chunkspkg.SurfaceCell) bool
	SendPatch        func(key ChunkKey, cells []ChunkPatchCell) bool
	SendEvict        func(key ChunkKey) bool
}

func StepChunkRuntime(states map[ChunkKey]*ChunkRuntimeState, in ChunkStepInput, deps ChunkStepDeps) {
	if states == nil {
		return
	}
	if in.MaxFullPerTick <= 0 {
		in.MaxFullPerTick = 32
	}
	if in.EvictAfterTicks == 0 {
		in.EvictAfterTicks = 50
	}

	wantKeys := ComputeWantedChunks(in.Connected, in.Radius, in.MaxChunks)
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	fullBudget := in.MaxFullPerTick
	canSend := true
	for _, k := range wantKeys {
		st := states[k]
		if st == nil {
			st = &ChunkRuntimeState{
				LastWantedTick: in.NowTick,
				NeedsFull:      true,
			}
			states[k] = st
		} else {
			st.LastWantedTick = in.NowTick
		}

		if canSend && st.NeedsFull && fullBudget > 0 {
			if st.Surface == nil {
				st.Surface = deps.ComputeSurface(k.CX, k.CZ)
			}
			if deps.SendFull(k, st.Surface) {
				st.SentFull = true
				st.NeedsFull = false
				fullBudget--
			} else {
				canSend = false
			}
		}
	}

	patches := map[ChunkKey]map[int]ChunkPatchCell{}
	for _, e := range in.Audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx := e.Pos[0]
		wz := e.Pos[2]
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		st := states[key]
		if st == nil || st.Surface == nil {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lz*16 + lx
		newCell := deps.ComputeSurfaceAt(wx, wz)
		old := st.Surface[idx]
		if old == newCell {
			continue
		}
		st.Surface[idx] = newCell
		if st.NeedsFull {
			continue
		}
		m := patches[key]
		if m == nil {
			m = map[int]ChunkPatchCell{}
			patches[key] = m
		}
		m[idx] = ChunkPatchCell{X: lx, Z: lz, Y: int(newCell.Y), Block: newCell.B}
	}

	for key, m := range patches {
		st := states[key]
		if st == nil || st.NeedsFull {
			continue
		}
		cells := make([]ChunkPatchCell, 0, len(m))
		for _, c := range m {
			cells = append(cells, c)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Z != cells[j].Z {
				return cells[i].Z < cells[j].Z
			}
			return cells[i].X < cells[j].X
		})
		if !deps.SendPatch(key, cells) {
			st.NeedsFull = true
		}
	}

	if len(states) > 0 {
		var evictKeys []ChunkKey
		for k, st := range states {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if in.NowTick-st.LastWantedTick < in.EvictAfterTicks {
				continue
			}
			if !st.SentFull {
				evictKeys = append(evictKeys, k)
				continue
			}
			if deps.SendEvict(k) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(states, k)
		}
	}
}

type VoxelStepInput struct {
	NowTick         uint64
	Enabled         bool
	FocusCenters    []ChunkKey
	Radius          int
	MaxChunks       int
	MaxFullPerTick  int
	EvictAfterTicks uint64
	Audits          []AuditEntry
}

type VoxelStepDeps struct {
	ComputeVoxels func(cx, cz int) []uint16
	SendFull      func(key ChunkKey, blocks []uint16) bool
	SendPatch     func(key ChunkKey, cells []VoxelPatchCell) bool
	SendEvict     func(key ChunkKey) bool
}

func StepVoxelRuntime(states map[ChunkKey]*VoxelRuntimeState, in VoxelStepInput, deps VoxelStepDeps) {
	if states == nil {
		return
	}
	if in.MaxFullPerTick <= 0 {
		in.MaxFullPerTick = 8
	}
	if in.EvictAfterTicks == 0 {
		in.EvictAfterTicks = 10
	}

	if !in.Enabled || len(in.FocusCenters) == 0 || in.Radius <= 0 {
		for k, st := range states {
			if st != nil && st.SentFull {
				_ = deps.SendEvict(k)
			}
			delete(states, k)
		}
		return
	}

	wantKeys := ComputeWantedChunks(in.FocusCenters, in.Radius, in.MaxChunks)
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	fullBudget := in.MaxFullPerTick
	canSend := true
	for _, k := range wantKeys {
		st := states[k]
		if st == nil {
			st = &VoxelRuntimeState{
				LastWantedTick: in.NowTick,
				NeedsFull:      true,
			}
			states[k] = st
		} else {
			st.LastWantedTick = in.NowTick
		}

		if canSend && st.NeedsFull && fullBudget > 0 {
			if st.Blocks == nil {
				st.Blocks = deps.ComputeVoxels(k.CX, k.CZ)
			}
			if deps.SendFull(k, st.Blocks) {
				st.SentFull = true
				st.NeedsFull = false
				fullBudget--
			} else {
				canSend = false
			}
		}
	}

	patches := map[ChunkKey]map[int]VoxelPatchCell{}
	for _, e := range in.Audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx, wy, wz := e.Pos[0], e.Pos[1], e.Pos[2]
		if wy != 0 {
			continue
		}
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		if _, ok := wantSet[key]; !ok {
			continue
		}
		st := states[key]
		if st == nil || st.Blocks == nil || st.NeedsFull {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lx + lz*16
		if idx < 0 || idx >= len(st.Blocks) {
			continue
		}
		if st.Blocks[idx] == e.To {
			continue
		}
		st.Blocks[idx] = e.To
		m := patches[key]
		if m == nil {
			m = map[int]VoxelPatchCell{}
			patches[key] = m
		}
		m[idx] = VoxelPatchCell{X: lx, Y: 0, Z: lz, Block: e.To}
	}

	for key, m := range patches {
		st := states[key]
		if st == nil || st.NeedsFull {
			continue
		}
		cells := make([]VoxelPatchCell, 0, len(m))
		for _, c := range m {
			cells = append(cells, c)
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
		if !deps.SendPatch(key, cells) {
			st.NeedsFull = true
		}
	}

	if len(states) > 0 {
		var evictKeys []ChunkKey
		for k, st := range states {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if in.NowTick-st.LastWantedTick < in.EvictAfterTicks {
				continue
			}
			if !st.SentFull {
				evictKeys = append(evictKeys, k)
				continue
			}
			if deps.SendEvict(k) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(states, k)
		}
	}
}

func floorDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	q := a / b
	r := a % b
	if r != 0 && ((r > 0) != (b > 0)) {
		q--
	}
	return q
}

func mod(a, b int) int {
	if b == 0 {
		return 0
	}
	r := a % b
	if r < 0 {
		r += b
	}
	return r
}
