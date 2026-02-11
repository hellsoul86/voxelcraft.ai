package world

import (
	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

func (w *World) importSnapshotOrgs(s snapshot.SnapshotV1) (maxOrg uint64) {
	w.orgs = map[string]*Organization{}
	for _, o := range s.Orgs {
		members := map[string]OrgRole{}
		for aid, role := range o.Members {
			if aid == "" || role == "" {
				continue
			}
			members[aid] = OrgRole(role)
		}
		treasury := map[string]int{}
		for item, n := range o.Treasury {
			if n != 0 {
				treasury[item] = n
			}
		}
		treasuryByWorld := map[string]map[string]int{}
		for wid, src := range o.TreasuryByWorld {
			if wid == "" || len(src) == 0 {
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
		currentWorldID := w.cfg.ID
		if currentWorldID == "" {
			currentWorldID = "GLOBAL"
		}
		currentTreasury := treasuryByWorld[currentWorldID]
		if currentTreasury == nil {
			currentTreasury = map[string]int{}
			for item, n := range treasury {
				if item == "" || n == 0 {
					continue
				}
				currentTreasury[item] = n
			}
			treasuryByWorld[currentWorldID] = currentTreasury
		}
		oo := &Organization{
			OrgID:           o.OrgID,
			Kind:            OrgKind(o.Kind),
			Name:            o.Name,
			CreatedTick:     o.CreatedTick,
			MetaVersion:     o.MetaVersion,
			Members:         members,
			Treasury:        currentTreasury,
			TreasuryByWorld: treasuryByWorld,
		}
		if oo.MetaVersion == 0 && len(oo.Members) > 0 {
			oo.MetaVersion = 1
		}
		w.orgs[oo.OrgID] = oo
		if n, ok := parseUintAfterPrefix("ORG", oo.OrgID); ok && n > maxOrg {
			maxOrg = n
		}
	}
	return maxOrg
}

func (w *World) importSnapshotStructures(s snapshot.SnapshotV1) {
	w.structures = map[string]*Structure{}
	for _, ss := range s.Structures {
		id := ss.StructureID
		if id == "" {
			continue
		}
		usedBy := map[string]uint64{}
		for aid, t := range ss.UsedBy {
			if aid != "" && t != 0 {
				usedBy[aid] = t
			}
		}
		w.structures[id] = &Structure{
			StructureID:      id,
			BlueprintID:      ss.BlueprintID,
			BuilderID:        ss.BuilderID,
			Anchor:           Vec3i{X: ss.Anchor[0], Y: ss.Anchor[1], Z: ss.Anchor[2]},
			Rotation:         blueprint.NormalizeRotation(ss.Rotation),
			Min:              Vec3i{X: ss.Min[0], Y: ss.Min[1], Z: ss.Min[2]},
			Max:              Vec3i{X: ss.Max[0], Y: ss.Max[1], Z: ss.Max[2]},
			CompletedTick:    ss.CompletedTick,
			AwardDueTick:     ss.AwardDueTick,
			Awarded:          ss.Awarded,
			UsedBy:           usedBy,
			LastInfluenceDay: ss.LastInfluenceDay,
		}
	}
}

func (w *World) importSnapshotStats(s snapshot.SnapshotV1) {
	if s.Stats != nil && len(s.Stats.Buckets) > 0 && s.Stats.BucketTicks > 0 {
		st := &WorldStats{
			BucketTicks:  s.Stats.BucketTicks,
			WindowTicksV: s.Stats.WindowTicks,
			Buckets:      make([]StatsBucket, len(s.Stats.Buckets)),
			CurIdx:       s.Stats.CurIdx,
			CurBase:      s.Stats.CurBase,
			SeenChunks:   map[StatsChunkKey]bool{},
		}
		for i, b := range s.Stats.Buckets {
			st.Buckets[i] = StatsBucket{
				Trades:             b.Trades,
				Denied:             b.Denied,
				ChunksDiscovered:   b.ChunksDiscovered,
				BlueprintsComplete: b.BlueprintsComplete,
			}
		}
		if st.CurIdx < 0 || st.CurIdx >= len(st.Buckets) {
			st.CurIdx = 0
		}
		for _, k := range s.Stats.SeenChunks {
			st.SeenChunks[StatsChunkKey{CX: k.CX, CZ: k.CZ}] = true
		}
		w.stats = st
		return
	}
	w.stats = NewWorldStats(300, 72000)
}
