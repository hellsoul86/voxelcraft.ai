package snapshot

import (
	"fmt"
	"strings"

	snapv1 "voxelcraft.ai/internal/persistence/snapshot"
	statspkg "voxelcraft.ai/internal/sim/world/feature/director/stats"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
	"voxelcraft.ai/internal/sim/world/logic/ids"
)

func ParseUintAfterPrefix(prefix, id string) (uint64, bool) {
	return ids.ParseUintAfterPrefix(prefix, id)
}

func ParseLandNum(id string) (uint64, bool) { return ids.ParseLandNum(id) }

func MaxU64(a, b uint64) uint64 { return ids.MaxU64(a, b) }

func taskVec3i(a [3]int) tasks.Vec3i {
	return tasks.Vec3i{X: a[0], Y: a[1], Z: a[2]}
}

func ImportTrades(s snapv1.SnapshotV1) (trades map[string]*modelpkg.Trade, maxTrade uint64) {
	trades = map[string]*modelpkg.Trade{}
	for _, tr := range s.Trades {
		offer := map[string]int{}
		for item, n := range tr.Offer {
			if n > 0 {
				offer[item] = n
			}
		}
		req := map[string]int{}
		for item, n := range tr.Request {
			if n > 0 {
				req[item] = n
			}
		}
		trades[tr.TradeID] = &modelpkg.Trade{
			TradeID:     tr.TradeID,
			From:        tr.From,
			To:          tr.To,
			Offer:       offer,
			Request:     req,
			CreatedTick: tr.CreatedTick,
		}
		if n, ok := ParseUintAfterPrefix("TR", tr.TradeID); ok && n > maxTrade {
			maxTrade = n
		}
	}
	return trades, maxTrade
}

func ImportBoards(s snapv1.SnapshotV1) (boards map[string]*modelpkg.Board, maxPost uint64) {
	boards = map[string]*modelpkg.Board{}
	for _, b := range s.Boards {
		bb := &modelpkg.Board{BoardID: b.BoardID}
		for _, p := range b.Posts {
			bb.Posts = append(bb.Posts, modelpkg.BoardPost{
				PostID: p.PostID,
				Author: p.Author,
				Title:  p.Title,
				Body:   p.Body,
				Tick:   p.Tick,
			})
			if n, ok := ParseUintAfterPrefix("P", p.PostID); ok && n > maxPost {
				maxPost = n
			}
		}
		boards[bb.BoardID] = bb
	}
	return boards, maxPost
}

