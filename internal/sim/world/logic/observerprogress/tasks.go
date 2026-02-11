package observerprogress

import "math"

type Vec3 struct {
	X int
	Y int
	Z int
}

func DistXZ(a, b Vec3) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dz := a.Z - b.Z
	if dz < 0 {
		dz = -dz
	}
	return dx + dz
}

func FollowProgress(agentPos, target Vec3, distance float64) (progress float64, etaTicks int) {
	want := int(math.Ceil(distance))
	if want < 1 {
		want = 1
	}
	d := DistXZ(agentPos, target)
	prog := 0.0
	if d <= want {
		prog = 1.0
	}
	eta := d - want
	if eta < 0 {
		eta = 0
	}
	return prog, eta
}

func MoveProgress(start, current, target Vec3, tolerance float64) (progress float64, etaTicks int) {
	want := int(math.Ceil(tolerance))
	if want < 1 {
		want = 1
	}
	distStart := DistXZ(start, target)
	distCur := DistXZ(current, target)
	totalEff := distStart - want
	if totalEff < 0 {
		totalEff = 0
	}
	remEff := distCur - want
	if remEff < 0 {
		remEff = 0
	}
	prog := 1.0
	if totalEff > 0 {
		prog = float64(totalEff-remEff) / float64(totalEff)
		if prog < 0 {
			prog = 0
		} else if prog > 1 {
			prog = 1
		}
	}
	return prog, remEff
}
