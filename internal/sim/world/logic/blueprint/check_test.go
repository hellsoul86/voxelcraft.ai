package blueprint

import "testing"

func TestCheckPlaced(t *testing.T) {
	index := map[string]uint16{
		"PLANK": 1,
		"STONE": 2,
	}
	blocks := []PlacementBlock{
		{Pos: [3]int{0, 0, 0}, Block: "PLANK"},
		{Pos: [3]int{1, 0, 0}, Block: "STONE"},
	}
	grid := map[[3]int]uint16{
		{10, 0, 10}: 1,
		{11, 0, 10}: 2,
	}
	ok := CheckPlaced(func(x, y, z int) uint16 {
		return grid[[3]int{x, y, z}]
	}, index, blocks, [3]int{10, 0, 10}, 0)
	if !ok {
		t.Fatalf("expected placed blueprint to validate")
	}
}