func ImportAgents(s snapv1.SnapshotV1) (agents map[string]*modelpkg.Agent, maxAgent uint64, maxTask uint64) {
	agents = map[string]*modelpkg.Agent{}
	for _, a := range s.Agents {
		aa := &modelpkg.Agent{
			ID:                           a.ID,
			Name:                         a.Name,
			OrgID:                        a.OrgID,
			CurrentWorldID:               a.CurrentWorldID,
			WorldSwitchCooldownUntilTick: a.WorldSwitchCooldownUntilTick,
			Pos:                          modelpkg.Vec3i{X: a.Pos[0], Y: a.Pos[1], Z: a.Pos[2]},
			Yaw:                          a.Yaw,
			HP:                           a.HP,
			Hunger:                       a.Hunger,
			StaminaMilli:                 a.StaminaMilli,
			RepTrade:                     a.RepTrade,
			RepBuild:                     a.RepBuild,
			RepSocial:                    a.RepSocial,
			RepLaw:                       a.RepLaw,
			Fun: modelpkg.FunScore{
				Novelty:    a.FunNovelty,
				Creation:   a.FunCreation,
				Social:     a.FunSocial,
				Influence:  a.FunInfluence,
				Narrative:  a.FunNarrative,
				RiskRescue: a.FunRiskRescue,
			},
			Inventory: map[string]int{},
		}
		for item, n := range a.Inventory {
			if n > 0 {
				aa.Inventory[item] = n
			}
		}
		if len(a.Memory) > 0 {
			aa.Memory = map[string]modelpkg.MemoryEntry{}
			for k, e := range a.Memory {
				if k == "" {
					continue
				}
				if e.ExpiryTick != 0 && s.Header.Tick >= e.ExpiryTick {
					continue
				}
				aa.Memory[k] = modelpkg.MemoryEntry{Value: e.Value, ExpiryTick: e.ExpiryTick}
			}
			if len(aa.Memory) == 0 {
				aa.Memory = nil
			}
		}
		if len(a.RateWindows) > 0 {
			rws := map[string]modelpkg.RateWindowSnapshot{}
			for k, rw := range a.RateWindows {
				if k == "" {
					continue
				}
				if rw.Count <= 0 {
					continue
				}
				rws[k] = modelpkg.RateWindowSnapshot{StartTick: rw.StartTick, Count: rw.Count}
			}
			aa.LoadRateWindowsSnapshot(rws)
		}

			// Equipment is not persisted in snapshot v1. Keep defaults from InitDefaults().

		aa.SetSeenBiomes(a.SeenBiomes)
		aa.SetSeenRecipes(a.SeenRecipes)
		aa.SetSeenEvents(a.SeenEvents)
		if len(a.FunDecay) > 0 {
			fd := map[string]modelpkg.FunDecaySnapshot{}
			for k, v := range a.FunDecay {
				if k == "" || v.Count <= 0 {
					continue
				}
				fd[k] = modelpkg.FunDecaySnapshot{StartTick: v.StartTick, Count: v.Count}
			}
			aa.LoadFunDecaySnapshot(fd)
		}

			if a.MoveTask != nil {
				aa.MoveTask = &tasks.MovementTask{
					TaskID:      a.MoveTask.TaskID,
					Kind:        tasks.Kind(strings.ToUpper(a.MoveTask.Kind)),
					Target:      taskVec3i(a.MoveTask.Target),
					Tolerance:   a.MoveTask.Tolerance,
					TargetID:    a.MoveTask.TargetID,
					Distance:    a.MoveTask.Distance,
					StartPos:    taskVec3i(a.MoveTask.StartPos),
					StartedTick: a.MoveTask.StartedTick,
				}
				if n, ok := ParseUintAfterPrefix("T", aa.MoveTask.TaskID); ok && n > maxTask {
					maxTask = n
				}
			}
			if a.WorkTask != nil {
				aa.WorkTask = &tasks.WorkTask{
					TaskID:       a.WorkTask.TaskID,
					Kind:         tasks.Kind(strings.ToUpper(a.WorkTask.Kind)),
					BlockPos:     taskVec3i(a.WorkTask.BlockPos),
					RecipeID:     a.WorkTask.RecipeID,
					ItemID:       a.WorkTask.ItemID,
					Count:        a.WorkTask.Count,
					BlueprintID:  a.WorkTask.BlueprintID,
					Anchor:       taskVec3i(a.WorkTask.Anchor),
					Rotation:     a.WorkTask.Rotation,
					BuildIndex:   a.WorkTask.BuildIndex,
					TargetID:     a.WorkTask.TargetID,
					SrcContainer: a.WorkTask.SrcContainer,
				DstContainer: a.WorkTask.DstContainer,
				StartedTick:  a.WorkTask.StartedTick,
				WorkTicks:    a.WorkTask.WorkTicks,
			}
			if n, ok := ParseUintAfterPrefix("T", aa.WorkTask.TaskID); ok && n > maxTask {
				maxTask = n
			}
		}

			aa.InitDefaults()
			if aa.CurrentWorldID == "" {
				aa.CurrentWorldID = s.Header.WorldID
			}
			agents[aa.ID] = aa
			if n, ok := ParseUintAfterPrefix("A", aa.ID); ok && n > maxAgent {
				maxAgent = n
			}
	}
	return agents, maxAgent, maxTask
}

