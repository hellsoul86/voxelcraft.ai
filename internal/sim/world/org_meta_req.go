package world

import (
	"context"
	"errors"
	"sort"
)

type orgMetaReq struct {
	Resp chan orgMetaResp
}

type orgMetaResp struct {
	Orgs []OrgTransfer
	Err  string
}

// RequestOrgMetaSnapshot returns a world-local snapshot of organization metadata.
// Treasury is intentionally excluded; this API is used for cross-world identity sync.
func (w *World) RequestOrgMetaSnapshot(ctx context.Context) ([]OrgTransfer, error) {
	if w == nil || w.orgMetaReq == nil {
		return nil, errors.New("org metadata query not available")
	}
	req := orgMetaReq{Resp: make(chan orgMetaResp, 1)}
	select {
	case w.orgMetaReq <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return nil, errors.New(resp.Err)
		}
		return resp.Orgs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *World) handleOrgMetaReq(req orgMetaReq) {
	resp := orgMetaResp{}
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
	ids := make([]string, 0, len(w.orgs))
	for id := range w.orgs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]OrgTransfer, 0, len(ids))
	for _, id := range ids {
		org := w.orgs[id]
		if org == nil || id == "" {
			continue
		}
		members := map[string]OrgRole{}
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
		out = append(out, OrgTransfer{
			OrgID:       org.OrgID,
			Kind:        org.Kind,
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     members,
		})
	}
	resp.Orgs = out
}
