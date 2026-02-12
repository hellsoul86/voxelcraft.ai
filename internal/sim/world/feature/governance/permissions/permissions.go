package permissions

import "voxelcraft.ai/internal/sim/world/feature/governance/claims"

type Permissions struct {
	CanBuild  bool
	CanBreak  bool
	CanDamage bool
	CanTrade  bool
}

func WildPermissions() Permissions {
	return Permissions{
		CanBuild:  true,
		CanBreak:  true,
		CanDamage: false,
		CanTrade:  true,
	}
}

func ForLand(isMember bool, maintenanceStage int, flags claims.Flags) Permissions {
	if isMember {
		return Permissions{
			CanBuild:  true,
			CanBreak:  true,
			CanDamage: flags.AllowDamage,
			CanTrade:  true,
		}
	}
	if maintenanceStage >= 2 {
		return WildPermissions()
	}
	return Permissions{
		CanBuild:  flags.AllowBuild,
		CanBreak:  flags.AllowBreak,
		CanDamage: flags.AllowDamage,
		CanTrade:  flags.AllowTrade,
	}
}
