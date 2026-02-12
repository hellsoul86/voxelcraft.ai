package model

type OrgKind string

const (
	OrgGuild OrgKind = "GUILD"
	OrgCity  OrgKind = "CITY"
)

type OrgRole string

const (
	OrgLeader  OrgRole = "LEADER"
	OrgOfficer OrgRole = "OFFICER"
	OrgMember  OrgRole = "MEMBER"
)

type Organization struct {
	OrgID       string
	Kind        OrgKind
	Name        string
	CreatedTick uint64
	MetaVersion uint64

	Members         map[string]OrgRole // agent_id -> role
	Treasury        map[string]int
	TreasuryByWorld map[string]map[string]int
}

func (o *Organization) TreasuryFor(worldID string) map[string]int {
	if o == nil {
		return nil
	}
	if worldID == "" {
		worldID = "GLOBAL"
	}
	if o.TreasuryByWorld == nil {
		o.TreasuryByWorld = map[string]map[string]int{}
	}
	m := o.TreasuryByWorld[worldID]
	if m == nil {
		m = map[string]int{}
		// One-time migration from legacy single-world treasury.
		// Only seed the very first world map; additional worlds start with empty treasuries.
		if len(o.TreasuryByWorld) == 0 {
			for item, n := range o.Treasury {
				if item == "" || n <= 0 {
					continue
				}
				m[item] = n
			}
		}
		o.TreasuryByWorld[worldID] = m
	}
	// Keep legacy field as a view for the currently accessed world to preserve existing callers/tests.
	o.Treasury = m
	return m
}

