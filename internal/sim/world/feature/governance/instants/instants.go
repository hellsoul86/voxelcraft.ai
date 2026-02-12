package instants

import (
	"sort"

	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
)

type ClaimRecord struct {
	LandID  string
	AnchorX int
	AnchorZ int
	Radius  int
}

func ValidateLandAdmin(landExists bool, isAdmin bool) (bool, string, string) {
	if !landExists {
		return false, "E_INVALID_TARGET", "land not found"
	}
	if !isAdmin {
		return false, "E_NO_PERMISSION", "not land admin"
	}
	return true, "", ""
}

func BuildZones(records []ClaimRecord) []claimspkg.Zone {
	if len(records) == 0 {
		return nil
	}
	sort.Slice(records, func(i, j int) bool { return records[i].LandID < records[j].LandID })
	out := make([]claimspkg.Zone, 0, len(records))
	for _, r := range records {
		if r.LandID == "" || r.Radius <= 0 {
			continue
		}
		out = append(out, claimspkg.Zone{
			LandID:  r.LandID,
			AnchorX: r.AnchorX,
			AnchorZ: r.AnchorZ,
			Radius:  r.Radius,
		})
	}
	return out
}
