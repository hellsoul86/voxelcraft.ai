package world

import "sort"

func (w *World) digestContracts(h hashWriter, tmp *[8]byte) {
	contractIDs := make([]string, 0, len(w.contracts))
	for id := range w.contracts {
		contractIDs = append(contractIDs, id)
	}
	sort.Strings(contractIDs)
	for _, id := range contractIDs {
		c := w.contracts[id]
		h.Write([]byte(id))
		h.Write([]byte(c.Kind))
		h.Write([]byte(string(c.State)))
		h.Write([]byte(c.Poster))
		h.Write([]byte(c.Acceptor))
		digestWriteU64(h, tmp, c.CreatedTick)
		digestWriteU64(h, tmp, c.DeadlineTick)
		digestWriteI64(h, tmp, int64(c.TerminalPos.X))
		digestWriteI64(h, tmp, int64(c.TerminalPos.Y))
		digestWriteI64(h, tmp, int64(c.TerminalPos.Z))
		writeItemMap(h, *tmp, c.Requirements)
		writeItemMap(h, *tmp, c.Reward)
		writeItemMap(h, *tmp, c.Deposit)
		h.Write([]byte(c.BlueprintID))
		digestWriteI64(h, tmp, int64(c.Anchor.X))
		digestWriteI64(h, tmp, int64(c.Anchor.Y))
		digestWriteI64(h, tmp, int64(c.Anchor.Z))
		digestWriteU64(h, tmp, uint64(c.Rotation))
	}
}

func (w *World) digestTrades(h hashWriter, tmp *[8]byte) {
	tradeIDs := make([]string, 0, len(w.trades))
	for id := range w.trades {
		tradeIDs = append(tradeIDs, id)
	}
	sort.Strings(tradeIDs)
	for _, id := range tradeIDs {
		tr := w.trades[id]
		h.Write([]byte(id))
		h.Write([]byte(tr.From))
		h.Write([]byte(tr.To))
		writeItemMap(h, *tmp, tr.Offer)
		writeItemMap(h, *tmp, tr.Request)
	}
}

func (w *World) digestBoards(h hashWriter, tmp *[8]byte) {
	boardIDs := make([]string, 0, len(w.boards))
	for id := range w.boards {
		boardIDs = append(boardIDs, id)
	}
	sort.Strings(boardIDs)
	for _, id := range boardIDs {
		b := w.boards[id]
		if b == nil {
			continue
		}
		h.Write([]byte(id))
		for _, p := range b.Posts {
			h.Write([]byte(p.PostID))
			h.Write([]byte(p.Author))
			h.Write([]byte(p.Title))
			h.Write([]byte(p.Body))
			digestWriteU64(h, tmp, p.Tick)
		}
	}
}

func (w *World) digestStructures(h hashWriter, tmp *[8]byte) {
	structIDs := make([]string, 0, len(w.structures))
	for id := range w.structures {
		structIDs = append(structIDs, id)
	}
	sort.Strings(structIDs)
	digestWriteU64(h, tmp, uint64(len(structIDs)))
	for _, id := range structIDs {
		s := w.structures[id]
		if s == nil {
			continue
		}
		h.Write([]byte(s.StructureID))
		h.Write([]byte(s.BlueprintID))
		h.Write([]byte(s.BuilderID))
		digestWriteI64(h, tmp, int64(s.Anchor.X))
		digestWriteI64(h, tmp, int64(s.Anchor.Y))
		digestWriteI64(h, tmp, int64(s.Anchor.Z))
		digestWriteU64(h, tmp, uint64(s.Rotation))
		digestWriteI64(h, tmp, int64(s.Min.X))
		digestWriteI64(h, tmp, int64(s.Min.Y))
		digestWriteI64(h, tmp, int64(s.Min.Z))
		digestWriteI64(h, tmp, int64(s.Max.X))
		digestWriteI64(h, tmp, int64(s.Max.Y))
		digestWriteI64(h, tmp, int64(s.Max.Z))
		digestWriteU64(h, tmp, s.CompletedTick)
		digestWriteU64(h, tmp, s.AwardDueTick)
		h.Write([]byte{boolByte(s.Awarded)})
		digestWriteU64(h, tmp, uint64(s.LastInfluenceDay))

		if len(s.UsedBy) > 0 {
			usedIDs := make([]string, 0, len(s.UsedBy))
			for aid := range s.UsedBy {
				usedIDs = append(usedIDs, aid)
			}
			sort.Strings(usedIDs)
			digestWriteU64(h, tmp, uint64(len(usedIDs)))
			for _, aid := range usedIDs {
				h.Write([]byte(aid))
				digestWriteU64(h, tmp, s.UsedBy[aid])
			}
		} else {
			digestWriteU64(h, tmp, 0)
		}
	}
}
