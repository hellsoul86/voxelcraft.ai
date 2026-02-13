package workexec

import (
	"voxelcraft.ai/internal/sim/catalogs"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type Env struct {
	GetContainerByIDFn             func(id string) *modelpkg.Container
	ParseContainerIDFn             func(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	BlockNameAtFn                  func(pos modelpkg.Vec3i) string
	EnsureBoardFn                  func(pos modelpkg.Vec3i) *modelpkg.Board
	BoardIDAtFn                    func(pos modelpkg.Vec3i) string
	GetSignFn                      func(pos modelpkg.Vec3i) *modelpkg.Sign
	SignIDAtFn                     func(pos modelpkg.Vec3i) string
	ContractSummariesForTerminalFn func(pos modelpkg.Vec3i) []map[string]interface{}
	OnContainerOpenedDuringEventFn func(a *modelpkg.Agent, c *modelpkg.Container, nowTick uint64)
	CanWithdrawFromContainerFn     func(agentID string, pos modelpkg.Vec3i) bool
	AuditTransferFn                func(nowTick uint64, actorID string, at modelpkg.Vec3i, srcID string, dstID string, item string, count int)

	GetRecipeFn             func(recipeID string) (catalogs.RecipeDef, bool)
	GetBlueprintFn          func(blueprintID string) (catalogs.BlueprintDef, bool)
	GetSmeltRecipeByInputFn func(itemID string) (catalogs.RecipeDef, bool)
	NearBlockFn             func(pos modelpkg.Vec3i, blockID string, dist int) bool
	OnRecipeFn              func(a *modelpkg.Agent, recipeID string, tier int, nowTick uint64)

	GetItemEntityFn       func(id string) *modelpkg.ItemEntity
	CanPickupItemEntityFn func(agentID string, pos modelpkg.Vec3i) bool
	RemoveItemEntityFn    func(nowTick uint64, actor string, id string, reason string)

	InBoundsFn                      func(pos modelpkg.Vec3i) bool
	CanBuildAtFn                    func(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	RecordDeniedFn                  func(nowTick uint64)
	BumpRepLawFn                    func(agentID string, delta int)
	BlockAtFn                       func(pos modelpkg.Vec3i) uint16
	AirBlockIDFn                    func() uint16
	ItemPlaceAsFn                   func(itemID string) (string, bool)
	BlockIDByNameFn                 func(blockName string) (uint16, bool)
	SetBlockFn                      func(pos modelpkg.Vec3i, blockID uint16)
	AuditSetBlockFn                 func(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string)
	EnsureContainerForPlacedBlockFn func(pos modelpkg.Vec3i, blockName string)
	EnsureBlueprintMaterialsFn      func(a *modelpkg.Agent, anchor modelpkg.Vec3i, needCost []catalogs.ItemCount, nowTick uint64) (bool, string)
	EnsureConveyorFromYawFn         func(pos modelpkg.Vec3i, yaw int)

	CanBreakAtFn          func(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	PermissionsForFn      func(agentID string, pos modelpkg.Vec3i) (*modelpkg.LandClaim, map[string]bool)
	IsLandMemberFn        func(agentID string, land *modelpkg.LandClaim) bool
	TransferToLandOwnerFn func(ownerID string, item string, count int)

	BlockNameFn               func(blockID uint16) string
	BlockIDToItemFn           func(blockID uint16) string
	SpawnItemEntityFn         func(nowTick uint64, actor string, pos modelpkg.Vec3i, item string, count int, reason string) string
	GetContainerAtFn          func(pos modelpkg.Vec3i) *modelpkg.Container
	RemoveContainerFn         func(pos modelpkg.Vec3i)
	RemoveBoardFn             func(pos modelpkg.Vec3i)
	RemoveSignFn              func(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)
	RemoveConveyorFn          func(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)
	RemoveSwitchFn            func(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)
	RemoveClaimByAnchorFn     func(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)
	OnMinedBlockDuringEventFn func(a *modelpkg.Agent, pos modelpkg.Vec3i, blockName string, nowTick uint64)
}

func (e Env) GetContainerByID(id string) *modelpkg.Container {
	if e.GetContainerByIDFn == nil {
		return nil
	}
	return e.GetContainerByIDFn(id)
}

func (e Env) ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool) {
	if e.ParseContainerIDFn == nil {
		return "", modelpkg.Vec3i{}, false
	}
	return e.ParseContainerIDFn(id)
}

func (e Env) BlockNameAt(pos modelpkg.Vec3i) string {
	if e.BlockNameAtFn == nil {
		return ""
	}
	return e.BlockNameAtFn(pos)
}

func (e Env) EnsureBoard(pos modelpkg.Vec3i) *modelpkg.Board {
	if e.EnsureBoardFn == nil {
		return nil
	}
	return e.EnsureBoardFn(pos)
}

func (e Env) BoardIDAt(pos modelpkg.Vec3i) string {
	if e.BoardIDAtFn == nil {
		return ""
	}
	return e.BoardIDAtFn(pos)
}

func (e Env) GetSign(pos modelpkg.Vec3i) *modelpkg.Sign {
	if e.GetSignFn == nil {
		return nil
	}
	return e.GetSignFn(pos)
}

func (e Env) SignIDAt(pos modelpkg.Vec3i) string {
	if e.SignIDAtFn == nil {
		return ""
	}
	return e.SignIDAtFn(pos)
}

func (e Env) ContractSummariesForTerminal(pos modelpkg.Vec3i) []map[string]interface{} {
	if e.ContractSummariesForTerminalFn == nil {
		return nil
	}
	return e.ContractSummariesForTerminalFn(pos)
}

func (e Env) OnContainerOpenedDuringEvent(a *modelpkg.Agent, c *modelpkg.Container, nowTick uint64) {
	if e.OnContainerOpenedDuringEventFn != nil {
		e.OnContainerOpenedDuringEventFn(a, c, nowTick)
	}
}

func (e Env) CanWithdrawFromContainer(agentID string, pos modelpkg.Vec3i) bool {
	if e.CanWithdrawFromContainerFn == nil {
		return false
	}
	return e.CanWithdrawFromContainerFn(agentID, pos)
}

func (e Env) AuditTransfer(nowTick uint64, actorID string, at modelpkg.Vec3i, srcID string, dstID string, item string, count int) {
	if e.AuditTransferFn != nil {
		e.AuditTransferFn(nowTick, actorID, at, srcID, dstID, item, count)
	}
}

func (e Env) GetRecipe(recipeID string) (catalogs.RecipeDef, bool) {
	if e.GetRecipeFn == nil {
		return catalogs.RecipeDef{}, false
	}
	return e.GetRecipeFn(recipeID)
}

func (e Env) GetBlueprint(blueprintID string) (catalogs.BlueprintDef, bool) {
	if e.GetBlueprintFn == nil {
		return catalogs.BlueprintDef{}, false
	}
	return e.GetBlueprintFn(blueprintID)
}

func (e Env) GetSmeltRecipeByInput(itemID string) (catalogs.RecipeDef, bool) {
	if e.GetSmeltRecipeByInputFn == nil {
		return catalogs.RecipeDef{}, false
	}
	return e.GetSmeltRecipeByInputFn(itemID)
}

func (e Env) NearBlock(pos modelpkg.Vec3i, blockID string, dist int) bool {
	if e.NearBlockFn == nil {
		return false
	}
	return e.NearBlockFn(pos, blockID, dist)
}

func (e Env) OnRecipe(a *modelpkg.Agent, recipeID string, tier int, nowTick uint64) {
	if e.OnRecipeFn != nil {
		e.OnRecipeFn(a, recipeID, tier, nowTick)
	}
}

func (e Env) GetItemEntity(id string) *modelpkg.ItemEntity {
	if e.GetItemEntityFn == nil {
		return nil
	}
	return e.GetItemEntityFn(id)
}

func (e Env) CanPickupItemEntity(agentID string, pos modelpkg.Vec3i) bool {
	if e.CanPickupItemEntityFn == nil {
		return false
	}
	return e.CanPickupItemEntityFn(agentID, pos)
}

func (e Env) RemoveItemEntity(nowTick uint64, actor string, id string, reason string) {
	if e.RemoveItemEntityFn != nil {
		e.RemoveItemEntityFn(nowTick, actor, id, reason)
	}
}

func (e Env) InBounds(pos modelpkg.Vec3i) bool {
	if e.InBoundsFn == nil {
		return false
	}
	return e.InBoundsFn(pos)
}

func (e Env) CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool {
	if e.CanBuildAtFn == nil {
		return false
	}
	return e.CanBuildAtFn(agentID, pos, nowTick)
}

func (e Env) RecordDenied(nowTick uint64) {
	if e.RecordDeniedFn != nil {
		e.RecordDeniedFn(nowTick)
	}
}

func (e Env) BumpRepLaw(agentID string, delta int) {
	if e.BumpRepLawFn != nil {
		e.BumpRepLawFn(agentID, delta)
	}
}

func (e Env) BlockAt(pos modelpkg.Vec3i) uint16 {
	if e.BlockAtFn == nil {
		return 0
	}
	return e.BlockAtFn(pos)
}

func (e Env) AirBlockID() uint16 {
	if e.AirBlockIDFn == nil {
		return 0
	}
	return e.AirBlockIDFn()
}

func (e Env) ItemPlaceAs(itemID string) (string, bool) {
	if e.ItemPlaceAsFn == nil {
		return "", false
	}
	return e.ItemPlaceAsFn(itemID)
}

func (e Env) BlockIDByName(blockName string) (uint16, bool) {
	if e.BlockIDByNameFn == nil {
		return 0, false
	}
	return e.BlockIDByNameFn(blockName)
}

func (e Env) SetBlock(pos modelpkg.Vec3i, blockID uint16) {
	if e.SetBlockFn != nil {
		e.SetBlockFn(pos, blockID)
	}
}

func (e Env) AuditSetBlock(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string) {
	if e.AuditSetBlockFn != nil {
		e.AuditSetBlockFn(nowTick, actor, pos, from, to, reason)
	}
}

func (e Env) EnsureContainerForPlacedBlock(pos modelpkg.Vec3i, blockName string) {
	if e.EnsureContainerForPlacedBlockFn != nil {
		e.EnsureContainerForPlacedBlockFn(pos, blockName)
	}
}

func (e Env) EnsureBlueprintMaterials(a *modelpkg.Agent, anchor modelpkg.Vec3i, needCost []catalogs.ItemCount, nowTick uint64) (bool, string) {
	if e.EnsureBlueprintMaterialsFn == nil {
		return false, "materials unavailable"
	}
	return e.EnsureBlueprintMaterialsFn(a, anchor, needCost, nowTick)
}

func (e Env) EnsureConveyorFromYaw(pos modelpkg.Vec3i, yaw int) {
	if e.EnsureConveyorFromYawFn != nil {
		e.EnsureConveyorFromYawFn(pos, yaw)
	}
}

func (e Env) CanBreakAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool {
	if e.CanBreakAtFn == nil {
		return false
	}
	return e.CanBreakAtFn(agentID, pos, nowTick)
}

func (e Env) PermissionsFor(agentID string, pos modelpkg.Vec3i) (*modelpkg.LandClaim, map[string]bool) {
	if e.PermissionsForFn == nil {
		return nil, nil
	}
	return e.PermissionsForFn(agentID, pos)
}

func (e Env) IsLandMember(agentID string, land *modelpkg.LandClaim) bool {
	if e.IsLandMemberFn == nil {
		return false
	}
	return e.IsLandMemberFn(agentID, land)
}

func (e Env) TransferToLandOwner(ownerID string, item string, count int) {
	if e.TransferToLandOwnerFn != nil {
		e.TransferToLandOwnerFn(ownerID, item, count)
	}
}

func (e Env) BlockName(blockID uint16) string {
	if e.BlockNameFn == nil {
		return ""
	}
	return e.BlockNameFn(blockID)
}

func (e Env) BlockIDToItem(blockID uint16) string {
	if e.BlockIDToItemFn == nil {
		return ""
	}
	return e.BlockIDToItemFn(blockID)
}

func (e Env) SpawnItemEntity(nowTick uint64, actor string, pos modelpkg.Vec3i, item string, count int, reason string) string {
	if e.SpawnItemEntityFn == nil {
		return ""
	}
	return e.SpawnItemEntityFn(nowTick, actor, pos, item, count, reason)
}

func (e Env) GetContainerAt(pos modelpkg.Vec3i) *modelpkg.Container {
	if e.GetContainerAtFn == nil {
		return nil
	}
	return e.GetContainerAtFn(pos)
}

func (e Env) RemoveContainer(pos modelpkg.Vec3i) {
	if e.RemoveContainerFn != nil {
		e.RemoveContainerFn(pos)
	}
}

func (e Env) RemoveBoard(pos modelpkg.Vec3i) {
	if e.RemoveBoardFn != nil {
		e.RemoveBoardFn(pos)
	}
}

func (e Env) RemoveSign(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string) {
	if e.RemoveSignFn != nil {
		e.RemoveSignFn(nowTick, actor, pos, reason)
	}
}

func (e Env) RemoveConveyor(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string) {
	if e.RemoveConveyorFn != nil {
		e.RemoveConveyorFn(nowTick, actor, pos, reason)
	}
}

func (e Env) RemoveSwitch(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string) {
	if e.RemoveSwitchFn != nil {
		e.RemoveSwitchFn(nowTick, actor, pos, reason)
	}
}

func (e Env) RemoveClaimByAnchor(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string) {
	if e.RemoveClaimByAnchorFn != nil {
		e.RemoveClaimByAnchorFn(nowTick, actor, pos, reason)
	}
}

func (e Env) OnMinedBlockDuringEvent(a *modelpkg.Agent, pos modelpkg.Vec3i, blockName string, nowTick uint64) {
	if e.OnMinedBlockDuringEventFn != nil {
		e.OnMinedBlockDuringEventFn(a, pos, blockName, nowTick)
	}
}