func ImportClaims(s snapv1.SnapshotV1) (claims map[string]*modelpkg.LandClaim, maxLand uint64) {
	claims = map[string]*modelpkg.LandClaim{}
	for _, c := range s.Claims {
		if c.LandID == "" || c.Owner == "" || c.Radius <= 0 {
			continue
		}
		flags := modelpkg.ClaimFlags{
			AllowBuild:  c.Flags.AllowBuild,
			AllowBreak:  c.Flags.AllowBreak,
			AllowDamage: c.Flags.AllowDamage,
			AllowTrade:  c.Flags.AllowTrade,
		}
		members := map[string]bool{}
		for _, aid := range c.Members {
			if aid == "" {
				continue
			}
			members[aid] = true
		}
		claimType := strings.TrimSpace(c.ClaimType)
		if claimType == "" {
			claimType = modelpkg.ClaimTypeDefault
		}
		cc := &modelpkg.LandClaim{
			LandID:    c.LandID,
			Owner:     c.Owner,
			ClaimType: claimType,
			Anchor:    modelpkg.Vec3i{X: c.Anchor[0], Y: c.Anchor[1], Z: c.Anchor[2]},
			Radius:    c.Radius,
			Flags:     flags,
			Members:   members,

			MarketTax:     c.MarketTax,
			CurfewEnabled: c.CurfewEnabled,
			CurfewStart:   c.CurfewStart,
			CurfewEnd:     c.CurfewEnd,

			FineBreakEnabled:  c.FineBreakEnabled,
			FineBreakItem:     c.FineBreakItem,
			FineBreakPerBlock: c.FineBreakPerBlock,

			AccessPassEnabled: c.AccessPassEnabled,
			AccessTicketItem:  c.AccessTicketItem,
			AccessTicketCost:  c.AccessTicketCost,

			MaintenanceDueTick: c.MaintenanceDueTick,
			MaintenanceStage:   c.MaintenanceStage,
		}
		claims[cc.LandID] = cc
		if n, ok := ParseLandNum(cc.LandID); ok && n > maxLand {
			maxLand = n
		}
	}
	return claims, maxLand
}

func ImportContainers(s snapv1.SnapshotV1) map[modelpkg.Vec3i]*modelpkg.Container {
	containers := map[modelpkg.Vec3i]*modelpkg.Container{}
	for _, c := range s.Containers {
		pos := modelpkg.Vec3i{X: c.Pos[0], Y: c.Pos[1], Z: c.Pos[2]}
		cc := &modelpkg.Container{
			Type:      c.Type,
			Pos:       pos,
			Inventory: map[string]int{},
		}
		for item, n := range c.Inventory {
			if n > 0 {
				cc.Inventory[item] = n
			}
		}
		if len(c.Reserved) > 0 {
			cc.Reserved = map[string]int{}
			for item, n := range c.Reserved {
				if n > 0 {
					cc.Reserved[item] = n
				}
			}
		}
		if len(c.Owed) > 0 {
			cc.Owed = map[string]map[string]int{}
			for aid, m := range c.Owed {
				if aid == "" || len(m) == 0 {
					continue
				}
				m2 := map[string]int{}
				for item, n := range m {
					if n > 0 {
						m2[item] = n
					}
				}
				if len(m2) > 0 {
					cc.Owed[aid] = m2
				}
			}
		}
		containers[pos] = cc
	}
	return containers
}

func ImportItems(s snapv1.SnapshotV1) (items map[string]*modelpkg.ItemEntity, itemsAt map[modelpkg.Vec3i][]string, maxItem uint64) {
	items = map[string]*modelpkg.ItemEntity{}
	itemsAt = map[modelpkg.Vec3i][]string{}
	for _, it := range s.Items {
		if it.EntityID == "" || it.Item == "" || it.Count <= 0 {
			continue
		}
		if it.ExpiresTick != 0 && s.Header.Tick >= it.ExpiresTick {
			continue
		}
		pos := modelpkg.Vec3i{X: it.Pos[0], Y: it.Pos[1], Z: it.Pos[2]}
		e := &modelpkg.ItemEntity{
			EntityID:    it.EntityID,
			Pos:         pos,
			Item:        it.Item,
			Count:       it.Count,
			CreatedTick: it.CreatedTick,
			ExpiresTick: it.ExpiresTick,
		}
		items[e.EntityID] = e
		itemsAt[pos] = append(itemsAt[pos], e.EntityID)
		if n, ok := ParseUintAfterPrefix("IT", e.EntityID); ok && n > maxItem {
			maxItem = n
		}
	}
	return items, itemsAt, maxItem
}

type BlockNameAt func(pos modelpkg.Vec3i) string

func ImportSigns(s snapv1.SnapshotV1, blockNameAt BlockNameAt) map[modelpkg.Vec3i]*modelpkg.Sign {
	out := map[modelpkg.Vec3i]*modelpkg.Sign{}
	for _, ss := range s.Signs {
		pos := modelpkg.Vec3i{X: ss.Pos[0], Y: ss.Pos[1], Z: ss.Pos[2]}
		if blockNameAt != nil && blockNameAt(pos) != "SIGN" {
			continue
		}
		out[pos] = &modelpkg.Sign{
			Pos:         pos,
			Text:        ss.Text,
			UpdatedTick: ss.UpdatedTick,
			UpdatedBy:   ss.UpdatedBy,
		}
	}
	return out
}

