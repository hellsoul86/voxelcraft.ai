package world

import (
	"voxelcraft.ai/internal/protocol"
	governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
)

func handleInstantSetPermissions(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleSetPermissions(
		newGovernanceClaimInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantUpgradeClaim(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleUpgradeClaim(
		newGovernanceClaimInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantAddMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleAddMember(
		newGovernanceClaimInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantRemoveMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleRemoveMember(
		newGovernanceClaimInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantDeedLand(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleDeedLand(
		newGovernanceClaimInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantProposeLaw(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleProposeLaw(
		newGovernanceLawInstantsEnv(w),
		actionResult,
		governanceinstantspkg.LawInstantHooks{
			OnProposed: func(law *lawspkg.Law) {
				if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
					w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
					w.addFun(a, nowTick, "NARRATIVE", "civic_vote_propose", a.FunDecayDelta("narrative:civic_vote_propose", 6, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
					a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "PROPOSE_LAW", "law_id": law.LawID})
				}
			},
		},
		a,
		inst,
		nowTick,
		w.cfg.AllowLaws,
		w.cfg.LawNoticeTicks,
		w.cfg.LawVoteTicks,
	)
}

func handleInstantVote(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleVoteLaw(
		newGovernanceLawInstantsEnv(w),
		actionResult,
		governanceinstantspkg.LawInstantHooks{
			OnVoted: func(law *lawspkg.Law, _ string) {
				w.funOnVote(a, nowTick)
				if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
					a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "VOTE", "law_id": law.LawID})
				}
			},
		},
		a,
		inst,
		nowTick,
		w.cfg.AllowLaws,
	)
}

func handleInstantCreateOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleCreateOrg(
		newGovernanceOrgInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantJoinOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleJoinOrg(
		newGovernanceOrgInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantOrgDeposit(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleOrgDeposit(
		newGovernanceOrgInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantOrgWithdraw(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleOrgWithdraw(
		newGovernanceOrgInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantLeaveOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleLeaveOrg(
		newGovernanceOrgInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}
