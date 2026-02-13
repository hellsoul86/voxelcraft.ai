package world

import governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"

type governanceClaimInstantsWorldEnv struct {
	w *World
}

func (e governanceClaimInstantsWorldEnv) GetLand(landID string) *LandClaim {
	if e.w == nil {
		return nil
	}
	return e.w.claims[landID]
}

func (e governanceClaimInstantsWorldEnv) IsLandAdmin(agentID string, land *LandClaim) bool {
	if e.w == nil {
		return false
	}
	return e.w.isLandAdmin(agentID, land)
}

func (e governanceClaimInstantsWorldEnv) BlockNameAt(pos Vec3i) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(e.w.chunks.GetBlock(pos))
}

func (e governanceClaimInstantsWorldEnv) ClaimRecords() []governanceinstantspkg.ClaimRecord {
	if e.w == nil {
		return nil
	}
	records := make([]governanceinstantspkg.ClaimRecord, 0, len(e.w.claims))
	for _, c := range e.w.claims {
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
}

func (e governanceClaimInstantsWorldEnv) OwnerExists(ownerID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.agents[ownerID] != nil || e.w.orgByID(ownerID) != nil
}

func (e governanceClaimInstantsWorldEnv) AuditClaimEvent(nowTick uint64, actorID string, action string, pos Vec3i, reason string, details map[string]any) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, action, pos, reason, details)
}