func ImportConveyors(s snapv1.SnapshotV1, blockNameAt BlockNameAt) map[modelpkg.Vec3i]modelpkg.ConveyorMeta {
	out := map[modelpkg.Vec3i]modelpkg.ConveyorMeta{}
	for _, cv := range s.Conveyors {
		pos := modelpkg.Vec3i{X: cv.Pos[0], Y: cv.Pos[1], Z: cv.Pos[2]}
		if blockNameAt != nil && blockNameAt(pos) != "CONVEYOR" {
			continue
		}
		out[pos] = modelpkg.ConveyorMeta{DX: int8(cv.DX), DZ: int8(cv.DZ)}
	}
	return out
}

func ImportSwitches(s snapv1.SnapshotV1, blockNameAt BlockNameAt) map[modelpkg.Vec3i]bool {
	out := map[modelpkg.Vec3i]bool{}
	for _, sv := range s.Switches {
		pos := modelpkg.Vec3i{X: sv.Pos[0], Y: sv.Pos[1], Z: sv.Pos[2]}
		if blockNameAt != nil && blockNameAt(pos) != "SWITCH" {
			continue
		}
		out[pos] = sv.On
	}
	return out
}

func ImportContracts(s snapv1.SnapshotV1) (contracts map[string]*modelpkg.Contract, maxContract uint64) {
	contracts = map[string]*modelpkg.Contract{}
	for _, cc := range s.Contracts {
		c := &modelpkg.Contract{
			ContractID:   cc.ContractID,
			TerminalPos:  modelpkg.Vec3i{X: cc.TerminalPos[0], Y: cc.TerminalPos[1], Z: cc.TerminalPos[2]},
			Poster:       cc.Poster,
			Acceptor:     cc.Acceptor,
			Kind:         cc.Kind,
			State:        modelpkg.ContractState(cc.State),
			Requirements: PositiveMap(cc.Requirements),
			Reward:       PositiveMap(cc.Reward),
			Deposit:      PositiveMap(cc.Deposit),
			BlueprintID:  cc.BlueprintID,
			Anchor:       modelpkg.Vec3i{X: cc.Anchor[0], Y: cc.Anchor[1], Z: cc.Anchor[2]},
			Rotation:     blueprint.NormalizeRotation(cc.Rotation),
			CreatedTick:  cc.CreatedTick,
			DeadlineTick: cc.DeadlineTick,
		}
		contracts[c.ContractID] = c
		if n, ok := ParseUintAfterPrefix("C", c.ContractID); ok && n > maxContract {
			maxContract = n
		}
	}
	return contracts, maxContract
}

func ImportLaws(s snapv1.SnapshotV1) (laws map[string]*lawspkg.Law, maxLaw uint64) {
	laws = map[string]*lawspkg.Law{}
	for _, ll := range s.Laws {
		params := map[string]string{}
		for k, v := range ll.Params {
			if k == "" || v == "" {
				continue
			}
			params[k] = v
		}
		votes := map[string]string{}
		for aid, v := range ll.Votes {
			if aid == "" || v == "" {
				continue
			}
			votes[aid] = v
		}
		l := &lawspkg.Law{
			LawID:         ll.LawID,
			LandID:        ll.LandID,
			TemplateID:    ll.TemplateID,
			Title:         ll.Title,
			Params:        params,
			ProposedBy:    ll.ProposedBy,
			ProposedTick:  ll.ProposedTick,
			NoticeEndsTick: ll.NoticeEndsTick,
			VoteEndsTick:   ll.VoteEndsTick,
			Status:        lawspkg.Status(ll.Status),
			Votes:         votes,
		}
		laws[l.LawID] = l
		if n, ok := ParseUintAfterPrefix("LAW", l.LawID); ok && n > maxLaw {
			maxLaw = n
		}
	}
	return laws, maxLaw
}

