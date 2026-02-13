package world

import (
	"voxelcraft.ai/internal/sim/catalogs"
	workruntimepkg "voxelcraft.ai/internal/sim/world/feature/work/runtime"
)

type containerCand = workruntimepkg.StorageCandidate

func (w *World) blueprintStorageCandidates(agentID string, anchor Vec3i) []containerCand {
	var anchorLandID string
	if land := w.landAt(anchor); land != nil {
		anchorLandID = land.LandID
	}
	return workruntimepkg.BuildStorageCandidates(workruntimepkg.StorageCandidateInput{
		Anchor:        anchor,
		AnchorLandID:  anchorLandID,
		AgentID:       agentID,
		Containers:    w.containers,
		AutoPullRange: w.cfg.BlueprintAutoPullRange,
		LandIDAt: func(pos Vec3i) (string, bool) {
			land := w.landAt(pos)
			if land == nil {
				return "", false
			}
			return land.LandID, true
		},
		CanWithdraw: func(agentID string, pos Vec3i) bool {
			return w.canWithdrawFromContainer(agentID, pos)
		},
		Manhattan: Manhattan,
	})
}

func (w *World) blueprintEnsureMaterials(a *Agent, anchor Vec3i, cost []catalogs.ItemCount, nowTick uint64) (ok bool, errMsg string) {
	if a == nil || len(cost) == 0 {
		return true, ""
	}

	cands := w.blueprintStorageCandidates(a.ID, anchor)
	return workruntimepkg.EnsureBlueprintMaterials(a.Inventory, cands, cost)
}
