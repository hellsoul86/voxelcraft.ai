package model

type Structure struct {
	StructureID string
	BlueprintID string
	BuilderID   string
	Anchor      Vec3i
	Rotation    int
	Min         Vec3i
	Max         Vec3i

	CompletedTick uint64
	AwardDueTick  uint64
	Awarded       bool

	// Usage: agent_id -> last tick seen inside the structure.
	UsedBy map[string]uint64

	// Influence: last day index we awarded influence for.
	LastInfluenceDay int
}

