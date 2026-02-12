package model

// Sign stores the text associated with a SIGN block.
//
// This is authoritative sim state and is included in snapshots/digests.
type Sign struct {
	Pos         Vec3i
	Text        string
	UpdatedTick uint64
	UpdatedBy   string
}

// ConveyorMeta stores minimal runtime metadata for a CONVEYOR block.
// We keep it intentionally small and deterministic: a single cardinal direction.
type ConveyorMeta struct {
	DX int8 // -1,0,1
	DZ int8 // -1,0,1
}
