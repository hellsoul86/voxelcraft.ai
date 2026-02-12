package org

import "sort"

type Meta struct {
	OrgID       string
	Kind        string
	Name        string
	CreatedTick uint64
	MetaVersion uint64
	Members     map[string]string
}

func NormalizeMembers(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	ids := make([]string, 0, len(src))
	for id := range src {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		role := src[id]
		if id == "" || role == "" {
			continue
		}
		out[id] = role
	}
	return out
}

func SortedMeta(src map[string]Meta) []Meta {
	if len(src) == 0 {
		return nil
	}
	ids := make([]string, 0, len(src))
	for id := range src {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Meta, 0, len(ids))
	for _, id := range ids {
		org := src[id]
		if org.OrgID == "" {
			continue
		}
		org.Members = NormalizeMembers(org.Members)
		out = append(out, org)
	}
	return out
}

func MergeMeta(dst Meta, src Meta) (Meta, bool) {
	if src.MetaVersion < dst.MetaVersion {
		return dst, false
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
	dst.Members = NormalizeMembers(src.Members)
	return dst, true
}

func OwnerByAgent(orgs map[string]Meta) map[string]string {
	if len(orgs) == 0 {
		return map[string]string{}
	}
	orgIDs := make([]string, 0, len(orgs))
	for id := range orgs {
		orgIDs = append(orgIDs, id)
	}
	sort.Strings(orgIDs)
	owners := map[string]string{}
	for _, orgID := range orgIDs {
		org := orgs[orgID]
		memberIDs := make([]string, 0, len(org.Members))
		for aid := range org.Members {
			memberIDs = append(memberIDs, aid)
		}
		sort.Strings(memberIDs)
		for _, aid := range memberIDs {
			if aid == "" {
				continue
			}
			if cur, ok := owners[aid]; !ok || orgID < cur {
				owners[aid] = orgID
			}
		}
	}
	return owners
}

func MergeMetaMaps(existing map[string]Meta, incoming map[string]Meta) map[string]Meta {
	if len(existing) == 0 && len(incoming) == 0 {
		return map[string]Meta{}
	}
	out := map[string]Meta{}
	for _, org := range SortedMeta(existing) {
		out[org.OrgID] = org
	}
	for _, src := range SortedMeta(incoming) {
		if src.OrgID == "" {
			continue
		}
		dst := out[src.OrgID]
		if dst.OrgID == "" {
			dst = Meta{
				OrgID:       src.OrgID,
				Kind:        src.Kind,
				Name:        src.Name,
				CreatedTick: src.CreatedTick,
				MetaVersion: src.MetaVersion,
				Members:     map[string]string{},
			}
		}
		merged, accepted := MergeMeta(dst, src)
		if !accepted {
			continue
		}
		out[src.OrgID] = merged
	}
	return out
}
