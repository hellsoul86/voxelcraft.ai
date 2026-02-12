package digest

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"

	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	storepkg "voxelcraft.ai/internal/sim/world/terrain/store"
)

type ChunkDigestFn func(k storepkg.ChunkKey) [32]byte

type StateInput struct {
	NowTick uint64

	Seed int64

	Weather          string
	WeatherUntilTick uint64

	ActiveEventID     string
	ActiveEventStart  uint64
	ActiveEventEnds   uint64
	ActiveEventCenter modelpkg.Vec3i
	ActiveEventRadius int

	ChunkKeys   []storepkg.ChunkKey
	ChunkDigest ChunkDigestFn

	Claims     map[string]*modelpkg.LandClaim
	Laws       map[string]*lawspkg.Law
	Orgs       map[string]*modelpkg.Organization
	Containers map[modelpkg.Vec3i]*modelpkg.Container
	Items      map[string]*modelpkg.ItemEntity
	Signs      map[modelpkg.Vec3i]*modelpkg.Sign
	Conveyors  map[modelpkg.Vec3i]modelpkg.ConveyorMeta
	Switches   map[modelpkg.Vec3i]bool
	Contracts  map[string]*modelpkg.Contract
	Trades     map[string]*modelpkg.Trade
	Boards     map[string]*modelpkg.Board
	Structures map[string]*modelpkg.Structure
	Agents     map[string]*modelpkg.Agent
}

func StateDigest(in StateInput) string {
	h := sha256.New()
	var tmp [8]byte

	digestHeader(h, &tmp, in)
	digestChunks(h, &tmp, in)
	digestClaims(h, &tmp, in.Claims)
	digestLaws(h, &tmp, in.Laws)
	digestOrgs(h, &tmp, in.Orgs)
	digestContainers(h, &tmp, in.Containers)
	digestItems(h, &tmp, in.Items)
	digestSigns(h, &tmp, in.Signs)
	digestConveyors(h, &tmp, in.Conveyors)
	digestSwitches(h, &tmp, in.Switches)
	digestContracts(h, &tmp, in.Contracts)
	digestTrades(h, &tmp, in.Trades)
	digestBoards(h, &tmp, in.Boards)
	digestStructures(h, &tmp, in.Structures)
	digestAgents(h, &tmp, in.Agents)

	return hex.EncodeToString(h.Sum(nil))
}

type hashWriter interface {
	Write(p []byte) (n int, err error)
}

func digestWriteU64(h hashWriter, tmp *[8]byte, v uint64) {
	binary.LittleEndian.PutUint64(tmp[:], v)
	h.Write(tmp[:])
}

func digestWriteI64(h hashWriter, tmp *[8]byte, v int64) {
	digestWriteU64(h, tmp, uint64(v))
}

func digestHeader(h hashWriter, tmp *[8]byte, in StateInput) {
	digestWriteU64(h, tmp, in.NowTick)
	digestWriteU64(h, tmp, uint64(in.Seed))
	h.Write([]byte(in.Weather))
	digestWriteU64(h, tmp, in.WeatherUntilTick)
	h.Write([]byte(in.ActiveEventID))
	digestWriteU64(h, tmp, in.ActiveEventStart)
	digestWriteU64(h, tmp, in.ActiveEventEnds)
	digestWriteI64(h, tmp, int64(in.ActiveEventCenter.X))
	digestWriteI64(h, tmp, int64(in.ActiveEventCenter.Y))
	digestWriteI64(h, tmp, int64(in.ActiveEventCenter.Z))
	digestWriteU64(h, tmp, uint64(in.ActiveEventRadius))
}

func digestChunks(h hashWriter, tmp *[8]byte, in StateInput) {
	if len(in.ChunkKeys) == 0 || in.ChunkDigest == nil {
		return
	}
	for _, k := range in.ChunkKeys {
		digestWriteI64(h, tmp, int64(k.CX))
		digestWriteI64(h, tmp, int64(k.CZ))
		d := in.ChunkDigest(k)
		h.Write(d[:])
	}
}

