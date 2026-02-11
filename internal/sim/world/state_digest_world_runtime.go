package world

import "sort"

func (w *World) digestContainers(h hashWriter, tmp *[8]byte) {
	if len(w.containers) == 0 {
		return
	}
	posKeys := make([]Vec3i, 0, len(w.containers))
	for p := range w.containers {
		posKeys = append(posKeys, p)
	}
	sort.Slice(posKeys, func(i, j int) bool {
		if posKeys[i].X != posKeys[j].X {
			return posKeys[i].X < posKeys[j].X
		}
		if posKeys[i].Y != posKeys[j].Y {
			return posKeys[i].Y < posKeys[j].Y
		}
		return posKeys[i].Z < posKeys[j].Z
	})
	for _, p := range posKeys {
		c := w.containers[p]
		h.Write([]byte(c.Type))
		digestWriteI64(h, tmp, int64(c.Pos.X))
		digestWriteI64(h, tmp, int64(c.Pos.Y))
		digestWriteI64(h, tmp, int64(c.Pos.Z))
		writeItemMap(h, *tmp, c.Inventory)
		writeItemMap(h, *tmp, c.Reserved)
		if c.Owed != nil {
			owedAgents := make([]string, 0, len(c.Owed))
			for aid := range c.Owed {
				owedAgents = append(owedAgents, aid)
			}
			sort.Strings(owedAgents)
			for _, aid := range owedAgents {
				h.Write([]byte(aid))
				writeItemMap(h, *tmp, c.Owed[aid])
			}
		}
	}
}

func (w *World) digestItems(h hashWriter, tmp *[8]byte) {
	if len(w.items) > 0 {
		itemIDs := make([]string, 0, len(w.items))
		for id, e := range w.items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			itemIDs = append(itemIDs, id)
		}
		sort.Strings(itemIDs)
		digestWriteU64(h, tmp, uint64(len(itemIDs)))
		for _, id := range itemIDs {
			e := w.items[id]
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			h.Write([]byte(id))
			digestWriteI64(h, tmp, int64(e.Pos.X))
			digestWriteI64(h, tmp, int64(e.Pos.Y))
			digestWriteI64(h, tmp, int64(e.Pos.Z))
			h.Write([]byte(e.Item))
			digestWriteU64(h, tmp, uint64(e.Count))
			digestWriteU64(h, tmp, e.CreatedTick)
			digestWriteU64(h, tmp, e.ExpiresTick)
		}
		return
	}
	digestWriteU64(h, tmp, 0)
}

func (w *World) digestSigns(h hashWriter, tmp *[8]byte) {
	if len(w.signs) > 0 {
		posKeys := make([]Vec3i, 0, len(w.signs))
		for p, s := range w.signs {
			if s == nil {
				continue
			}
			posKeys = append(posKeys, p)
		}
		sort.Slice(posKeys, func(i, j int) bool {
			if posKeys[i].X != posKeys[j].X {
				return posKeys[i].X < posKeys[j].X
			}
			if posKeys[i].Y != posKeys[j].Y {
				return posKeys[i].Y < posKeys[j].Y
			}
			return posKeys[i].Z < posKeys[j].Z
		})
		digestWriteU64(h, tmp, uint64(len(posKeys)))
		for _, p := range posKeys {
			s := w.signs[p]
			if s == nil {
				continue
			}
			digestWriteI64(h, tmp, int64(p.X))
			digestWriteI64(h, tmp, int64(p.Y))
			digestWriteI64(h, tmp, int64(p.Z))
			h.Write([]byte(s.Text))
			digestWriteU64(h, tmp, s.UpdatedTick)
			h.Write([]byte(s.UpdatedBy))
		}
		return
	}
	digestWriteU64(h, tmp, 0)
}

func (w *World) digestConveyors(h hashWriter, tmp *[8]byte) {
	if len(w.conveyors) > 0 {
		posKeys := make([]Vec3i, 0, len(w.conveyors))
		for p := range w.conveyors {
			posKeys = append(posKeys, p)
		}
		sort.Slice(posKeys, func(i, j int) bool {
			if posKeys[i].X != posKeys[j].X {
				return posKeys[i].X < posKeys[j].X
			}
			if posKeys[i].Y != posKeys[j].Y {
				return posKeys[i].Y < posKeys[j].Y
			}
			return posKeys[i].Z < posKeys[j].Z
		})
		digestWriteU64(h, tmp, uint64(len(posKeys)))
		for _, p := range posKeys {
			m := w.conveyors[p]
			digestWriteI64(h, tmp, int64(p.X))
			digestWriteI64(h, tmp, int64(p.Y))
			digestWriteI64(h, tmp, int64(p.Z))
			digestWriteI64(h, tmp, int64(m.DX))
			digestWriteI64(h, tmp, int64(m.DZ))
		}
		return
	}
	digestWriteU64(h, tmp, 0)
}

func (w *World) digestSwitches(h hashWriter, tmp *[8]byte) {
	if len(w.switches) > 0 {
		posKeys := make([]Vec3i, 0, len(w.switches))
		for p := range w.switches {
			posKeys = append(posKeys, p)
		}
		sort.Slice(posKeys, func(i, j int) bool {
			if posKeys[i].X != posKeys[j].X {
				return posKeys[i].X < posKeys[j].X
			}
			if posKeys[i].Y != posKeys[j].Y {
				return posKeys[i].Y < posKeys[j].Y
			}
			return posKeys[i].Z < posKeys[j].Z
		})
		digestWriteU64(h, tmp, uint64(len(posKeys)))
		for _, p := range posKeys {
			on := w.switches[p]
			digestWriteI64(h, tmp, int64(p.X))
			digestWriteI64(h, tmp, int64(p.Y))
			digestWriteI64(h, tmp, int64(p.Z))
			h.Write([]byte{boolByte(on)})
		}
		return
	}
	digestWriteU64(h, tmp, 0)
}
