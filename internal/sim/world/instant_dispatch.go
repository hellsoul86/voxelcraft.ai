package world

import "voxelcraft.ai/internal/protocol"

type instantHandler func(*World, *Agent, protocol.InstantReq, uint64)

var instantDispatch = map[string]instantHandler{
	InstantTypeSay:     handleInstantSay,
	InstantTypeWhisper: handleInstantWhisper,
	InstantTypeEat:     handleInstantEat,
	InstantTypeSaveMemory: func(_ *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
		handleInstantSaveMemory(a, inst, nowTick)
	},
	InstantTypeLoadMemory: func(_ *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
		handleInstantLoadMemory(a, inst, nowTick)
	},
	InstantTypeOfferTrade:     handleInstantOfferTrade,
	InstantTypeAcceptTrade:    handleInstantAcceptTrade,
	InstantTypeDeclineTrade:   handleInstantDeclineTrade,
	InstantTypePostBoard:      handleInstantPostBoard,
	InstantTypeSearchBoard:    handleInstantSearchBoard,
	InstantTypeSetSign:        handleInstantSetSign,
	InstantTypeToggleSwitch:   handleInstantToggleSwitch,
	InstantTypeClaimOwed:      handleInstantClaimOwed,
	InstantTypePostContract:   handleInstantPostContract,
	InstantTypeAcceptContract: handleInstantAcceptContract,
	InstantTypeSubmitContract: handleInstantSubmitContract,
	InstantTypeSetPermissions: handleInstantSetPermissions,
	InstantTypeUpgradeClaim:   handleInstantUpgradeClaim,
	InstantTypeAddMember:      handleInstantAddMember,
	InstantTypeRemoveMember:   handleInstantRemoveMember,
	InstantTypeCreateOrg:      handleInstantCreateOrg,
	InstantTypeJoinOrg:        handleInstantJoinOrg,
	InstantTypeOrgDeposit:     handleInstantOrgDeposit,
	InstantTypeOrgWithdraw:    handleInstantOrgWithdraw,
	InstantTypeLeaveOrg:       handleInstantLeaveOrg,
	InstantTypeDeedLand:       handleInstantDeedLand,
	InstantTypeProposeLaw:     handleInstantProposeLaw,
	InstantTypeVote:           handleInstantVote,
}
