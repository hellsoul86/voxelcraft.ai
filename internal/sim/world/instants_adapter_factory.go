package world

import (
	economyinstantspkg "voxelcraft.ai/internal/sim/world/feature/economy/instants"
	taxpkg "voxelcraft.ai/internal/sim/world/feature/economy/tax"
	governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	contractsinstctxpkg "voxelcraft.ai/internal/sim/world/featurectx/instants/contracts"
	conveyorinstctxpkg "voxelcraft.ai/internal/sim/world/featurectx/instants/conveyor"
	economyinstctxpkg "voxelcraft.ai/internal/sim/world/featurectx/instants/economy"
	governanceinstctxpkg "voxelcraft.ai/internal/sim/world/featurectx/instants/governance"
	observerpostinginstctxpkg "voxelcraft.ai/internal/sim/world/featurectx/instants/observerposting"
	sessioninstctxpkg "voxelcraft.ai/internal/sim/world/featurectx/instants/session"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func newSessionInstantsEnv(w *World) sessioninstctxpkg.Env {
	if w == nil {
		return sessioninstctxpkg.Env{}
	}
	return sessioninstctxpkg.Env{
		IsOrgMemberFn: w.isOrgMember,
		PermissionsForFn: func(agentID string, pos modelpkg.Vec3i) map[string]bool {
			_, perms := w.permissionsFor(agentID, pos)
			return perms
		},
		BroadcastChatFn: w.broadcastChat,
		AgentByIDFn: func(agentID string) *modelpkg.Agent {
			return w.agents[agentID]
		},
	}
}

func newEconomyInstantsEnv(w *World) economyinstctxpkg.Env {
	if w == nil {
		return economyinstctxpkg.Env{}
	}
	return economyinstctxpkg.Env{
		PermissionsForFn: func(agentID string, pos modelpkg.Vec3i) map[string]bool {
			_, perms := w.permissionsFor(agentID, pos)
			return perms
		},
		AgentByIDFn: func(agentID string) *modelpkg.Agent {
			return w.agents[agentID]
		},
		NewTradeIDFn: func() string {
			return economyinstantspkg.SimpleTradeIDFromCounter(w.nextTradeNum.Add(1))
		},
		PutTradeFn: func(tr *modelpkg.Trade) {
			if tr != nil {
				w.trades[tr.TradeID] = tr
			}
		},
		GetTradeFn: func(tradeID string) *modelpkg.Trade {
			return w.trades[tradeID]
		},
		DeleteTradeFn: func(tradeID string) {
			delete(w.trades, tradeID)
		},
		ResolveTradeTaxFn: func(tr *modelpkg.Trade, from *modelpkg.Agent, to *modelpkg.Agent, nowTick uint64) economyinstantspkg.TradeTaxResolution {
			if tr == nil || from == nil || to == nil {
				return economyinstantspkg.TradeTaxResolution{}
			}
			landFrom, _ := w.permissionsFor(from.ID, from.Pos)
			landTo, _ := w.permissionsFor(to.ID, to.Pos)
			res := economyinstantspkg.TradeTaxResolution{}
			if landFrom != nil && landTo != nil {
				res.Rate = taxpkg.EffectiveMarketTax(landFrom.MarketTax, landFrom.LandID == landTo.LandID, w.activeEventID, nowTick, w.activeEventEnds)
			}
			if res.Rate <= 0 || landFrom == nil || landFrom.Owner == "" {
				return res
			}
			if owner := w.agents[landFrom.Owner]; owner != nil {
				res.Sink = owner.Inventory
			} else if org := w.orgByID(landFrom.Owner); org != nil {
				res.Sink = w.orgTreasury(org)
			}
			res.LandID = landFrom.LandID
			res.TaxTo = landFrom.Owner
			return res
		},
	}
}

func newContractInstantsEnv(w *World) contractsinstctxpkg.Env {
	if w == nil {
		return contractsinstctxpkg.Env{}
	}
	return contractsinstctxpkg.Env{
		GetContainerByIDFn: w.getContainerByID,
		DistanceFn:         Manhattan,
		NewContractIDFn:    w.newContractID,
		PutContractFn: func(c *modelpkg.Contract) {
			if c != nil {
				w.contracts[c.ContractID] = c
			}
		},
		GetContractFn: func(contractID string) *modelpkg.Contract {
			return w.contracts[contractID]
		},
		RepDepositMultiplierFn: w.repDepositMultiplier,
		CheckBuildContractFn: func(c *modelpkg.Contract) bool {
			if c == nil {
				return false
			}
			buildOK := w.checkBlueprintPlaced(c.BlueprintID, c.Anchor, c.Rotation)
			if !buildOK {
				return false
			}
			bp, ok := w.catalogs.Blueprints.ByID[c.BlueprintID]
			if !ok {
				return false
			}
			return w.structureStable(&bp, c.Anchor, c.Rotation)
		},
	}
}

func newConveyorInstantsEnv(w *World) conveyorinstctxpkg.Env {
	if w == nil {
		return conveyorinstctxpkg.Env{}
	}
	return conveyorinstctxpkg.Env{
		ParseContainerIDFn: parseContainerID,
		BlockNameAtFn: func(pos modelpkg.Vec3i) string {
			return w.blockName(w.chunks.GetBlock(pos))
		},
		DistanceFn:   Manhattan,
		CanBuildAtFn: w.canBuildAt,
		SwitchStateAtFn: func(pos modelpkg.Vec3i) bool {
			if w.switches == nil {
				return false
			}
			return w.switches[pos]
		},
		SetSwitchStateFn: func(pos modelpkg.Vec3i, on bool) {
			if w.switches == nil {
				w.switches = map[Vec3i]bool{}
			}
			w.switches[pos] = on
		},
		SwitchIDAtFn: switchIDAt,
		AuditSwitchToggleFn: func(nowTick uint64, actorID string, pos modelpkg.Vec3i, switchID string, on bool) {
			w.auditEvent(nowTick, actorID, "SWITCH_TOGGLE", pos, "TOGGLE_SWITCH", map[string]any{
				"switch_id": switchID,
				"on":        on,
			})
		},
		BumpLawRepFn: w.bumpRepLaw,
		RecordDeniedFn: func(nowTick uint64) {
			if w.stats != nil {
				w.stats.RecordDenied(nowTick)
			}
		},
	}
}

func newGovernanceClaimInstantsEnv(w *World) governanceinstctxpkg.ClaimEnv {
	if w == nil {
		return governanceinstctxpkg.ClaimEnv{}
	}
	return governanceinstctxpkg.ClaimEnv{
		GetLandFn:     func(landID string) *modelpkg.LandClaim { return w.claims[landID] },
		IsLandAdminFn: w.isLandAdmin,
		BlockNameAtFn: func(pos modelpkg.Vec3i) string {
			return w.blockName(w.chunks.GetBlock(pos))
		},
		ClaimRecordsFn: func() []governanceinstantspkg.ClaimRecord {
			records := make([]governanceinstantspkg.ClaimRecord, 0, len(w.claims))
			for _, c := range w.claims {
				if c == nil {
					continue
				}
				records = append(records, governanceinstantspkg.ClaimRecord{
					LandID:  c.LandID,
					AnchorX: c.Anchor.X,
					AnchorZ: c.Anchor.Z,
					Radius:  c.Radius,
				})
			}
			return records
		},
		OwnerExistsFn: func(ownerID string) bool {
			return w.agents[ownerID] != nil || w.orgByID(ownerID) != nil
		},
		AuditClaimEventFn: w.auditEvent,
	}
}

func newGovernanceLawInstantsEnv(w *World) governanceinstctxpkg.LawEnv {
	if w == nil {
		return governanceinstctxpkg.LawEnv{}
	}
	return governanceinstctxpkg.LawEnv{
		GetLandFn:      func(landID string) *modelpkg.LandClaim { return w.claims[landID] },
		IsLandMemberFn: w.isLandMember,
		GetLawTemplateTitleFn: func(templateID string) (string, bool) {
			tmpl, ok := w.catalogs.Laws.ByID[templateID]
			if !ok {
				return "", false
			}
			return tmpl.Title, true
		},
		ItemExistsFn: func(itemID string) bool {
			_, ok := w.catalogs.Items.Defs[itemID]
			return ok
		},
		NewLawIDFn: w.newLawID,
		PutLawFn: func(law *lawspkg.Law) {
			if law != nil {
				w.laws[law.LawID] = law
			}
		},
		GetLawFn:            func(lawID string) *lawspkg.Law { return w.laws[lawID] },
		BroadcastLawEventFn: w.broadcastLawEvent,
		AuditLawEventFn:     w.auditEvent,
	}
}

func newGovernanceOrgInstantsEnv(w *World) governanceinstctxpkg.OrgEnv {
	if w == nil {
		return governanceinstctxpkg.OrgEnv{}
	}
	return governanceinstctxpkg.OrgEnv{
		NewOrgIDFn: w.newOrgID,
		GetOrgFn:   w.orgByID,
		PutOrgFn: func(org *modelpkg.Organization) {
			if org != nil {
				w.orgs[org.OrgID] = org
			}
		},
		DeleteOrgFn: func(orgID string) { delete(w.orgs, orgID) },
		OrgTreasuryFn: func(org *modelpkg.Organization) map[string]int {
			return w.orgTreasury(org)
		},
		IsOrgMemberFn: w.isOrgMember,
		IsOrgAdminFn:  w.isOrgAdmin,
		AuditOrgEventFn: func(nowTick uint64, actorID string, action string, reason string, details map[string]any) {
			pos := modelpkg.Vec3i{}
			if a := w.agents[actorID]; a != nil {
				pos = a.Pos
			}
			w.auditEvent(nowTick, actorID, action, pos, reason, details)
		},
	}
}

func newObserverPostingEnv(w *World) observerpostinginstctxpkg.Env {
	if w == nil {
		return observerpostinginstctxpkg.Env{}
	}
	return observerpostinginstctxpkg.Env{
		ParseContainerIDFn: parseContainerID,
		CanonicalBoardIDFn: boardIDAt,
		BlockNameAtFn: func(pos modelpkg.Vec3i) string {
			return w.blockName(w.chunks.GetBlock(pos))
		},
		DistanceFn: Manhattan,
		PostingAllowedFn: func(agentID string, pos modelpkg.Vec3i) bool {
			land := w.landAt(pos)
			return land == nil || w.isLandMember(agentID, land) || land.Flags.AllowTrade
		},
		GetBoardFn:    func(boardID string) *modelpkg.Board { return w.boards[boardID] },
		EnsureBoardFn: w.ensureBoard,
		PutBoardFn: func(boardID string, board *modelpkg.Board) {
			if board != nil {
				w.boards[boardID] = board
			}
		},
		NewPostIDFn: w.newPostID,
		AuditBoardPostFn: func(nowTick uint64, actorID string, pos modelpkg.Vec3i, boardID string, postID string, title string) {
			w.auditEvent(nowTick, actorID, "BOARD_POST", pos, "POST_BOARD", map[string]any{
				"board_id": boardID,
				"post_id":  postID,
				"title":    title,
			})
		},
		CanBuildAtFn: w.canBuildAt,
		EnsureSignFn: w.ensureSign,
		SignIDAtFn:   signIDAt,
		AuditSignSetFn: func(nowTick uint64, actorID string, pos modelpkg.Vec3i, signID string, text string) {
			w.auditEvent(nowTick, actorID, "SIGN_SET", pos, "SET_SIGN", map[string]any{
				"sign_id":     signID,
				"text":        text,
				"text_length": len(text),
			})
		},
		BumpLawRepFn: w.bumpRepLaw,
		RecordDeniedFn: func(nowTick uint64) {
			if w.stats != nil {
				w.stats.RecordDenied(nowTick)
			}
		},
	}
}
