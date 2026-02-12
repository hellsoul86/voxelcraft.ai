package model

// ItemEntity is a dropped item stack in the world (e.g. from mining/respawn drops).
// It is part of the authoritative sim state and must be snapshot/digest'd.
type ItemEntity struct {
	EntityID    string
	Pos         Vec3i
	Item        string
	Count       int
	CreatedTick uint64
	ExpiresTick uint64
}

func (e *ItemEntity) ID() string { return e.EntityID }

