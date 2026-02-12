package boards

import (
	"fmt"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type Post = modelpkg.BoardPost
type Board = modelpkg.Board

func NewPostID(n uint64) string {
	return fmt.Sprintf("P%06d", n)
}
