package world

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
)

func (w *World) stateDigest(nowTick uint64) string {
	h := sha256.New()
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], nowTick)
	h.Write(tmp[:])
	binary.LittleEndian.PutUint64(tmp[:], uint64(w.cfg.Seed))
	h.Write(tmp[:])
	h.Write([]byte(w.weather))
	binary.LittleEndian.PutUint64(tmp[:], w.weatherUntilTick)
	h.Write(tmp[:])
	h.Write([]byte(w.activeEventID))
	binary.LittleEndian.PutUint64(tmp[:], w.activeEventStart)
	h.Write(tmp[:])
	binary.LittleEndian.PutUint64(tmp[:], w.activeEventEnds)
	h.Write(tmp[:])
	binary.LittleEndian.PutUint64(tmp[:], uint64(int64(w.activeEventCenter.X)))
	h.Write(tmp[:])
	binary.LittleEndian.PutUint64(tmp[:], uint64(int64(w.activeEventCenter.Y)))
	h.Write(tmp[:])
	binary.LittleEndian.PutUint64(tmp[:], uint64(int64(w.activeEventCenter.Z)))
	h.Write(tmp[:])
	binary.LittleEndian.PutUint64(tmp[:], uint64(w.activeEventRadius))
	h.Write(tmp[:])

	// Chunks (sorted keys).
	for _, k := range w.chunks.LoadedChunkKeys() {
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(k.CX)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(k.CZ)))
		h.Write(tmp[:])
		ch := w.chunks.chunks[k]
		d := ch.Digest()
		h.Write(d[:])
	}

	// Claims (sorted).
	landIDs := make([]string, 0, len(w.claims))
	for id := range w.claims {
		landIDs = append(landIDs, id)
	}
	sort.Strings(landIDs)
	for _, id := range landIDs {
		c := w.claims[id]
		h.Write([]byte(id))
		h.Write([]byte(c.Owner))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.Radius))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.Flags.AllowBuild), boolByte(c.Flags.AllowBreak), boolByte(c.Flags.AllowDamage), boolByte(c.Flags.AllowTrade)})
		if len(c.Members) > 0 {
			memberIDs := make([]string, 0, len(c.Members))
			for mid, ok := range c.Members {
				if ok {
					memberIDs = append(memberIDs, mid)
				}
			}
			sort.Strings(memberIDs)
			binary.LittleEndian.PutUint64(tmp[:], uint64(len(memberIDs)))
			h.Write(tmp[:])
			for _, mid := range memberIDs {
				h.Write([]byte(mid))
			}
		} else {
			binary.LittleEndian.PutUint64(tmp[:], 0)
			h.Write(tmp[:])
		}
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(c.MarketTax))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.CurfewEnabled)})
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(c.CurfewStart))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(c.CurfewEnd))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.FineBreakEnabled)})
		h.Write([]byte(c.FineBreakItem))
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.FineBreakPerBlock))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.AccessPassEnabled)})
		h.Write([]byte(c.AccessTicketItem))
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.AccessTicketCost))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], c.MaintenanceDueTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.MaintenanceStage))
		h.Write(tmp[:])
	}

	// Laws (sorted).
	if len(w.laws) > 0 {
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
			binary.LittleEndian.PutUint64(tmp[:], l.ProposedTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], l.NoticeEndsTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], l.VoteEndsTick)
			h.Write(tmp[:])

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

	// Orgs (sorted).
	if len(w.orgs) > 0 {
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
			binary.LittleEndian.PutUint64(tmp[:], o.CreatedTick)
			h.Write(tmp[:])
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
			writeItemMap(h, tmp, w.orgTreasury(o))
		}
	}

	// Containers (sorted by pos).
	if len(w.containers) > 0 {
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
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Pos.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Pos.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Pos.Z)))
			h.Write(tmp[:])
			writeItemMap(h, tmp, c.Inventory)
			writeItemMap(h, tmp, c.Reserved)
			if c.Owed != nil {
				owedAgents := make([]string, 0, len(c.Owed))
				for aid := range c.Owed {
					owedAgents = append(owedAgents, aid)
				}
				sort.Strings(owedAgents)
				for _, aid := range owedAgents {
					h.Write([]byte(aid))
					writeItemMap(h, tmp, c.Owed[aid])
				}
			}
		}
	}

	// Item entities (sorted).
	if len(w.items) > 0 {
		itemIDs := make([]string, 0, len(w.items))
		for id, e := range w.items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			itemIDs = append(itemIDs, id)
		}
		sort.Strings(itemIDs)
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(itemIDs)))
		h.Write(tmp[:])
		for _, id := range itemIDs {
			e := w.items[id]
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			h.Write([]byte(id))
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(e.Pos.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(e.Pos.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(e.Pos.Z)))
			h.Write(tmp[:])
			h.Write([]byte(e.Item))
			binary.LittleEndian.PutUint64(tmp[:], uint64(e.Count))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], e.CreatedTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], e.ExpiresTick)
			h.Write(tmp[:])
		}
	} else {
		binary.LittleEndian.PutUint64(tmp[:], 0)
		h.Write(tmp[:])
	}

	// Signs (sorted by pos).
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
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(posKeys)))
		h.Write(tmp[:])
		for _, p := range posKeys {
			s := w.signs[p]
			if s == nil {
				continue
			}
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.Z)))
			h.Write(tmp[:])
			h.Write([]byte(s.Text))
			binary.LittleEndian.PutUint64(tmp[:], s.UpdatedTick)
			h.Write(tmp[:])
			h.Write([]byte(s.UpdatedBy))
		}
	} else {
		binary.LittleEndian.PutUint64(tmp[:], 0)
		h.Write(tmp[:])
	}

	// Conveyors (sorted by pos).
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
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(posKeys)))
		h.Write(tmp[:])
		for _, p := range posKeys {
			m := w.conveyors[p]
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.Z)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(m.DX)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(m.DZ)))
			h.Write(tmp[:])
		}
	} else {
		binary.LittleEndian.PutUint64(tmp[:], 0)
		h.Write(tmp[:])
	}

	// Switches (sorted by pos).
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
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(posKeys)))
		h.Write(tmp[:])
		for _, p := range posKeys {
			on := w.switches[p]
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(p.Z)))
			h.Write(tmp[:])
			h.Write([]byte{boolByte(on)})
		}
	} else {
		binary.LittleEndian.PutUint64(tmp[:], 0)
		h.Write(tmp[:])
	}

	// Contracts (sorted).
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
		binary.LittleEndian.PutUint64(tmp[:], c.CreatedTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], c.DeadlineTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.TerminalPos.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.TerminalPos.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.TerminalPos.Z)))
		h.Write(tmp[:])
		writeItemMap(h, tmp, c.Requirements)
		writeItemMap(h, tmp, c.Reward)
		writeItemMap(h, tmp, c.Deposit)
		h.Write([]byte(c.BlueprintID))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.Rotation))
		h.Write(tmp[:])
	}

	// Trades (sorted).
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
		writeItemMap(h, tmp, tr.Offer)
		writeItemMap(h, tmp, tr.Request)
	}

	// Boards (sorted).
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
			binary.LittleEndian.PutUint64(tmp[:], p.Tick)
			h.Write(tmp[:])
		}
	}

	// Structures (sorted).
	structIDs := make([]string, 0, len(w.structures))
	for id := range w.structures {
		structIDs = append(structIDs, id)
	}
	sort.Strings(structIDs)
	binary.LittleEndian.PutUint64(tmp[:], uint64(len(structIDs)))
	h.Write(tmp[:])
	for _, id := range structIDs {
		s := w.structures[id]
		if s == nil {
			continue
		}
		h.Write([]byte(s.StructureID))
		h.Write([]byte(s.BlueprintID))
		h.Write([]byte(s.BuilderID))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Anchor.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Anchor.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Anchor.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(s.Rotation))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Min.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Min.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Min.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Max.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Max.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Max.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], s.CompletedTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], s.AwardDueTick)
		h.Write(tmp[:])
		h.Write([]byte{boolByte(s.Awarded)})
		binary.LittleEndian.PutUint64(tmp[:], uint64(s.LastInfluenceDay))
		h.Write(tmp[:])

		if len(s.UsedBy) > 0 {
			usedIDs := make([]string, 0, len(s.UsedBy))
			for aid := range s.UsedBy {
				usedIDs = append(usedIDs, aid)
			}
			sort.Strings(usedIDs)
			binary.LittleEndian.PutUint64(tmp[:], uint64(len(usedIDs)))
			h.Write(tmp[:])
			for _, aid := range usedIDs {
				h.Write([]byte(aid))
				binary.LittleEndian.PutUint64(tmp[:], s.UsedBy[aid])
				h.Write(tmp[:])
			}
		} else {
			binary.LittleEndian.PutUint64(tmp[:], 0)
			h.Write(tmp[:])
		}
	}

	// Agents (sorted).
	agents := w.sortedAgents()
	for _, a := range agents {
		h.Write([]byte(a.ID))
		h.Write([]byte(a.Name))
		h.Write([]byte(a.OrgID))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Pos.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Pos.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Pos.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Yaw)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.HP))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Hunger))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.StaminaMilli))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepTrade))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepBuild))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepSocial))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepLaw))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Novelty))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Creation))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Social))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Influence))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Narrative))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.RiskRescue))
		h.Write(tmp[:])

		// Fun novelty memory (seen biome/recipes/events) and anti-exploit state.
		biomes := make([]string, 0, len(a.seenBiomes))
		for b, ok := range a.seenBiomes {
			if ok {
				biomes = append(biomes, b)
			}
		}
		sort.Strings(biomes)
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(biomes)))
		h.Write(tmp[:])
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
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(recipes)))
		h.Write(tmp[:])
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
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(events)))
		h.Write(tmp[:])
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
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(decayKeys)))
		h.Write(tmp[:])
		for _, k := range decayKeys {
			dw := a.funDecay[k]
			h.Write([]byte(k))
			binary.LittleEndian.PutUint64(tmp[:], dw.StartTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(dw.Count))
			h.Write(tmp[:])
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
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.Target.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.Target.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.Target.Z)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(mt.Tolerance))
			h.Write(tmp[:])
			h.Write([]byte(mt.TargetID))
			binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(mt.Distance))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.StartPos.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.StartPos.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.StartPos.Z)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], mt.StartedTick)
			h.Write(tmp[:])
		}
		h.Write([]byte{boolByte(a.WorkTask != nil)})
		if a.WorkTask != nil {
			wt := a.WorkTask
			h.Write([]byte(wt.TaskID))
			h.Write([]byte(string(wt.Kind)))
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.BlockPos.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.BlockPos.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.BlockPos.Z)))
			h.Write(tmp[:])
			h.Write([]byte(wt.RecipeID))
			h.Write([]byte(wt.ItemID))
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.Count))
			h.Write(tmp[:])
			h.Write([]byte(wt.BlueprintID))
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.Anchor.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.Anchor.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.Anchor.Z)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.Rotation))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.BuildIndex))
			h.Write(tmp[:])
			h.Write([]byte(wt.TargetID))
			h.Write([]byte(wt.SrcContainer))
			h.Write([]byte(wt.DstContainer))
			binary.LittleEndian.PutUint64(tmp[:], wt.StartedTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.WorkTicks))
			h.Write(tmp[:])
		}

		// Inventory (sorted).
		inv := a.InventoryList()
		for _, it := range inv {
			h.Write([]byte(it.Item))
			binary.LittleEndian.PutUint64(tmp[:], uint64(it.Count))
			h.Write(tmp[:])
		}
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func writeItemMap(h hashWriter, tmp [8]byte, m map[string]int) {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		binary.LittleEndian.PutUint64(tmp[:], uint64(m[k]))
		h.Write(tmp[:])
	}
}

type hashWriter interface {
	Write(p []byte) (n int, err error)
}

func taskProgress(start, cur, target Vec3i) float64 {
	total := Manhattan(start, target)
	if total <= 0 {
		return 1
	}
	rem := Manhattan(cur, target)
	p := float64(total-rem) / float64(total)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
