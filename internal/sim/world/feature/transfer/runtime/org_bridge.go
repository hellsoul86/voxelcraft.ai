package runtime

import (
	orgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TransfersFromStates(states []orgpkg.State) []OrgTransfer {
	if len(states) == 0 {
		return nil
	}
	out := make([]OrgTransfer, 0, len(states))
	for _, s := range states {
		if s.OrgID == "" {
			continue
		}
		members := map[string]modelpkg.OrgRole{}
		for aid, role := range s.Members {
			members[aid] = modelpkg.OrgRole(role)
		}
		out = append(out, OrgTransfer{
			OrgID:       s.OrgID,
			Kind:        modelpkg.OrgKind(s.Kind),
			Name:        s.Name,
			CreatedTick: s.CreatedTick,
			MetaVersion: s.MetaVersion,
			Members:     members,
		})
	}
	return out
}

func StatesFromTransfers(orgs []OrgTransfer) []orgpkg.State {
	if len(orgs) == 0 {
		return nil
	}
	incoming := make([]orgpkg.State, 0, len(orgs))
	for _, org := range orgs {
		if org.OrgID == "" {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			members[aid] = string(role)
		}
		incoming = append(incoming, orgpkg.State{
			OrgID:       org.OrgID,
			Kind:        string(org.Kind),
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     members,
		})
	}
	return incoming
}
