package runtime

type PlacementEffects struct {
	ContainerType  string
	EnsureBoard    bool
	EnsureSign     bool
	EnsureConveyor bool
	ConveyorDX     int
	ConveyorDZ     int
	EnsureSwitch   bool
	SwitchOn       bool
}

func EffectsForPlacedBlock(blockName string) PlacementEffects {
	switch blockName {
	case "CHEST", "FURNACE", "CONTRACT_TERMINAL":
		return PlacementEffects{ContainerType: blockName}
	case "BULLETIN_BOARD":
		return PlacementEffects{EnsureBoard: true}
	case "SIGN":
		return PlacementEffects{EnsureSign: true}
	case "CONVEYOR":
		// Blueprint placements don't have a notion of placement yaw yet, so default to +X.
		return PlacementEffects{
			EnsureConveyor: true,
			ConveyorDX:     1,
			ConveyorDZ:     0,
		}
	case "SWITCH":
		return PlacementEffects{
			EnsureSwitch: true,
			SwitchOn:     false,
		}
	default:
		return PlacementEffects{}
	}
}
