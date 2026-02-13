package world

import (
	"context"
	"errors"

	orgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
	transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"
)

func (w *World) RequestOrgMetaSnapshot(ctx context.Context) ([]OrgTransfer, error) {
	if w == nil {
		return nil, errors.New("org metadata query not available")
	}
	states, err := transferruntimepkg.RequestOrgMetaSnapshot(ctx, w.orgMetaReq)
	if err != nil {
		return nil, err
	}
	return orgTransfersFromStates(states), nil
}

func (w *World) RequestUpsertOrgMeta(ctx context.Context, orgs []OrgTransfer) error {
	if w == nil {
		return errors.New("org metadata upsert not available")
	}
	return transferruntimepkg.RequestOrgMetaUpsert(ctx, w.orgMetaUpsert, orgStatesFromTransfers(orgs))
}

func (w *World) handleOrgMetaReq(req transferruntimepkg.OrgMetaReq) {
	resp := transferruntimepkg.SnapshotOrgMeta(w.orgs)
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}

func (w *World) handleOrgMetaUpsertReq(req transferruntimepkg.OrgMetaUpsertReq) {
	if len(req.Orgs) > 0 {
		transferruntimepkg.ApplyOrgMetaUpsert(transferruntimepkg.ApplyOrgMetaUpsertInput{
			Orgs:     w.orgs,
			Agents:   w.agents,
			Incoming: req.Orgs,
			OnOrg: func(org *Organization) {
				if org == nil {
					return
				}
				_ = org.TreasuryFor(w.cfg.ID)
			},
		})
	}
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- transferruntimepkg.OrgMetaUpsertResp{}:
	default:
	}
}

func orgTransfersFromStates(states []orgpkg.State) []OrgTransfer {
	if len(states) == 0 {
		return nil
	}
	out := make([]OrgTransfer, 0, len(states))
	for _, s := range states {
		if s.OrgID == "" {
			continue
		}
		members := map[string]OrgRole{}
		for aid, role := range s.Members {
			if aid == "" || role == "" {
				continue
			}
			members[aid] = OrgRole(role)
		}
		out = append(out, OrgTransfer{
			OrgID:       s.OrgID,
			Kind:        OrgKind(s.Kind),
			Name:        s.Name,
			CreatedTick: s.CreatedTick,
			MetaVersion: s.MetaVersion,
			Members:     members,
		})
	}
	return out
}

func orgStatesFromTransfers(orgs []OrgTransfer) []orgpkg.State {
	if len(orgs) == 0 {
		return nil
	}
	out := make([]orgpkg.State, 0, len(orgs))
	for _, o := range orgs {
		if o.OrgID == "" {
			continue
		}
		members := map[string]string{}
		for aid, role := range o.Members {
			if aid == "" || role == "" {
				continue
			}
			members[aid] = string(role)
		}
		out = append(out, orgpkg.State{
			OrgID:       o.OrgID,
			Kind:        string(o.Kind),
			Name:        o.Name,
			CreatedTick: o.CreatedTick,
			MetaVersion: o.MetaVersion,
			Members:     members,
		})
	}
	return out
}
