package runtime

import (
	"sort"

	transfermapspkg "voxelcraft.ai/internal/sim/world/feature/transfer/maps"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func BuildIncomingAgent(t AgentTransfer, worldID string) *modelpkg.Agent {
	a := &modelpkg.Agent{
		ID:                           t.ID,
		Name:                         t.Name,
		OrgID:                        t.OrgID,
		CurrentWorldID:               worldID,
		WorldSwitchCooldownUntilTick: t.WorldSwitchCooldownUntilTick,
		Pos:                          t.Pos,
		Yaw:                          t.Yaw,
		HP:                           t.HP,
		Hunger:                       t.Hunger,
		StaminaMilli:                 t.StaminaMilli,
		RepTrade:                     t.RepTrade,
		RepBuild:                     t.RepBuild,
		RepSocial:                    t.RepSocial,
		RepLaw:                       t.RepLaw,
		Fun:                          t.Fun,
		Inventory:                    transfermapspkg.CopyPositiveIntMap(t.Inventory),
		Equipment:                    t.Equipment,
		Memory: transfermapspkg.CopyMap(t.Memory, func(k string, _ modelpkg.MemoryEntry) bool {
			return k != ""
		}),
	}
	if a.Pos.Y != 0 {
		a.Pos.Y = 0
	}
	if a.OrgID == "" && t.Org != nil && t.Org.OrgID != "" {
		a.OrgID = t.Org.OrgID
	}
	a.MoveTask = nil
	a.WorkTask = nil
	a.InitDefaults()
	return a
}

func BuildOutgoingAgent(a *modelpkg.Agent, worldID string, org *OrgTransfer) AgentTransfer {
	if a == nil {
		return AgentTransfer{}
	}
	inv := transfermapspkg.CopyPositiveIntMap(a.Inventory)
	mem := transfermapspkg.CopyMap(a.Memory, func(k string, _ modelpkg.MemoryEntry) bool { return k != "" })
	return AgentTransfer{
		ID:                           a.ID,
		Name:                         a.Name,
		OrgID:                        a.OrgID,
		Org:                          org,
		FromWorldID:                  worldID,
		CurrentWorldID:               a.CurrentWorldID,
		WorldSwitchCooldownUntilTick: a.WorldSwitchCooldownUntilTick,
		Pos:                          a.Pos,
		Yaw:                          a.Yaw,
		HP:                           a.HP,
		Hunger:                       a.Hunger,
		StaminaMilli:                 a.StaminaMilli,
		RepTrade:                     a.RepTrade,
		RepBuild:                     a.RepBuild,
		RepSocial:                    a.RepSocial,
		RepLaw:                       a.RepLaw,
		Fun:                          a.Fun,
		Inventory:                    inv,
		Equipment:                    a.Equipment,
		Memory:                       mem,
	}
}

func BuildOrgTransferFromOrganization(org *modelpkg.Organization) *OrgTransfer {
	if org == nil {
		return nil
	}
	members := map[string]modelpkg.OrgRole{}
	memberIDs := make([]string, 0, len(org.Members))
	for aid := range org.Members {
		memberIDs = append(memberIDs, aid)
	}
	sort.Strings(memberIDs)
	for _, aid := range memberIDs {
		role := org.Members[aid]
		if aid == "" || role == "" {
			continue
		}
		members[aid] = role
	}
	return &OrgTransfer{
		OrgID:       org.OrgID,
		Kind:        org.Kind,
		Name:        org.Name,
		CreatedTick: org.CreatedTick,
		MetaVersion: org.MetaVersion,
		Members:     members,
	}
}
