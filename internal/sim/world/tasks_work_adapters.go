package world

import (
	"voxelcraft.ai/internal/sim/catalogs"
	conveyruntimepkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
)

type workTaskReqWorldEnv struct {
	w *World
}

func (e workTaskReqWorldEnv) NewTaskID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newTaskID()
}

func (e workTaskReqWorldEnv) ItemEntityExists(entityID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.items[entityID] != nil
}

func (e workTaskReqWorldEnv) RecipeExists(recipeID string) bool {
	if e.w == nil {
		return false
	}
	_, ok := e.w.catalogs.Recipes.ByID[recipeID]
	return ok
}

func (e workTaskReqWorldEnv) SmeltExists(itemID string) bool {
	if e.w == nil {
		return false
	}
	_, ok := e.w.smeltByInput[itemID]
	return ok
}

func (e workTaskReqWorldEnv) BlueprintExists(blueprintID string) bool {
	if e.w == nil {
		return false
	}
	_, ok := e.w.catalogs.Blueprints.ByID[blueprintID]
	return ok
}

type workTaskExecWorldEnv struct {
	w *World
}

func (e workTaskExecWorldEnv) GetContainerByID(id string) *Container {
	if e.w == nil {
		return nil
	}
	return e.w.getContainerByID(id)
}

func (e workTaskExecWorldEnv) ParseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	return parseContainerID(id)
}

func (e workTaskExecWorldEnv) BlockNameAt(pos Vec3i) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(e.w.chunks.GetBlock(pos))
}

func (e workTaskExecWorldEnv) EnsureBoard(pos Vec3i) *Board {
	if e.w == nil {
		return nil
	}
	return e.w.ensureBoard(pos)
}

func (e workTaskExecWorldEnv) BoardIDAt(pos Vec3i) string {
	return boardIDAt(pos)
}

func (e workTaskExecWorldEnv) GetSign(pos Vec3i) *Sign {
	if e.w == nil {
		return nil
	}
	return e.w.signs[pos]
}

func (e workTaskExecWorldEnv) SignIDAt(pos Vec3i) string {
	return signIDAt(pos)
}

func (e workTaskExecWorldEnv) ContractSummariesForTerminal(pos Vec3i) []map[string]interface{} {
	if e.w == nil {
		return nil
	}
	return e.w.contractSummariesForTerminal(pos)
}

func (e workTaskExecWorldEnv) OnContainerOpenedDuringEvent(a *Agent, c *Container, nowTick uint64) {
	if e.w == nil {
		return
	}
	e.w.onContainerOpenedDuringEvent(a, c, nowTick)
}

func (e workTaskExecWorldEnv) CanWithdrawFromContainer(agentID string, pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.canWithdrawFromContainer(agentID, pos)
}

func (e workTaskExecWorldEnv) AuditTransfer(nowTick uint64, actorID string, at Vec3i, srcID string, dstID string, item string, count int) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, "TRANSFER", at, "TRANSFER", map[string]any{
		"src":   srcID,
		"dst":   dstID,
		"item":  item,
		"count": count,
	})
}

func (e workTaskExecWorldEnv) GetRecipe(recipeID string) (catalogs.RecipeDef, bool) {
	if e.w == nil {
		return catalogs.RecipeDef{}, false
	}
	rec, ok := e.w.catalogs.Recipes.ByID[recipeID]
	return rec, ok
}

func (e workTaskExecWorldEnv) GetBlueprint(blueprintID string) (catalogs.BlueprintDef, bool) {
	if e.w == nil {
		return catalogs.BlueprintDef{}, false
	}
	bp, ok := e.w.catalogs.Blueprints.ByID[blueprintID]
	return bp, ok
}

func (e workTaskExecWorldEnv) GetSmeltRecipeByInput(itemID string) (catalogs.RecipeDef, bool) {
	if e.w == nil {
		return catalogs.RecipeDef{}, false
	}
	rec, ok := e.w.smeltByInput[itemID]
	return rec, ok
}

func (e workTaskExecWorldEnv) NearBlock(pos Vec3i, blockID string, dist int) bool {
	if e.w == nil {
		return false
	}
	return e.w.nearBlock(pos, blockID, dist)
}

func (e workTaskExecWorldEnv) OnRecipe(a *Agent, recipeID string, tier int, nowTick uint64) {
	if e.w == nil {
		return
	}
	e.w.funOnRecipe(a, recipeID, tier, nowTick)
}

func (e workTaskExecWorldEnv) GetItemEntity(id string) *ItemEntity {
	if e.w == nil {
		return nil
	}
	return e.w.items[id]
}

func (e workTaskExecWorldEnv) CanPickupItemEntity(agentID string, pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.canPickupItemEntity(agentID, pos)
}

