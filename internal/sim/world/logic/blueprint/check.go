package blueprint

type BlockGetter func(x, y, z int) uint16

type PlacementBlock struct {
	Pos   [3]int
	Block string
}

func CheckPlaced(getBlock BlockGetter, blockIndex map[string]uint16, blocks []PlacementBlock, anchor [3]int, rotation int) bool {
	if getBlock == nil || len(blocks) == 0 {
		return false
	}
	rot := NormalizeRotation(rotation)
	for _, b := range blocks {
		want, ok := blockIndex[b.Block]
		if !ok {
			return false
		}
		off := RotateOffset(b.Pos, rot)
		x := anchor[0] + off[0]
		y := anchor[1] + off[1]
		z := anchor[2] + off[2]
		if getBlock(x, y, z) != want {
			return false
		}
	}
	return true
}
