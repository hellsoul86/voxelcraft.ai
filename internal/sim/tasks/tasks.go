package tasks

type Kind string

const (
	KindMoveTo         Kind = "MOVE_TO"
	KindMine           Kind = "MINE"
	KindPlace          Kind = "PLACE"
	KindOpen           Kind = "OPEN"
	KindTransfer       Kind = "TRANSFER"
	KindCraft          Kind = "CRAFT"
	KindSmelt          Kind = "SMELT"
	KindBuildBlueprint Kind = "BUILD_BLUEPRINT"
)

type MovementTask struct {
	TaskID      string
	Kind        Kind
	Target      Vec3i
	Tolerance   float64
	StartPos    Vec3i
	StartedTick uint64
}

type WorkTask struct {
	TaskID string
	Kind   Kind

	// MINE
	BlockPos Vec3i
	// CRAFT/SMELT
	RecipeID string
	ItemID   string
	Count    int

	// BUILD
	BlueprintID string
	Anchor      Vec3i
	Rotation    int
	BuildIndex  int // next block index to place

	// OPEN/TRANSFER
	TargetID     string
	SrcContainer string
	DstContainer string

	StartedTick uint64
	WorkTicks   int // elapsed ticks on current unit of work
}

// Vec3i is duplicated here to avoid import cycles (tasks is used by world).
type Vec3i struct{ X, Y, Z int }
