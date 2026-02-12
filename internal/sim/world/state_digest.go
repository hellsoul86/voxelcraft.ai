package world

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"

	digestfeaturepkg "voxelcraft.ai/internal/sim/world/feature/persistence/digest"
)

func (w *World) stateDigest(nowTick uint64) string {
	h := sha256.New()
	var tmp [8]byte

	w.digestHeader(h, &tmp, nowTick)
	w.digestChunks(h, &tmp)
	w.digestClaims(h, &tmp)
	w.digestLaws(h, &tmp)
	w.digestOrgs(h, &tmp)
	w.digestContainers(h, &tmp)
	w.digestItems(h, &tmp)
	w.digestSigns(h, &tmp)
	w.digestConveyors(h, &tmp)
	w.digestSwitches(h, &tmp)
	w.digestContracts(h, &tmp)
	w.digestTrades(h, &tmp)
	w.digestBoards(h, &tmp)
	w.digestStructures(h, &tmp)
	w.digestAgents(h, &tmp)

	return hex.EncodeToString(h.Sum(nil))
}

func digestWriteU64(h hashWriter, tmp *[8]byte, v uint64) {
	binary.LittleEndian.PutUint64(tmp[:], v)
	h.Write(tmp[:])
}

func digestWriteI64(h hashWriter, tmp *[8]byte, v int64) {
	digestWriteU64(h, tmp, uint64(v))
}

func boolByte(b bool) byte {
	return digestfeaturepkg.BoolByte(b)
}

func writeItemMap(h hashWriter, tmp [8]byte, m map[string]int) {
	digestfeaturepkg.WriteSortedNonZeroIntMap(h, &tmp, m)
}

type hashWriter interface {
	Write(p []byte) (n int, err error)
}

func clamp01(x float64) float64 {
	return digestfeaturepkg.Clamp01(x)
}

