package world

import (
	"voxelcraft.ai/internal/sim/catalogs"
	conveyruntimepkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
	workexecctxpkg "voxelcraft.ai/internal/sim/world/featurectx/workexec"
)

func newWorkTaskExecEnv(w *World) workexecctxpkg.Env {
	return workexecctxpkg.Env{
		GetContainerByIDFn: w.getContainerByID,
		ParseContainerIDFn: parseContainerID,
		BlockNameAtFn: func(pos Vec3i) string {
			return w.blockName(w.chunks.GetBlock(pos))
		},
		EnsureBoardFn:                  w.ensureBoard,
		BoardIDAtFn:                    boardIDAt,
		GetSignFn:                      func(pos Vec3i) *Sign { return w.signs[pos] },
		SignIDAtFn:                     signIDAt,
		ContractSummariesForTerminalFn: w.contractSummariesForTerminal,
		OnContainerOpenedDuringEventFn: w.onContainerOpenedDuringEvent,
		CanWithdrawFromContainerFn:     w.canWithdrawFromContainer,
		AuditTransferFn: func(nowTick uint64, actorID string, at Vec3i, srcID string, dstID string, item string, count int) {
			w.auditEvent(nowTick, actorID, "TRANSFER", at, "TRANSFER", map[string]any{
				"src":   srcID,
				"dst":   dstID,
				"item":  item,
				"count": count,
			})
		},
		GetRecipeFn: func(recipeID string) (catalogs.RecipeDef, bool) {
			rec, ok := w.catalogs.Recipes.ByID[recipeID]
			return rec, ok
		},
		GetBlueprintFn: func(blueprintID string) (catalogs.BlueprintDef, bool) {
			bp, ok := w.catalogs.Blueprints.ByID[blueprintID]
			return bp, ok
		},
		GetSmeltRecipeByInputFn: func(itemID string) (catalogs.RecipeDef, bool) {
			rec, ok := w.smeltByInput[itemID]
			return rec, ok
		},
		NearBlockFn: w.nearBlock,
		OnRecipeFn:  w.funOnRecipe,
		GetItemEntityFn: func(id string) *ItemEntity {
			return w.items[id]
		},
		CanPickupItemEntityFn: w.canPickupItemEntity,
		RemoveItemEntityFn:    w.removeItemEntity,
		InBoundsFn:            w.chunks.inBounds,
		CanBuildAtFn:          w.canBuildAt,
		RecordDeniedFn: func(nowTick uint64) {
			if w.stats != nil {
				w.stats.RecordDenied(nowTick)
			}
		},
		BumpRepLawFn:                    w.bumpRepLaw,
		BlockAtFn:                       w.chunks.GetBlock,
		AirBlockIDFn:                    func() uint16 { return w.chunks.gen.Air },
		ItemPlaceAsFn:                   itemPlaceAs(w.catalogs.Items.Defs),
		BlockIDByNameFn:                 blockIDByName(w.catalogs.Blocks.Index),
		SetBlockFn:                      w.chunks.SetBlock,
		AuditSetBlockFn:                 w.auditSetBlock,
		EnsureContainerForPlacedBlockFn: w.ensureContainerForPlacedBlock,
		EnsureBlueprintMaterialsFn:      w.blueprintEnsureMaterials,
		EnsureConveyorFromYawFn: func(pos Vec3i, yaw int) {
			dx, dz := conveyruntimepkg.YawToDir(yaw)
			w.ensureConveyor(pos, dx, dz)
		},
		CanBreakAtFn:     w.canBreakAt,
		PermissionsForFn: w.permissionsFor,
		IsLandMemberFn:   w.isLandMember,
		TransferToLandOwnerFn: func(ownerID string, item string, count int) {
			if ownerID == "" || item == "" || count <= 0 {
				return
			}
			if owner := w.agents[ownerID]; owner != nil {
				owner.Inventory[item] += count
				return
			}
			if org := w.orgByID(ownerID); org != nil {
				w.orgTreasury(org)[item] += count
			}
		},
		BlockNameFn:     w.blockName,
		BlockIDToItemFn: w.blockIDToItem,
		SpawnItemEntityFn: func(nowTick uint64, actor string, pos Vec3i, item string, count int, reason string) string {
			return w.spawnItemEntity(nowTick, actor, pos, item, count, reason)
		},
		GetContainerAtFn: func(pos Vec3i) *Container { return w.containers[pos] },
		RemoveContainerFn: func(pos Vec3i) {
			_ = w.removeContainer(pos)
		},
		RemoveBoardFn:         w.removeBoard,
		RemoveSignFn:          w.removeSign,
		RemoveConveyorFn:      w.removeConveyor,
		RemoveSwitchFn:        w.removeSwitch,
		RemoveClaimByAnchorFn: w.removeClaimByAnchor,
		OnMinedBlockDuringEventFn: func(a *Agent, pos Vec3i, blockName string, nowTick uint64) {
			w.onMinedBlockDuringEvent(a, pos, blockName, nowTick)
		},
	}
}

func itemPlaceAs(defs map[string]catalogs.ItemDef) func(itemID string) (string, bool) {
	return func(itemID string) (string, bool) {
		def, ok := defs[itemID]
		if !ok {
			return "", false
		}
		return def.PlaceAs, true
	}
}

func blockIDByName(index map[string]uint16) func(blockName string) (uint16, bool) {
	return func(blockName string) (uint16, bool) {
		bid, ok := index[blockName]
		return bid, ok
	}
}
