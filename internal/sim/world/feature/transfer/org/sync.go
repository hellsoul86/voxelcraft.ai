package org

type State struct {
	OrgID       string
	Kind        string
	Name        string
	CreatedTick uint64
	MetaVersion uint64
	Members     map[string]string
}

func NormalizeStates(states []State) []State {
	records := make([]Record, 0, len(states))
	for _, s := range states {
		records = append(records, stateToRecord(s))
	}
	records = SnapshotRecords(records)
	out := make([]State, 0, len(records))
	for _, r := range records {
		out = append(out, recordToState(r))
	}
	return out
}

func MergeStates(existing, incoming []State) (merged []State, ownerByAgent map[string]string) {
	existingRecords := make([]Record, 0, len(existing))
	for _, s := range existing {
		existingRecords = append(existingRecords, stateToRecord(s))
	}
	incomingRecords := make([]Record, 0, len(incoming))
	for _, s := range incoming {
		incomingRecords = append(incomingRecords, stateToRecord(s))
	}
	mergedRecords, ownerByAgent := MergeRecords(existingRecords, incomingRecords)
	merged = make([]State, 0, len(mergedRecords))
	for _, r := range mergedRecords {
		merged = append(merged, recordToState(r))
	}
	return merged, ownerByAgent
}

func stateToRecord(s State) Record {
	members := map[string]string{}
	for aid, role := range s.Members {
		if aid == "" || role == "" {
			continue
		}
		members[aid] = role
	}
	return Record{
		OrgID:       s.OrgID,
		Kind:        s.Kind,
		Name:        s.Name,
		CreatedTick: s.CreatedTick,
		MetaVersion: s.MetaVersion,
		Members:     members,
	}
}

func recordToState(r Record) State {
	members := map[string]string{}
	for aid, role := range r.Members {
		if aid == "" || role == "" {
			continue
		}
		members[aid] = role
	}
	return State{
		OrgID:       r.OrgID,
		Kind:        r.Kind,
		Name:        r.Name,
		CreatedTick: r.CreatedTick,
		MetaVersion: r.MetaVersion,
		Members:     members,
	}
}
