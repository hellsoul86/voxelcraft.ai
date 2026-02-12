package model

type ClaimFlags struct {
	AllowBuild  bool
	AllowBreak  bool
	AllowDamage bool
	AllowTrade  bool
}

const (
	ClaimTypeDefault   = "DEFAULT"
	ClaimTypeHomestead = "HOMESTEAD"
	ClaimTypeCityCore  = "CITY_CORE"
)

type LandClaim struct {
	LandID    string
	Owner     string // agent id (MVP)
	ClaimType string
	Anchor    Vec3i
	Radius    int // square radius in blocks
	Flags     ClaimFlags
	Members   map[string]bool // agent ids

	// MVP configurable parameters (via laws or direct settings).
	MarketTax     float64 // 0..1
	CurfewEnabled bool
	CurfewStart   float64 // 0..1 time_of_day
	CurfewEnd     float64 // 0..1 time_of_day

	// Law: fine for illegal break attempts.
	FineBreakEnabled  bool
	FineBreakItem     string
	FineBreakPerBlock int

	// Law: access pass for the claim's "core" area.
	AccessPassEnabled bool
	AccessTicketItem  string
	AccessTicketCost  int

	// Maintenance: stage 0=ok, 1=late (no expansion), 2=unprotected.
	MaintenanceDueTick uint64
	MaintenanceStage   int
}

func (c *LandClaim) Contains(pos Vec3i) bool {
	dx := pos.X - c.Anchor.X
	if dx < 0 {
		dx = -dx
	}
	dz := pos.Z - c.Anchor.Z
	if dz < 0 {
		dz = -dz
	}
	return dx <= c.Radius && dz <= c.Radius
}