func (e workTaskExecWorldEnv) RemoveItemEntity(nowTick uint64, actor string, id string, reason string) {
	if e.w == nil {
		return
	}
	e.w.removeItemEntity(nowTick, actor, id, reason)
}

func (e workTaskExecWorldEnv) InBounds(pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.chunks.inBounds(pos)
}

func (e workTaskExecWorldEnv) CanBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	if e.w == nil {
		return false
	}
	return e.w.canBuildAt(agentID, pos, nowTick)
}

func (e workTaskExecWorldEnv) RecordDenied(nowTick uint64) {
	if e.w == nil || e.w.stats == nil {
		return
	}
	e.w.stats.RecordDenied(nowTick)
}

func (e workTaskExecWorldEnv) BumpRepLaw(agentID string, delta int) {
	if e.w == nil {
		return
	}
	e.w.bumpRepLaw(agentID, delta)
}

func (e workTaskExecWorldEnv) BlockAt(pos Vec3i) uint16 {
	if e.w == nil {
		return 0
	}
	return e.w.chunks.GetBlock(pos)
}

func (e workTaskExecWorldEnv) AirBlockID() uint16 {
	if e.w == nil {
		return 0
	}
	return e.w.chunks.gen.Air
}

func (e workTaskExecWorldEnv) ItemPlaceAs(itemID string) (string, bool) {
	if e.w == nil {
		return "", false
	}
	def, ok := e.w.catalogs.Items.Defs[itemID]
	if !ok {
		return "", false
	}
	return def.PlaceAs, true
}

func (e workTaskExecWorldEnv) BlockIDByName(blockName string) (uint16, bool) {
	if e.w == nil {
		return 0, false
	}
	bid, ok := e.w.catalogs.Blocks.Index[blockName]
	return bid, ok
}

func (e workTaskExecWorldEnv) SetBlock(pos Vec3i, blockID uint16) {
	if e.w == nil {
		return
	}
	e.w.chunks.SetBlock(pos, blockID)
}

func (e workTaskExecWorldEnv) AuditSetBlock(nowTick uint64, actor string, pos Vec3i, from uint16, to uint16, reason string) {
	if e.w == nil {
		return
	}
	e.w.auditSetBlock(nowTick, actor, pos, from, to, reason)
}

func (e workTaskExecWorldEnv) EnsureContainerForPlacedBlock(pos Vec3i, blockName string) {
	if e.w == nil {
		return
	}
	e.w.ensureContainerForPlacedBlock(pos, blockName)
}

func (e workTaskExecWorldEnv) EnsureBlueprintMaterials(a *Agent, anchor Vec3i, needCost []catalogs.ItemCount, nowTick uint64) (bool, string) {
	if e.w == nil {
		return false, "world unavailable"
	}
	return e.w.blueprintEnsureMaterials(a, anchor, needCost, nowTick)
}

func (e workTaskExecWorldEnv) EnsureConveyorFromYaw(pos Vec3i, yaw int) {
	if e.w == nil {
		return
	}
	dx, dz := conveyruntimepkg.YawToDir(yaw)
	e.w.ensureConveyor(pos, dx, dz)
}

func (e workTaskExecWorldEnv) CanBreakAt(agentID string, pos Vec3i, nowTick uint64) bool {
	if e.w == nil {
		return false
	}
	return e.w.canBreakAt(agentID, pos, nowTick)
}

func (e workTaskExecWorldEnv) PermissionsFor(agentID string, pos Vec3i) (*LandClaim, map[string]bool) {
	if e.w == nil {
		return nil, nil
	}
	return e.w.permissionsFor(agentID, pos)
}

func (e workTaskExecWorldEnv) IsLandMember(agentID string, land *LandClaim) bool {
	if e.w == nil {
		return false
	}
	return e.w.isLandMember(agentID, land)
}

func (e workTaskExecWorldEnv) TransferToLandOwner(ownerID string, item string, count int) {
	if e.w == nil || ownerID == "" || item == "" || count <= 0 {
		return
	}
	if owner := e.w.agents[ownerID]; owner != nil {
		owner.Inventory[item] += count
		return
	}
	if org := e.w.orgByID(ownerID); org != nil {
		e.w.orgTreasury(org)[item] += count
	}
}

func (e workTaskExecWorldEnv) BlockName(blockID uint16) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(blockID)
}

func (e workTaskExecWorldEnv) BlockIDToItem(blockID uint16) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockIDToItem(blockID)
}

func (e workTaskExecWorldEnv) SpawnItemEntity(nowTick uint64, actor string, pos Vec3i, item string, count int, reason string) string {
	if e.w == nil {
		return ""
	}
	return e.w.spawnItemEntity(nowTick, actor, pos, item, count, reason)
}