func digestClaims(h hashWriter, tmp *[8]byte, claims map[string]*modelpkg.LandClaim) {
	landIDs := make([]string, 0, len(claims))
	for id := range claims {
		landIDs = append(landIDs, id)
	}
	sort.Strings(landIDs)
	for _, id := range landIDs {
		c := claims[id]
		h.Write([]byte(id))
		h.Write([]byte(c.Owner))
		digestWriteI64(h, tmp, int64(c.Anchor.X))
		digestWriteI64(h, tmp, int64(c.Anchor.Y))
		digestWriteI64(h, tmp, int64(c.Anchor.Z))
		digestWriteU64(h, tmp, uint64(c.Radius))
		h.Write([]byte{BoolByte(c.Flags.AllowBuild), BoolByte(c.Flags.AllowBreak), BoolByte(c.Flags.AllowDamage), BoolByte(c.Flags.AllowTrade)})
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
		h.Write([]byte{BoolByte(c.CurfewEnabled)})
		digestWriteU64(h, tmp, math.Float64bits(c.CurfewStart))
		digestWriteU64(h, tmp, math.Float64bits(c.CurfewEnd))
		h.Write([]byte{BoolByte(c.FineBreakEnabled)})
		h.Write([]byte(c.FineBreakItem))
		digestWriteU64(h, tmp, uint64(c.FineBreakPerBlock))
		h.Write([]byte{BoolByte(c.AccessPassEnabled)})
		h.Write([]byte(c.AccessTicketItem))
		digestWriteU64(h, tmp, uint64(c.AccessTicketCost))
		digestWriteU64(h, tmp, c.MaintenanceDueTick)
		digestWriteU64(h, tmp, uint64(c.MaintenanceStage))
	}
}

