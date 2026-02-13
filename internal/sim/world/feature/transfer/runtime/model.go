package runtime

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type AgentTransfer struct {
	ID    string
	Name  string
	OrgID string
	Org   *OrgTransfer

	FromWorldID                  string
	CurrentWorldID               string
	FromEntryPointID             string
	ToEntryPointID               string
	WorldSwitchCooldownUntilTick uint64

	Pos modelpkg.Vec3i
	Yaw int

	HP           int
	Hunger       int
	StaminaMilli int

	RepTrade  int
	RepBuild  int
	RepSocial int
	RepLaw    int

	Fun       modelpkg.FunScore
	Inventory map[string]int
	Equipment modelpkg.Equipment
	Memory    map[string]modelpkg.MemoryEntry
}

type OrgTransfer struct {
	OrgID       string
	Kind        modelpkg.OrgKind
	Name        string
	CreatedTick uint64
	MetaVersion uint64
	Members     map[string]modelpkg.OrgRole
}
