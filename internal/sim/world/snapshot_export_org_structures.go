package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) exportSnapshotOrgs() []snapshot.OrgV1 {
	orgIDs := make([]string, 0, len(w.orgs))
	for id := range w.orgs {
		orgIDs = append(orgIDs, id)
	}
	sort.Strings(orgIDs)
	orgSnaps := make([]snapshot.OrgV1, 0, len(orgIDs))
	for _, id := range orgIDs {
		o := w.orgs[id]
		if o == nil {
			continue
		}
		members := map[string]string{}
		for aid, role := range o.Members {
			if aid != "" && role != "" {
				members[aid] = string(role)
			}
		}
		treasury := map[string]int{}
		for item, n := range w.orgTreasury(o) {
			if n != 0 {
				treasury[item] = n
			}
		}
		var treasuryByWorld map[string]map[string]int
		if len(o.TreasuryByWorld) > 0 {
			treasuryByWorld = map[string]map[string]int{}
			worldIDs := make([]string, 0, len(o.TreasuryByWorld))
			for wid := range o.TreasuryByWorld {
				worldIDs = append(worldIDs, wid)
			}
			sort.Strings(worldIDs)
			for _, wid := range worldIDs {
				src := o.TreasuryByWorld[wid]
				if len(src) == 0 {
					continue
				}
				dst := map[string]int{}
				for item, n := range src {
					if item == "" || n == 0 {
						continue
					}
					dst[item] = n
				}
				if len(dst) > 0 {
					treasuryByWorld[wid] = dst
				}
			}
			if len(treasuryByWorld) == 0 {
				treasuryByWorld = nil
			}
		}
		orgSnaps = append(orgSnaps, snapshot.OrgV1{
			OrgID:           o.OrgID,
			Kind:            string(o.Kind),
			Name:            o.Name,
			CreatedTick:     o.CreatedTick,
			MetaVersion:     o.MetaVersion,
			Members:         members,
			Treasury:        treasury,
			TreasuryByWorld: treasuryByWorld,
		})
	}
	return orgSnaps
}

func (w *World) exportSnapshotStructures() []snapshot.StructureV1 {
	structIDs := make([]string, 0, len(w.structures))
	for id := range w.structures {
		structIDs = append(structIDs, id)
	}
	sort.Strings(structIDs)
	structSnaps := make([]snapshot.StructureV1, 0, len(structIDs))
	for _, id := range structIDs {
		s := w.structures[id]
		if s == nil {
			continue
		}
		var usedBy map[string]uint64
		if len(s.UsedBy) > 0 {
			usedBy = map[string]uint64{}
			for aid, t := range s.UsedBy {
				if aid != "" && t != 0 {
					usedBy[aid] = t
				}
			}
			if len(usedBy) == 0 {
				usedBy = nil
			}
		}
		structSnaps = append(structSnaps, snapshot.StructureV1{
			StructureID:      s.StructureID,
			BlueprintID:      s.BlueprintID,
			BuilderID:        s.BuilderID,
			Anchor:           s.Anchor.ToArray(),
			Rotation:         s.Rotation,
			Min:              s.Min.ToArray(),
			Max:              s.Max.ToArray(),
			CompletedTick:    s.CompletedTick,
			AwardDueTick:     s.AwardDueTick,
			Awarded:          s.Awarded,
			UsedBy:           usedBy,
			LastInfluenceDay: s.LastInfluenceDay,
		})
	}
	return structSnaps
}