func ImportOrgs(s snapv1.SnapshotV1) (orgs map[string]*modelpkg.Organization, maxOrg uint64) {
	orgs = map[string]*modelpkg.Organization{}
	for _, o := range s.Orgs {
		if strings.TrimSpace(o.OrgID) == "" {
			continue
		}
		members := map[string]modelpkg.OrgRole{}
		for aid, role := range o.Members {
			if aid == "" || role == "" {
				continue
			}
			members[aid] = modelpkg.OrgRole(role)
		}
		treasuryByWorld := map[string]map[string]int{}
		for wid, m := range o.TreasuryByWorld {
			if wid == "" || len(m) == 0 {
				continue
			}
			treasuryByWorld[wid] = PositiveMap(m)
		}
		oo := &modelpkg.Organization{
			OrgID:           o.OrgID,
			Kind:            modelpkg.OrgKind(o.Kind),
			Name:            o.Name,
			CreatedTick:     o.CreatedTick,
			MetaVersion:     o.MetaVersion,
			Members:         members,
			Treasury:        PositiveMap(o.Treasury),
			TreasuryByWorld: treasuryByWorld,
		}
		orgs[oo.OrgID] = oo
		if n, ok := ParseUintAfterPrefix("ORG", oo.OrgID); ok && n > maxOrg {
			maxOrg = n
		}
	}
	return orgs, maxOrg
}

func ImportStructures(s snapv1.SnapshotV1) map[string]*modelpkg.Structure {
	out := map[string]*modelpkg.Structure{}
	for _, ss := range s.Structures {
		id := ss.StructureID
		if id == "" {
			continue
		}
		usedBy := map[string]uint64{}
		for aid, t := range ss.UsedBy {
			if aid != "" && t != 0 {
				usedBy[aid] = t
			}
		}
		out[id] = &modelpkg.Structure{
			StructureID:      id,
			BlueprintID:      ss.BlueprintID,
			BuilderID:        ss.BuilderID,
			Anchor:           modelpkg.Vec3i{X: ss.Anchor[0], Y: ss.Anchor[1], Z: ss.Anchor[2]},
			Rotation:         blueprint.NormalizeRotation(ss.Rotation),
			Min:              modelpkg.Vec3i{X: ss.Min[0], Y: ss.Min[1], Z: ss.Min[2]},
			Max:              modelpkg.Vec3i{X: ss.Max[0], Y: ss.Max[1], Z: ss.Max[2]},
			CompletedTick:    ss.CompletedTick,
			AwardDueTick:     ss.AwardDueTick,
			Awarded:          ss.Awarded,
			UsedBy:           usedBy,
			LastInfluenceDay: ss.LastInfluenceDay,
		}
	}
	return out
}

func ImportStats(s snapv1.SnapshotV1) *statspkg.WorldStats {
	if s.Stats != nil && len(s.Stats.Buckets) > 0 && s.Stats.BucketTicks > 0 {
		st := &statspkg.WorldStats{
			BucketTicks:  s.Stats.BucketTicks,
			WindowTicksV: s.Stats.WindowTicks,
			Buckets:      make([]statspkg.Bucket, len(s.Stats.Buckets)),
			CurIdx:       s.Stats.CurIdx,
			CurBase:      s.Stats.CurBase,
			SeenChunks:   map[statspkg.ChunkKey]bool{},
		}
		for i, b := range s.Stats.Buckets {
			st.Buckets[i] = statspkg.Bucket{
				Trades:             b.Trades,
				Denied:             b.Denied,
				ChunksDiscovered:   b.ChunksDiscovered,
				BlueprintsComplete: b.BlueprintsComplete,
			}
		}
		if st.CurIdx < 0 || st.CurIdx >= len(st.Buckets) {
			st.CurIdx = 0
		}
		for _, k := range s.Stats.SeenChunks {
			st.SeenChunks[statspkg.ChunkKey{CX: k.CX, CZ: k.CZ}] = true
		}
		return st
	}
	return statspkg.NewWorldStats(300, 72000)
}

func ValidateSnapshotBasics(s snapv1.SnapshotV1, expectedWorldID string) error {
	if s.Header.Version != 1 {
		return fmt.Errorf("unsupported snapshot version: %d", s.Header.Version)
	}
	if expectedWorldID != "" && s.Header.WorldID != expectedWorldID {
		return fmt.Errorf("snapshot world mismatch: got %q want %q", s.Header.WorldID, expectedWorldID)
	}
	if s.Height != 1 {
		return fmt.Errorf("snapshot height mismatch: got %d want 1", s.Height)
	}
	if s.TickRate <= 0 || s.DayTicks <= 0 {
		return fmt.Errorf("snapshot tick/day ticks invalid")
	}
	if s.ObsRadius < 0 {
		return fmt.Errorf("snapshot obs radius invalid")
	}
	return nil
}
