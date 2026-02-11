package world

import (
	"fmt"

	"voxelcraft.ai/internal/sim/tasks"
)

const (
	InstantTypeSay            = "SAY"
	InstantTypeWhisper        = "WHISPER"
	InstantTypeEat            = "EAT"
	InstantTypeSaveMemory     = "SAVE_MEMORY"
	InstantTypeLoadMemory     = "LOAD_MEMORY"
	InstantTypeOfferTrade     = "OFFER_TRADE"
	InstantTypeAcceptTrade    = "ACCEPT_TRADE"
	InstantTypeDeclineTrade   = "DECLINE_TRADE"
	InstantTypePostBoard      = "POST_BOARD"
	InstantTypeSearchBoard    = "SEARCH_BOARD"
	InstantTypeSetSign        = "SET_SIGN"
	InstantTypeToggleSwitch   = "TOGGLE_SWITCH"
	InstantTypeClaimOwed      = "CLAIM_OWED"
	InstantTypePostContract   = "POST_CONTRACT"
	InstantTypeAcceptContract = "ACCEPT_CONTRACT"
	InstantTypeSubmitContract = "SUBMIT_CONTRACT"
	InstantTypeSetPermissions = "SET_PERMISSIONS"
	InstantTypeUpgradeClaim   = "UPGRADE_CLAIM"
	InstantTypeAddMember      = "ADD_MEMBER"
	InstantTypeRemoveMember   = "REMOVE_MEMBER"
	InstantTypeCreateOrg      = "CREATE_ORG"
	InstantTypeJoinOrg        = "JOIN_ORG"
	InstantTypeOrgDeposit     = "ORG_DEPOSIT"
	InstantTypeOrgWithdraw    = "ORG_WITHDRAW"
	InstantTypeLeaveOrg       = "LEAVE_ORG"
	InstantTypeDeedLand       = "DEED_LAND"
	InstantTypeProposeLaw     = "PROPOSE_LAW"
	InstantTypeVote           = "VOTE"

	TaskTypeStop      = "STOP"
	TaskTypeClaimLand = "CLAIM_LAND"
)

var supportedInstantTypes = []string{
	InstantTypeSay,
	InstantTypeWhisper,
	InstantTypeEat,
	InstantTypeSaveMemory,
	InstantTypeLoadMemory,
	InstantTypeOfferTrade,
	InstantTypeAcceptTrade,
	InstantTypeDeclineTrade,
	InstantTypePostBoard,
	InstantTypeSearchBoard,
	InstantTypeSetSign,
	InstantTypeToggleSwitch,
	InstantTypeClaimOwed,
	InstantTypePostContract,
	InstantTypeAcceptContract,
	InstantTypeSubmitContract,
	InstantTypeSetPermissions,
	InstantTypeUpgradeClaim,
	InstantTypeAddMember,
	InstantTypeRemoveMember,
	InstantTypeCreateOrg,
	InstantTypeJoinOrg,
	InstantTypeOrgDeposit,
	InstantTypeOrgWithdraw,
	InstantTypeLeaveOrg,
	InstantTypeDeedLand,
	InstantTypeProposeLaw,
	InstantTypeVote,
}

var supportedTaskReqTypes = []string{
	TaskTypeStop,
	string(tasks.KindMoveTo),
	string(tasks.KindFollow),
	string(tasks.KindMine),
	string(tasks.KindGather),
	string(tasks.KindPlace),
	string(tasks.KindOpen),
	string(tasks.KindTransfer),
	string(tasks.KindCraft),
	string(tasks.KindSmelt),
	TaskTypeClaimLand,
	string(tasks.KindBuildBlueprint),
}

func validateActionDispatchMaps() error {
	if err := validateDispatchMap("instantDispatch", instantDispatch, supportedInstantTypes); err != nil {
		return err
	}
	if err := validateDispatchMap("taskReqDispatch", taskReqDispatch, supportedTaskReqTypes); err != nil {
		return err
	}
	return nil
}

func validateDispatchMap[T any](name string, handlers map[string]T, supported []string) error {
	allowed := make(map[string]struct{}, len(supported))
	for _, k := range supported {
		if k == "" {
			return fmt.Errorf("%s: empty supported key", name)
		}
		if _, ok := allowed[k]; ok {
			return fmt.Errorf("%s: duplicate supported key %q", name, k)
		}
		allowed[k] = struct{}{}
	}
	if len(handlers) != len(allowed) {
		return fmt.Errorf("%s size mismatch: got=%d want=%d", name, len(handlers), len(allowed))
	}
	for k := range handlers {
		if _, ok := allowed[k]; !ok {
			return fmt.Errorf("%s has unsupported key %q", name, k)
		}
	}
	for k := range allowed {
		if _, ok := handlers[k]; !ok {
			return fmt.Errorf("%s missing key %q", name, k)
		}
	}
	return nil
}