func (w *World) digestAgents(h hashWriter, tmp *[8]byte) {
	agents := w.sortedAgents()
	for _, a := range agents {
		h.Write([]byte(a.ID))
		h.Write([]byte(a.Name))
		h.Write([]byte(a.OrgID))
		digestWriteI64(h, tmp, int64(a.Pos.X))
		digestWriteI64(h, tmp, int64(a.Pos.Y))
		digestWriteI64(h, tmp, int64(a.Pos.Z))
		digestWriteI64(h, tmp, int64(a.Yaw))
		digestWriteU64(h, tmp, uint64(a.HP))
		digestWriteU64(h, tmp, uint64(a.Hunger))
		digestWriteU64(h, tmp, uint64(a.StaminaMilli))
		digestWriteU64(h, tmp, uint64(a.RepTrade))
		digestWriteU64(h, tmp, uint64(a.RepBuild))
		digestWriteU64(h, tmp, uint64(a.RepSocial))
		digestWriteU64(h, tmp, uint64(a.RepLaw))
		digestWriteU64(h, tmp, uint64(a.Fun.Novelty))
		digestWriteU64(h, tmp, uint64(a.Fun.Creation))
		digestWriteU64(h, tmp, uint64(a.Fun.Social))
		digestWriteU64(h, tmp, uint64(a.Fun.Influence))
		digestWriteU64(h, tmp, uint64(a.Fun.Narrative))
		digestWriteU64(h, tmp, uint64(a.Fun.RiskRescue))

		// Fun novelty memory (seen biome/recipes/events) and anti-exploit state.
		biomes := make([]string, 0, len(a.seenBiomes))
		for b, ok := range a.seenBiomes {
			if ok {
				biomes = append(biomes, b)
			}
		}
		sort.Strings(biomes)
		digestWriteU64(h, tmp, uint64(len(biomes)))
		for _, b := range biomes {
			h.Write([]byte(b))
		}
		recipes := make([]string, 0, len(a.seenRecipes))
		for r, ok := range a.seenRecipes {
			if ok {
				recipes = append(recipes, r)
			}
		}
		sort.Strings(recipes)
		digestWriteU64(h, tmp, uint64(len(recipes)))
		for _, r := range recipes {
			h.Write([]byte(r))
		}
		events := make([]string, 0, len(a.seenEvents))
		for e, ok := range a.seenEvents {
			if ok {
				events = append(events, e)
			}
		}
		sort.Strings(events)
		digestWriteU64(h, tmp, uint64(len(events)))
		for _, e := range events {
			h.Write([]byte(e))
		}
		decayKeys := make([]string, 0, len(a.funDecay))
		for k, dw := range a.funDecay {
			if dw != nil {
				decayKeys = append(decayKeys, k)
			}
		}
		sort.Strings(decayKeys)
		digestWriteU64(h, tmp, uint64(len(decayKeys)))
		for _, k := range decayKeys {
			dw := a.funDecay[k]
			h.Write([]byte(k))
			digestWriteU64(h, tmp, dw.StartTick)
			digestWriteU64(h, tmp, uint64(dw.Count))
		}
		h.Write([]byte(a.Equipment.MainHand))
		for i := 0; i < 4; i++ {
			h.Write([]byte(a.Equipment.Armor[i]))
		}

		// Tasks (affects future simulation state; include in digest).
		h.Write([]byte{boolByte(a.MoveTask != nil)})
		if a.MoveTask != nil {
			mt := a.MoveTask
			h.Write([]byte(mt.TaskID))
			h.Write([]byte(string(mt.Kind)))
			digestWriteI64(h, tmp, int64(mt.Target.X))
			digestWriteI64(h, tmp, int64(mt.Target.Y))
			digestWriteI64(h, tmp, int64(mt.Target.Z))
			digestWriteU64(h, tmp, math.Float64bits(mt.Tolerance))
			h.Write([]byte(mt.TargetID))
			digestWriteU64(h, tmp, math.Float64bits(mt.Distance))
			digestWriteI64(h, tmp, int64(mt.StartPos.X))
			digestWriteI64(h, tmp, int64(mt.StartPos.Y))
			digestWriteI64(h, tmp, int64(mt.StartPos.Z))
			digestWriteU64(h, tmp, mt.StartedTick)
		}
		h.Write([]byte{boolByte(a.WorkTask != nil)})
		if a.WorkTask != nil {
			wt := a.WorkTask
			h.Write([]byte(wt.TaskID))
			h.Write([]byte(string(wt.Kind)))
			digestWriteI64(h, tmp, int64(wt.BlockPos.X))
			digestWriteI64(h, tmp, int64(wt.BlockPos.Y))
			digestWriteI64(h, tmp, int64(wt.BlockPos.Z))
			h.Write([]byte(wt.RecipeID))
			h.Write([]byte(wt.ItemID))
			digestWriteU64(h, tmp, uint64(wt.Count))
			h.Write([]byte(wt.BlueprintID))
			digestWriteI64(h, tmp, int64(wt.Anchor.X))
			digestWriteI64(h, tmp, int64(wt.Anchor.Y))
			digestWriteI64(h, tmp, int64(wt.Anchor.Z))
			digestWriteU64(h, tmp, uint64(wt.Rotation))
			digestWriteU64(h, tmp, uint64(wt.BuildIndex))
			h.Write([]byte(wt.TargetID))
			h.Write([]byte(wt.SrcContainer))
			h.Write([]byte(wt.DstContainer))
			digestWriteU64(h, tmp, wt.StartedTick)
			digestWriteU64(h, tmp, uint64(wt.WorkTicks))
		}

		// Inventory (sorted).
		inv := a.InventoryList()
		for _, it := range inv {
			h.Write([]byte(it.Item))
			digestWriteU64(h, tmp, uint64(it.Count))
		}
	}
}

func (w *World) digestHeader(h hashWriter, tmp *[8]byte, nowTick uint64) {
	digestWriteU64(h, tmp, nowTick)
	digestWriteU64(h, tmp, uint64(w.cfg.Seed))
	h.Write([]byte(w.weather))
	digestWriteU64(h, tmp, w.weatherUntilTick)
	h.Write([]byte(w.activeEventID))
	digestWriteU64(h, tmp, w.activeEventStart)
	digestWriteU64(h, tmp, w.activeEventEnds)
	digestWriteI64(h, tmp, int64(w.activeEventCenter.X))
	digestWriteI64(h, tmp, int64(w.activeEventCenter.Y))
	digestWriteI64(h, tmp, int64(w.activeEventCenter.Z))
	digestWriteU64(h, tmp, uint64(w.activeEventRadius))
}

func (w *World) digestChunks(h hashWriter, tmp *[8]byte) {
	for _, k := range w.chunks.LoadedChunkKeys() {
		digestWriteI64(h, tmp, int64(k.CX))
		digestWriteI64(h, tmp, int64(k.CZ))
		ch := w.chunks.chunks[k]
		d := ch.Digest()
		h.Write(d[:])
	}
}

