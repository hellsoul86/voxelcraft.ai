package world

import (
	"math"
	"sort"
)

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
