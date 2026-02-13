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

func BoardIDAt(pos modelpkg.Vec3i) string {
	return modelpkg.ContainerID("BULLETIN_BOARD", pos)
}

func EnsureBoard(boards map[string]*Board, pos modelpkg.Vec3i) *Board {
	if boards == nil {
		return nil
	}
	id := BoardIDAt(pos)
	if b := boards[id]; b != nil {
		return b
	}
	b := &Board{BoardID: id}
	boards[id] = b
	return b
}

func RemoveBoard(boards map[string]*Board, pos modelpkg.Vec3i) {
	if boards == nil {
		return
	}
	delete(boards, BoardIDAt(pos))
}