func digestLaws(h hashWriter, tmp *[8]byte, laws map[string]*lawspkg.Law) {
	if len(laws) == 0 {
		return
	}
	lawIDs := make([]string, 0, len(laws))
	for id := range laws {
		lawIDs = append(lawIDs, id)
	}
	sort.Strings(lawIDs)
	for _, id := range lawIDs {
		l := laws[id]
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

func digestOrgs(h hashWriter, tmp *[8]byte, orgs map[string]*modelpkg.Organization) {
	orgIDs := make([]string, 0, len(orgs))
	for id := range orgs {
		orgIDs = append(orgIDs, id)
	}
	sort.Strings(orgIDs)
	for _, id := range orgIDs {
		org := orgs[id]
		if org == nil {
			continue
		}
		h.Write([]byte(id))
		h.Write([]byte(string(org.Kind)))
		h.Write([]byte(org.Name))
		digestWriteU64(h, tmp, org.CreatedTick)

		// World-local treasury map.
		if len(org.TreasuryByWorld) > 0 {
			worldIDs := make([]string, 0, len(org.TreasuryByWorld))
			for wid := range org.TreasuryByWorld {
				worldIDs = append(worldIDs, wid)
			}
			sort.Strings(worldIDs)
			for _, wid := range worldIDs {
				h.Write([]byte(wid))
				WriteSortedNonZeroIntMap(h, tmp, org.TreasuryByWorld[wid])
			}
		}

		if len(org.Members) > 0 {
			memberIDs := make([]string, 0, len(org.Members))
			for mid := range org.Members {
				memberIDs = append(memberIDs, mid)
			}
			sort.Strings(memberIDs)
			digestWriteU64(h, tmp, uint64(len(memberIDs)))
			for _, mid := range memberIDs {
				h.Write([]byte(mid))
				h.Write([]byte(string(org.Members[mid])))
			}
		} else {
			digestWriteU64(h, tmp, 0)
		}
		digestWriteU64(h, tmp, org.MetaVersion)
	}
}

func digestContainers(h hashWriter, tmp *[8]byte, containers map[modelpkg.Vec3i]*modelpkg.Container) {
	if len(containers) == 0 {
		return
	}
	posKeys := make([]modelpkg.Vec3i, 0, len(containers))
	for p := range containers {
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
		c := containers[p]
		h.Write([]byte(c.Type))
		digestWriteI64(h, tmp, int64(c.Pos.X))
		digestWriteI64(h, tmp, int64(c.Pos.Y))
		digestWriteI64(h, tmp, int64(c.Pos.Z))
		WriteSortedNonZeroIntMap(h, tmp, c.Inventory)
		WriteSortedNonZeroIntMap(h, tmp, c.Reserved)
		if c.Owed != nil {
			owedAgents := make([]string, 0, len(c.Owed))
			for aid := range c.Owed {
				owedAgents = append(owedAgents, aid)
			}
			sort.Strings(owedAgents)
			for _, aid := range owedAgents {
				h.Write([]byte(aid))
				WriteSortedNonZeroIntMap(h, tmp, c.Owed[aid])
			}
		}
	}
}

func digestItems(h hashWriter, tmp *[8]byte, items map[string]*modelpkg.ItemEntity) {
	if len(items) > 0 {
		itemIDs := make([]string, 0, len(items))
		for id, e := range items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			itemIDs = append(itemIDs, id)
		}
		sort.Strings(itemIDs)
		digestWriteU64(h, tmp, uint64(len(itemIDs)))
		for _, id := range itemIDs {
			e := items[id]
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

func digestSigns(h hashWriter, tmp *[8]byte, signs map[modelpkg.Vec3i]*modelpkg.Sign) {
	if len(signs) > 0 {
		posKeys := make([]modelpkg.Vec3i, 0, len(signs))
		for p, s := range signs {
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
			s := signs[p]
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

func digestConveyors(h hashWriter, tmp *[8]byte, conveyors map[modelpkg.Vec3i]modelpkg.ConveyorMeta) {
	if len(conveyors) > 0 {
		posKeys := make([]modelpkg.Vec3i, 0, len(conveyors))
		for p := range conveyors {
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
			m := conveyors[p]
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

func digestSwitches(h hashWriter, tmp *[8]byte, switches map[modelpkg.Vec3i]bool) {
	if len(switches) > 0 {
		posKeys := make([]modelpkg.Vec3i, 0, len(switches))
		for p := range switches {
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
			on := switches[p]
			digestWriteI64(h, tmp, int64(p.X))
			digestWriteI64(h, tmp, int64(p.Y))
			digestWriteI64(h, tmp, int64(p.Z))
			h.Write([]byte{BoolByte(on)})
		}
		return
	}
	digestWriteU64(h, tmp, 0)
}

func digestContracts(h hashWriter, tmp *[8]byte, contracts map[string]*modelpkg.Contract) {
	contractIDs := make([]string, 0, len(contracts))
	for id := range contracts {
		contractIDs = append(contractIDs, id)
	}
	sort.Strings(contractIDs)
	for _, id := range contractIDs {
		c := contracts[id]
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
		WriteSortedNonZeroIntMap(h, tmp, c.Requirements)
		WriteSortedNonZeroIntMap(h, tmp, c.Reward)
		WriteSortedNonZeroIntMap(h, tmp, c.Deposit)
		h.Write([]byte(c.BlueprintID))
		digestWriteI64(h, tmp, int64(c.Anchor.X))
		digestWriteI64(h, tmp, int64(c.Anchor.Y))
		digestWriteI64(h, tmp, int64(c.Anchor.Z))
		digestWriteU64(h, tmp, uint64(c.Rotation))
	}
}

func digestTrades(h hashWriter, tmp *[8]byte, trades map[string]*modelpkg.Trade) {
	tradeIDs := make([]string, 0, len(trades))
	for id := range trades {
		tradeIDs = append(tradeIDs, id)
	}
	sort.Strings(tradeIDs)
	for _, id := range tradeIDs {
		tr := trades[id]
		h.Write([]byte(id))
		h.Write([]byte(tr.From))
		h.Write([]byte(tr.To))
		WriteSortedNonZeroIntMap(h, tmp, tr.Offer)
		WriteSortedNonZeroIntMap(h, tmp, tr.Request)
	}
}

func digestBoards(h hashWriter, tmp *[8]byte, boards map[string]*modelpkg.Board) {
	boardIDs := make([]string, 0, len(boards))
	for id := range boards {
		boardIDs = append(boardIDs, id)
	}
	sort.Strings(boardIDs)
	for _, id := range boardIDs {
		b := boards[id]
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

func digestStructures(h hashWriter, tmp *[8]byte, structures map[string]*modelpkg.Structure) {
	structIDs := make([]string, 0, len(structures))
	for id := range structures {
		structIDs = append(structIDs, id)
	}
	sort.Strings(structIDs)
	digestWriteU64(h, tmp, uint64(len(structIDs)))
	for _, id := range structIDs {
		s := structures[id]
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
		h.Write([]byte{BoolByte(s.Awarded)})
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

func digestAgents(h hashWriter, tmp *[8]byte, agents map[string]*modelpkg.Agent) {
	agentIDs := make([]string, 0, len(agents))
	for id := range agents {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)
	for _, id := range agentIDs {
		a := agents[id]
		if a == nil {
			continue
		}
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
		biomes := a.SeenBiomesSorted()
		digestWriteU64(h, tmp, uint64(len(biomes)))
		for _, b := range biomes {
			h.Write([]byte(b))
		}
		recipes := a.SeenRecipesSorted()
		digestWriteU64(h, tmp, uint64(len(recipes)))
		for _, r := range recipes {
			h.Write([]byte(r))
		}
		events := a.SeenEventsSorted()
		digestWriteU64(h, tmp, uint64(len(events)))
		for _, e := range events {
			h.Write([]byte(e))
		}
		fd := a.FunDecaySnapshot()
		decayKeys := make([]string, 0, len(fd))
		for k := range fd {
			decayKeys = append(decayKeys, k)
		}
		sort.Strings(decayKeys)
		digestWriteU64(h, tmp, uint64(len(decayKeys)))
		for _, k := range decayKeys {
			dw := fd[k]
			h.Write([]byte(k))
			digestWriteU64(h, tmp, dw.StartTick)
			digestWriteU64(h, tmp, uint64(dw.Count))
		}
		h.Write([]byte(a.Equipment.MainHand))
		for i := 0; i < 4; i++ {
			h.Write([]byte(a.Equipment.Armor[i]))
		}

		// Tasks (affects future simulation state; include in digest).
		h.Write([]byte{BoolByte(a.MoveTask != nil)})
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
		h.Write([]byte{BoolByte(a.WorkTask != nil)})
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