func (e workTaskExecWorldEnv) GetContainerAt(pos Vec3i) *Container {
	if e.w == nil {
		return nil
	}
	return e.w.containers[pos]
}

func (e workTaskExecWorldEnv) RemoveContainer(pos Vec3i) {
	if e.w == nil {
		return
	}
	e.w.removeContainer(pos)
}

func (e workTaskExecWorldEnv) RemoveBoard(pos Vec3i) {
	if e.w == nil {
		return
	}
	e.w.removeBoard(pos)
}

func (e workTaskExecWorldEnv) RemoveSign(nowTick uint64, actor string, pos Vec3i, reason string) {
	if e.w == nil {
		return
	}
	e.w.removeSign(nowTick, actor, pos, reason)
}

func (e workTaskExecWorldEnv) RemoveConveyor(nowTick uint64, actor string, pos Vec3i, reason string) {
	if e.w == nil {
		return
	}
	e.w.removeConveyor(nowTick, actor, pos, reason)
}

func (e workTaskExecWorldEnv) RemoveSwitch(nowTick uint64, actor string, pos Vec3i, reason string) {
	if e.w == nil {
		return
	}
	e.w.removeSwitch(nowTick, actor, pos, reason)
}

func (e workTaskExecWorldEnv) RemoveClaimByAnchor(nowTick uint64, actor string, pos Vec3i, reason string) {
	if e.w == nil {
		return
	}
	e.w.removeClaimByAnchor(nowTick, actor, pos, reason)
}

func (e workTaskExecWorldEnv) OnMinedBlockDuringEvent(a *Agent, pos Vec3i, blockName string, nowTick uint64) {
	if e.w == nil {
		return
	}
	e.w.onMinedBlockDuringEvent(a, pos, blockName, nowTick)
}

type claimTaskWorldEnv struct {
	w *World
}

func (e claimTaskWorldEnv) InBounds(pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.chunks.inBounds(pos)
}

func (e claimTaskWorldEnv) CanBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	if e.w == nil {
		return false
	}
	return e.w.canBuildAt(agentID, pos, nowTick)
}

func (e claimTaskWorldEnv) Claims() []*LandClaim {
	if e.w == nil {
		return nil
	}
	out := make([]*LandClaim, 0, len(e.w.claims))
	for _, c := range e.w.claims {
		out = append(out, c)
	}
	return out
}

func (e claimTaskWorldEnv) BlockAt(pos Vec3i) uint16 {
	if e.w == nil {
		return 0
	}
	return e.w.chunks.GetBlock(pos)
}

func (e claimTaskWorldEnv) AirBlockID() uint16 {
	if e.w == nil {
		return 0
	}
	return e.w.chunks.gen.Air
}

func (e claimTaskWorldEnv) ClaimTotemBlockID() (uint16, bool) {
	if e.w == nil {
		return 0, false
	}
	id, ok := e.w.catalogs.Blocks.Index["CLAIM_TOTEM"]
	return id, ok
}

func (e claimTaskWorldEnv) SetBlock(pos Vec3i, blockID uint16) {
	if e.w == nil {
		return
	}
	e.w.chunks.SetBlock(pos, blockID)
}

func (e claimTaskWorldEnv) AuditSetBlock(nowTick uint64, actor string, pos Vec3i, from uint16, to uint16, reason string) {
	if e.w == nil {
		return
	}
	e.w.auditSetBlock(nowTick, actor, pos, from, to, reason)
}

func (e claimTaskWorldEnv) NewLandID(owner string) string {
	if e.w == nil {
		return ""
	}
	return e.w.newLandID(owner)
}

func (e claimTaskWorldEnv) WorldType() string {
	if e.w == nil {
		return ""
	}
	return e.w.cfg.WorldType
}

func (e claimTaskWorldEnv) DayTicks() int {
	if e.w == nil {
		return 0
	}
	return e.w.cfg.DayTicks
}

func (e claimTaskWorldEnv) PutClaim(c *LandClaim) {
	if e.w == nil || c == nil {
		return
	}
	e.w.claims[c.LandID] = c
}

type movementTaskReqWorldEnv struct {
	w *World
}

func (e movementTaskReqWorldEnv) NewTaskID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newTaskID()
}

func (e movementTaskReqWorldEnv) InBounds(pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.chunks.inBounds(pos)
}

func (e movementTaskReqWorldEnv) FollowTargetPos(targetID string) (Vec3i, bool) {
	if e.w == nil {
		return Vec3i{}, false
	}
	return e.w.followTargetPos(targetID)
}