func (w *World) digestClaims(h hashWriter, tmp *[8]byte) {
	landIDs := make([]string, 0, len(w.claims))
	for id := range w.claims {
		landIDs = append(landIDs, id)
	}
	sort.Strings(landIDs)
	for _, id := range landIDs {
		c := w.claims[id]
		h.Write([]byte(id))
		h.Write([]byte(c.Owner))
		digestWriteI64(h, tmp, int64(c.Anchor.X))
		digestWriteI64(h, tmp, int64(c.Anchor.Y))
		digestWriteI64(h, tmp, int64(c.Anchor.Z))
		digestWriteU64(h, tmp, uint64(c.Radius))
		h.Write([]byte{boolByte(c.Flags.AllowBuild), boolByte(c.Flags.AllowBreak), boolByte(c.Flags.AllowDamage), boolByte(c.Flags.AllowTrade)})
		if len(c.Members) > 0 {
			memberIDs := make([]string, 0, len(c.Members))
			for mid, ok := range c.Members {
				if ok {
					memberIDs = append(memberIDs, mid)
				}
			}
			sort.Strings(memberIDs)
			digestWriteU64(h, tmp, uint64(len(memberIDs)))
			for _, mid := range memberIDs {
				h.Write([]byte(mid))
			}
		} else {
			digestWriteU64(h, tmp, 0)
		}
		digestWriteU64(h, tmp, math.Float64bits(c.MarketTax))
		h.Write([]byte{boolByte(c.CurfewEnabled)})
		digestWriteU64(h, tmp, math.Float64bits(c.CurfewStart))
		digestWriteU64(h, tmp, math.Float64bits(c.CurfewEnd))
		h.Write([]byte{boolByte(c.FineBreakEnabled)})
		h.Write([]byte(c.FineBreakItem))
		digestWriteU64(h, tmp, uint64(c.FineBreakPerBlock))
		h.Write([]byte{boolByte(c.AccessPassEnabled)})
		h.Write([]byte(c.AccessTicketItem))
		digestWriteU64(h, tmp, uint64(c.AccessTicketCost))
		digestWriteU64(h, tmp, c.MaintenanceDueTick)
		digestWriteU64(h, tmp, uint64(c.MaintenanceStage))
	}
}

func (w *World) digestLaws(h hashWriter, tmp *[8]byte) {
	if len(w.laws) == 0 {
		return
	}
	lawIDs := make([]string, 0, len(w.laws))
	for id := range w.laws {
		lawIDs = append(lawIDs, id)
	}
	sort.Strings(lawIDs)
	for _, id := range lawIDs {
		l := w.laws[id]
		if l == nil {
			continue
		}
		h.Write([]byte(id))
		h.Write([]byte(l.LandID))
		h.Write([]byte(l.TemplateID))
		h.Write([]byte(l.Title))
		h.Write([]byte(l.ProposedBy))
		h.Write([]byte(string(l.Status)))
		digestWriteU64(h, tmp, l.ProposedTick)
		digestWriteU64(h, tmp, l.NoticeEndsTick)
		digestWriteU64(h, tmp, l.VoteEndsTick)

		if len(l.Params) > 0 {
			keys := make([]string, 0, len(l.Params))
			for k := range l.Params {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				h.Write([]byte(k))
				h.Write([]byte(l.Params[k]))
			}
		}
		if len(l.Votes) > 0 {
			voters := make([]string, 0, len(l.Votes))
			for aid := range l.Votes {
				voters = append(voters, aid)
			}
			sort.Strings(voters)
			for _, aid := range voters {
				h.Write([]byte(aid))
				h.Write([]byte(l.Votes[aid]))
			}
		}
	}
}

func (w *World) digestOrgs(h hashWriter, tmp *[8]byte) {
	if len(w.orgs) == 0 {
		return
	}
	orgIDs := make([]string, 0, len(w.orgs))
	for id := range w.orgs {
		orgIDs = append(orgIDs, id)
	}
	sort.Strings(orgIDs)
	for _, id := range orgIDs {
		o := w.orgs[id]
		if o == nil {
			continue
		}
		h.Write([]byte(id))
		h.Write([]byte(string(o.Kind)))
		h.Write([]byte(o.Name))
		digestWriteU64(h, tmp, o.CreatedTick)
		if len(o.Members) > 0 {
			memberIDs := make([]string, 0, len(o.Members))
			for aid := range o.Members {
				memberIDs = append(memberIDs, aid)
			}
			sort.Strings(memberIDs)
			for _, aid := range memberIDs {
				h.Write([]byte(aid))
				h.Write([]byte(string(o.Members[aid])))
			}
		}
		writeItemMap(h, *tmp, w.orgTreasury(o))
	}
}

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
