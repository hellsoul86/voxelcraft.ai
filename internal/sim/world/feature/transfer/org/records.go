package org

type Record struct {
	OrgID       string
	Kind        string
	Name        string
	CreatedTick uint64
	MetaVersion uint64
	Members     map[string]string
}

func MetaMapFromRecords(in []Record) map[string]Meta {
	out := map[string]Meta{}
	for _, rec := range in {
		if rec.OrgID == "" {
			continue
		}
		out[rec.OrgID] = Meta{
			OrgID:       rec.OrgID,
			Kind:        rec.Kind,
			Name:        rec.Name,
			CreatedTick: rec.CreatedTick,
			MetaVersion: rec.MetaVersion,
			Members:     NormalizeMembers(rec.Members),
		}
	}
	return out
}

func SortedRecordsFromMeta(src map[string]Meta) []Record {
	if len(src) == 0 {
		return nil
	}
	sorted := SortedMeta(src)
	out := make([]Record, 0, len(sorted))
	for _, o := range sorted {
		out = append(out, Record{
			OrgID:       o.OrgID,
			Kind:        o.Kind,
			Name:        o.Name,
			CreatedTick: o.CreatedTick,
			MetaVersion: o.MetaVersion,
			Members:     NormalizeMembers(o.Members),
		})
	}
	return out
}

func SnapshotRecords(records []Record) []Record {
	return SortedRecordsFromMeta(MetaMapFromRecords(records))
}

func MergeRecords(existing []Record, incoming []Record) (merged []Record, ownerByAgent map[string]string) {
	existingMeta := MetaMapFromRecords(existing)
	incomingMeta := MetaMapFromRecords(incoming)
	mergedMeta := MergeMetaMaps(existingMeta, incomingMeta)
	return SortedRecordsFromMeta(mergedMeta), OwnerByAgent(mergedMeta)
}
