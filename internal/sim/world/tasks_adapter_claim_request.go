package world

import claimrequestctxpkg "voxelcraft.ai/internal/sim/world/featurectx/claimrequest"

func newClaimTaskEnv(w *World) claimrequestctxpkg.Env {
	return claimrequestctxpkg.Env{
		InBoundsFn:   w.chunks.inBounds,
		CanBuildAtFn: w.canBuildAt,
		ClaimsFn: func() []*LandClaim {
			out := make([]*LandClaim, 0, len(w.claims))
			for _, c := range w.claims {
				out = append(out, c)
			}
			return out
		},
		BlockAtFn:           w.chunks.GetBlock,
		AirBlockIDFn:        func() uint16 { return w.chunks.gen.Air },
		ClaimTotemBlockIDFn: claimTotemBlockID(w.catalogs.Blocks.Index),
		SetBlockFn:          w.chunks.SetBlock,
		AuditSetBlockFn:     w.auditSetBlock,
		NewLandIDFn:         w.newLandID,
		WorldTypeFn:         func() string { return w.cfg.WorldType },
		DayTicksFn:          func() int { return w.cfg.DayTicks },
		PutClaimFn: func(c *LandClaim) {
			if c != nil {
				w.claims[c.LandID] = c
			}
		},
	}
}

func claimTotemBlockID(index map[string]uint16) func() (uint16, bool) {
	return func() (uint16, bool) {
		id, ok := index["CLAIM_TOTEM"]
		return id, ok
	}
}
