package world

import (
	"voxelcraft.ai/internal/protocol"
	auditpkg "voxelcraft.ai/internal/sim/world/feature/contracts/audit"
	contractinstantspkg "voxelcraft.ai/internal/sim/world/feature/contracts/instants"
	conveyorinstantspkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
)

func handleInstantPostContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	contractinstantspkg.HandlePostContract(
		newContractInstantsEnv(w),
		actionResult,
		contractinstantspkg.ContractLifecycleHooks{
			OnPosted: func(out contractinstantspkg.ContractPostOutcome) {
				c := out.Contract
				term := out.Terminal
				w.auditEvent(nowTick, a.ID, "CONTRACT_POST", term.Pos, "POST_CONTRACT", auditpkg.BuildPostAuditFields(
					c.ContractID,
					term.ID(),
					c.Kind,
					inventorypkg.EncodeItemPairs(c.Requirements),
					inventorypkg.EncodeItemPairs(c.Reward),
					inventorypkg.EncodeItemPairs(c.Deposit),
					c.DeadlineTick,
					c.BlueprintID,
					c.Anchor.ToArray(),
					c.Rotation,
				))
			},
		},
		a,
		inst,
		nowTick,
		w.cfg.DayTicks,
	)
}

func handleInstantAcceptContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	contractinstantspkg.HandleAcceptContract(
		newContractInstantsEnv(w),
		actionResult,
		contractinstantspkg.ContractLifecycleHooks{
			OnAccepted: func(out contractinstantspkg.ContractAcceptOutcome) {
				c := out.Contract
				term := out.Terminal
				w.auditEvent(nowTick, a.ID, "CONTRACT_ACCEPT", term.Pos, "ACCEPT_CONTRACT",
					auditpkg.BuildAcceptAuditFields(c.ContractID, term.ID(), c.Kind, c.Poster, c.Acceptor, inventorypkg.EncodeItemPairs(c.Deposit)))
			},
		},
		a,
		inst,
		nowTick,
	)
}

func handleInstantSubmitContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	contractinstantspkg.HandleSubmitContract(
		newContractInstantsEnv(w),
		actionResult,
		contractinstantspkg.ContractLifecycleHooks{
			OnSubmitted: func(out contractinstantspkg.ContractSubmitOutcome) {
				c := out.Contract
				term := out.Terminal
				switch c.Kind {
				case "GATHER", "DELIVER":
					w.addTradeCredit(nowTick, a.ID, c.Poster, c.Kind)
				case "BUILD":
					w.addBuildCredit(nowTick, a.ID, c.Poster, c.Kind)
				}
				w.auditEvent(nowTick, a.ID, "CONTRACT_COMPLETE", term.Pos, "SUBMIT_CONTRACT",
					auditpkg.BuildSubmitAuditFields(c.ContractID, term.ID(), c.Kind, c.Poster, c.Acceptor))
			},
		},
		a,
		inst,
		nowTick,
	)
}

func handleInstantToggleSwitch(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	conveyorinstantspkg.HandleToggleSwitch(
		newConveyorInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantClaimOwed(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	contractinstantspkg.HandleClaimOwed(
		newContractInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}
