package events

type MineOutcomeResult struct {
	OK         bool
	GrantItem  string
	GrantCount int
	GoalKind   string
	Narrative  int
}

func MineOutcome(eventID string, blockName string) MineOutcomeResult {
	switch eventID {
	case "CRYSTAL_RIFT":
		if blockName != "CRYSTAL_ORE" {
			return MineOutcomeResult{}
		}
		return MineOutcomeResult{
			OK:         true,
			GrantItem:  "CRYSTAL_SHARD",
			GrantCount: 1,
			GoalKind:   "MINE_CRYSTAL",
			Narrative:  5,
		}
	case "DEEP_VEIN":
		switch blockName {
		case "IRON_ORE", "COPPER_ORE":
			return MineOutcomeResult{
				OK:         true,
				GrantItem:  blockName,
				GrantCount: 1,
				GoalKind:   "MINE_VEIN",
				Narrative:  5,
			}
		default:
			return MineOutcomeResult{}
		}
	default:
		return MineOutcomeResult{}
	}
}

type OpenOutcomeResult struct {
	OK        bool
	GoalKind  string
	Narrative int
	Risk      int
}

func OpenContainerOutcome(eventID string, containerType string) OpenOutcomeResult {
	if containerType != "CHEST" {
		return OpenOutcomeResult{}
	}
	switch eventID {
	case "RUINS_GATE":
		return OpenOutcomeResult{
			OK:        true,
			GoalKind:  "OPEN_RUINS",
			Narrative: 12,
		}
	case "BANDIT_CAMP":
		return OpenOutcomeResult{
			OK:        true,
			GoalKind:  "LOOT_BANDITS",
			Narrative: 8,
			Risk:      10,
		}
	default:
		return OpenOutcomeResult{}
	}
}
