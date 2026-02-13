package runtime

import (
	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type TickInput struct {
	NowTick uint64
	Agents  []*modelpkg.Agent

	Weather string

	ActiveEventID     string
	ActiveEventCenter modelpkg.Vec3i
	ActiveEventRadius int
	ActiveEventEnds   uint64
}

type TickHooks struct {
	TimeOfDay      func(nowTick uint64) float64
	DistXZ         func(a, b modelpkg.Vec3i) int
	NearBlock      func(pos modelpkg.Vec3i, blockName string, dist int) bool
	Respawn        func(nowTick uint64, a *modelpkg.Agent, reason string)
	CleanupExpired func(nowTick uint64)
}

func inEventRange(nowTick uint64, pos modelpkg.Vec3i, in TickInput, distXZ func(a, b modelpkg.Vec3i) int) bool {
	if in.ActiveEventID == "" || in.ActiveEventRadius <= 0 || nowTick >= in.ActiveEventEnds || distXZ == nil {
		return false
	}
	return distXZ(pos, in.ActiveEventCenter) <= in.ActiveEventRadius
}

func Tick(in TickInput, hooks TickHooks) {
	nowTick := in.NowTick
	agents := in.Agents

	// Soft survival: hunger ticks down slowly; low hunger reduces stamina recovery.
	if nowTick%200 == 0 { // ~40s at 5Hz
		for _, a := range agents {
			if a == nil {
				continue
			}
			if a.Hunger > 0 {
				inBlight := in.ActiveEventID == "BLIGHT_ZONE" && inEventRange(nowTick, a.Pos, in, hooks.DistXZ)
				a.Hunger = HungerAfterTick(a.Hunger, inBlight)
			} else if a.HP > 0 {
				// Starvation pressure (slow, non-lethal alone unless ignored).
				a.HP--
				a.AddEvent(protocol.Event{"t": nowTick, "type": "DAMAGE", "kind": "STARVATION", "hp": a.HP})
			}
		}
	}

	// Weather hazards (minimal): cold snaps hurt at night unless near a torch.
	if in.Weather == "COLD" && nowTick%50 == 0 && hooks.TimeOfDay != nil && IsNight(hooks.TimeOfDay(nowTick)) {
		for _, a := range agents {
			if a == nil || a.HP <= 0 {
				continue
			}
			if hooks.NearBlock != nil && hooks.NearBlock(a.Pos, "TORCH", 3) {
				continue
			}
			a.HP--
			a.AddEvent(protocol.Event{"t": nowTick, "type": "DAMAGE", "kind": "COLD", "hp": a.HP})
		}
	}

	// Event hazard: bandit camp is safer in groups.
	banditZoneCount := 0
	if in.ActiveEventID == "BANDIT_CAMP" && in.ActiveEventRadius > 0 && nowTick < in.ActiveEventEnds && hooks.DistXZ != nil {
		for _, a := range agents {
			if a == nil || a.HP <= 0 {
				continue
			}
			if hooks.DistXZ(a.Pos, in.ActiveEventCenter) <= in.ActiveEventRadius {
				banditZoneCount++
			}
		}
	}

	for _, a := range agents {
		if a == nil {
			continue
		}

		inEventRadius := inEventRange(nowTick, a.Pos, in, hooks.DistXZ)
		rec := StaminaRecovery(in.Weather, a.Hunger, in.ActiveEventID, inEventRadius)

		// Bandit camp damage: when alone, take periodic hits.
		if in.ActiveEventID == "BANDIT_CAMP" && in.ActiveEventRadius > 0 && nowTick < in.ActiveEventEnds &&
			nowTick%50 == 0 && banditZoneCount > 0 && banditZoneCount < 2 &&
			hooks.DistXZ != nil && hooks.DistXZ(a.Pos, in.ActiveEventCenter) <= in.ActiveEventRadius && a.HP > 0 {
			a.HP--
			a.AddEvent(protocol.Event{"t": nowTick, "type": "DAMAGE", "kind": "BANDIT", "hp": a.HP})
		}

		if a.StaminaMilli < 1000 && rec > 0 {
			a.StaminaMilli += rec
			if a.StaminaMilli > 1000 {
				a.StaminaMilli = 1000
			}
		}

		// Downed -> respawn.
		if a.HP <= 0 && hooks.Respawn != nil {
			hooks.Respawn(nowTick, a, "DOWNED")
		}
	}

	// Cleanup: despawn expired dropped items (rate-limited to keep per-tick work low).
	if nowTick%50 == 0 && hooks.CleanupExpired != nil {
		hooks.CleanupExpired(nowTick)
	}
}
