package world

import (
	"context"
	"errors"
	"sort"
)

type orgMetaUpsertReq struct {
	Orgs []OrgTransfer
	Resp chan orgMetaUpsertResp
}

type orgMetaUpsertResp struct {
	Err string
}

// RequestUpsertOrgMeta applies manager-authoritative org metadata into this world.
// It updates identity/membership only; treasury remains world-local.
func (w *World) RequestUpsertOrgMeta(ctx context.Context, orgs []OrgTransfer) error {
	if w == nil || w.orgMetaUpsert == nil {
		return errors.New("org metadata upsert not available")
	}
	req := orgMetaUpsertReq{
		Orgs: orgs,
		Resp: make(chan orgMetaUpsertResp, 1),
	}
	select {
	case w.orgMetaUpsert <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return errors.New(resp.Err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *World) handleOrgMetaUpsertReq(req orgMetaUpsertReq) {
	resp := orgMetaUpsertResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()
	if w == nil {
		resp.Err = "world unavailable"
		return
	}

	incoming := map[string]OrgTransfer{}
	for _, org := range req.Orgs {
		if org.OrgID == "" {
			continue
		}
		incoming[org.OrgID] = org
	}

	ids := make([]string, 0, len(incoming))
	for id := range incoming {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, orgID := range ids {
		src := incoming[orgID]
		dst := w.orgByID(orgID)
		if dst == nil {
			dst = &Organization{
				OrgID:           orgID,
				Kind:            src.Kind,
				Name:            src.Name,
				CreatedTick:     src.CreatedTick,
				MetaVersion:     src.MetaVersion,
				Members:         map[string]OrgRole{},
				Treasury:        map[string]int{},
				TreasuryByWorld: map[string]map[string]int{},
			}
			w.orgs[orgID] = dst
		}
		if src.MetaVersion < dst.MetaVersion {
			continue
		}
		if src.Kind != "" {
			dst.Kind = src.Kind
		}
		if src.Name != "" {
			dst.Name = src.Name
		}
		if dst.CreatedTick == 0 || (src.CreatedTick != 0 && src.CreatedTick < dst.CreatedTick) {
			dst.CreatedTick = src.CreatedTick
		}
		if src.MetaVersion > dst.MetaVersion {
			dst.MetaVersion = src.MetaVersion
		}
		// Replace members with authoritative set.
		members := map[string]OrgRole{}
		memberIDs := make([]string, 0, len(src.Members))
		for aid := range src.Members {
			memberIDs = append(memberIDs, aid)
		}
		sort.Strings(memberIDs)
		for _, aid := range memberIDs {
			role := src.Members[aid]
			if aid == "" || role == "" {
				continue
			}
			members[aid] = role
		}
		dst.Members = members
		// Keep world-local treasury view initialized.
		_ = w.orgTreasury(dst)
	}

	// Reconcile online agents' org assignment with upserted membership.
	ownerByAgent := map[string]string{}
	for _, orgID := range ids {
		org := w.orgByID(orgID)
		if org == nil {
			continue
		}
		memberIDs := make([]string, 0, len(org.Members))
		for aid := range org.Members {
			memberIDs = append(memberIDs, aid)
		}
		sort.Strings(memberIDs)
		for _, aid := range memberIDs {
			if aid == "" {
				continue
			}
			if cur, ok := ownerByAgent[aid]; !ok || orgID < cur {
				ownerByAgent[aid] = orgID
			}
		}
	}
	for _, a := range w.agents {
		if a == nil {
			continue
		}
		if orgID, ok := ownerByAgent[a.ID]; ok {
			a.OrgID = orgID
			continue
		}
		if a.OrgID == "" {
			continue
		}
		org := w.orgByID(a.OrgID)
		if org == nil || org.Members == nil || org.Members[a.ID] == "" {
			a.OrgID = ""
		}
	}
}
