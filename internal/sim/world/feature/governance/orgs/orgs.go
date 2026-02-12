package orgs

import (
	"slices"
	"strings"
)

const (
	KindGuild = "GUILD"
	KindCity  = "CITY"
)

func NormalizeOrgKind(kind string) string {
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case KindGuild:
		return KindGuild
	case KindCity:
		return KindCity
	default:
		return ""
	}
}

func ValidateOrgName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && len(trimmed) <= 40
}

func NormalizeOrgName(name string) string {
	return strings.TrimSpace(name)
}

func ValidateOrgTransferInput(orgID, itemID string, count int) (ok bool, code string, msg string) {
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(itemID) == "" || count <= 0 {
		return false, "E_BAD_REQUEST", "missing org_id/item_id/count"
	}
	return true, "", ""
}

func SelectNextLeader(memberIDs []string) string {
	if len(memberIDs) == 0 {
		return ""
	}
	copied := append([]string(nil), memberIDs...)
	slices.Sort(copied)
	return copied[0]
}
