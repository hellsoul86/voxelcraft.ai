package spawns

import "sort"

type Pos struct {
	X int
	Y int
	Z int
}

func Square(center Pos, radius int) []Pos {
	if radius < 0 {
		radius = 0
	}
	out := make([]Pos, 0, (2*radius+1)*(2*radius+1))
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			out = append(out, Pos{X: center.X + dx, Y: center.Y, Z: center.Z + dz})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].Z < out[j].Z
	})
	return out
}

func Diamond(center Pos, manhattan int) []Pos {
	if manhattan < 0 {
		manhattan = 0
	}
	out := make([]Pos, 0, (2*manhattan+1)*(2*manhattan+1))
	for dz := -manhattan; dz <= manhattan; dz++ {
		for dx := -manhattan; dx <= manhattan; dx++ {
			abs := dx
			if abs < 0 {
				abs = -abs
			}
			abs2 := dz
			if abs2 < 0 {
				abs2 = -abs2
			}
			if abs+abs2 > manhattan {
				continue
			}
			out = append(out, Pos{X: center.X + dx, Y: center.Y, Z: center.Z + dz})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].Z < out[j].Z
	})
	return out
}

// RingSquare returns all cells on the square ring at exact radius.
func RingSquare(center Pos, radius int) []Pos {
	if radius <= 0 {
		return []Pos{center}
	}
	out := make([]Pos, 0, radius*8)
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			adx := dx
			if adx < 0 {
				adx = -adx
			}
			adz := dz
			if adz < 0 {
				adz = -adz
			}
			if adx != radius && adz != radius {
				continue
			}
			out = append(out, Pos{X: center.X + dx, Y: center.Y, Z: center.Z + dz})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].Z < out[j].Z
	})
	return out
}

func DeepVeinIsCopper(x, z int) bool {
	return (x+z)&1 == 0
}

type BlockPlacement struct {
	Pos   Pos
	Block string
}

type LootContainer struct {
	Pos   Pos
	Type  string
	Items map[string]int
}

type SignPlacement struct {
	Pos  Pos
	Text string
}

type BoardPost struct {
	Pos    Pos
	Author string
	Title  string
	Body   string
}

type Plan struct {
	Placements []BlockPlacement
	Containers []LootContainer
	Signs      []SignPlacement
	BoardPosts []BoardPost
	Center     *Pos
}

func CrystalRiftPlan(center Pos) Plan {
	plan := Plan{Center: &Pos{X: center.X, Y: center.Y, Z: center.Z}}
	for _, p := range Square(center, 2) {
		plan.Placements = append(plan.Placements, BlockPlacement{Pos: p, Block: "CRYSTAL_ORE"})
	}
	return plan
}

func DeepVeinPlan(center Pos) Plan {
	plan := Plan{Center: &Pos{X: center.X, Y: center.Y, Z: center.Z}}
	for _, p := range Square(center, 3) {
		block := "IRON_ORE"
		if DeepVeinIsCopper(p.X-center.X, p.Z-center.Z) {
			block = "COPPER_ORE"
		}
		plan.Placements = append(plan.Placements, BlockPlacement{Pos: p, Block: block})
	}
	return plan
}

func RuinsGatePlan(center Pos) Plan {
	plan := Plan{Center: &Pos{X: center.X, Y: center.Y, Z: center.Z}}
	for _, p := range Square(center, 1) {
		block := "BRICK"
		if p.X == center.X && p.Z == center.Z {
			block = "CHEST"
		}
		plan.Placements = append(plan.Placements, BlockPlacement{Pos: p, Block: block})
	}
	plan.Containers = append(plan.Containers, LootContainer{
		Pos:  center,
		Type: "CHEST",
		Items: map[string]int{
			"CRYSTAL_SHARD": 2,
			"IRON_INGOT":    4,
			"COPPER_INGOT":  4,
		},
	})
	return plan
}

func EventNoticeBoardPlan(center Pos, headline string, body string) Plan {
	boardPos := center
	signPos := Pos{X: center.X + 1, Y: center.Y, Z: center.Z}
	plan := Plan{Center: &Pos{X: center.X, Y: center.Y, Z: center.Z}}
	plan.Placements = append(plan.Placements, BlockPlacement{Pos: boardPos, Block: "BULLETIN_BOARD"})
	plan.Placements = append(plan.Placements, BlockPlacement{Pos: signPos, Block: "SIGN"})
	plan.Signs = append(plan.Signs, SignPlacement{Pos: signPos, Text: headline})
	plan.BoardPosts = append(plan.BoardPosts, BoardPost{
		Pos:    boardPos,
		Author: "WORLD",
		Title:  headline,
		Body:   body,
	})
	return plan
}

func FloodWarningPlan(center Pos) Plan {
	plan := Plan{Center: &Pos{X: center.X, Y: center.Y, Z: center.Z}}
	for _, p := range Square(center, 2) {
		plan.Placements = append(plan.Placements, BlockPlacement{Pos: p, Block: "WATER"})
	}
	return plan
}

func BlightZonePlan(center Pos) Plan {
	plan := Plan{Center: &Pos{X: center.X, Y: center.Y, Z: center.Z}}
	for _, p := range Diamond(center, 4) {
		dx := p.X - center.X
		dz := p.Z - center.Z
		if absInt(dx) > 3 || absInt(dz) > 3 {
			continue
		}
		plan.Placements = append(plan.Placements, BlockPlacement{Pos: p, Block: "GRAVEL"})
	}
	return plan
}

func BanditCampPlan(center Pos) Plan {
	plan := Plan{Center: &Pos{X: center.X, Y: center.Y, Z: center.Z}}
	for _, p := range Square(center, 2) {
		dx := p.X - center.X
		dz := p.Z - center.Z
		block := "AIR"
		if dx == 0 && dz == 0 {
			block = "CHEST"
		} else if absInt(dx) == 2 || absInt(dz) == 2 {
			block = "BRICK"
		}
		plan.Placements = append(plan.Placements, BlockPlacement{Pos: p, Block: block})
	}
	plan.Containers = append(plan.Containers, LootContainer{
		Pos:  center,
		Type: "CHEST",
		Items: map[string]int{
			"IRON_INGOT":    6,
			"COPPER_INGOT":  4,
			"CRYSTAL_SHARD": 1,
			"BREAD":         2,
		},
	})
	signPos := Pos{X: center.X + 3, Y: center.Y, Z: center.Z}
	plan.Placements = append(plan.Placements, BlockPlacement{Pos: signPos, Block: "SIGN"})
	plan.Signs = append(plan.Signs, SignPlacement{Pos: signPos, Text: "BANDIT CAMP"})
	return plan
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
