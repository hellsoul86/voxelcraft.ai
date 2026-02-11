package blueprint

// NormalizeRotation converts a client-provided rotation value into a stable
// quarter-turn count in [0,3].
//
// It accepts either quarter-turns (0..3) or degrees (multiples of 90).
func NormalizeRotation(r int) int {
	// Treat large multiples of 90 as degrees.
	if r%90 == 0 && (r > 3 || r < -3) {
		r = r / 90
	}
	r %= 4
	if r < 0 {
		r += 4
	}
	return r
}

// RotateXZ rotates an (x,z) offset around the Y axis by rot*90 degrees
// clockwise. rot must be a normalized quarter-turn count in [0,3].
func RotateXZ(x, z, rot int) (rx, rz int) {
	switch rot & 3 {
	case 0:
		return x, z
	case 1:
		return z, -x
	case 2:
		return -x, -z
	default: // 3
		return -z, x
	}
}

func RotateOffset(off [3]int, rot int) [3]int {
	rx, rz := RotateXZ(off[0], off[2], rot)
	return [3]int{rx, off[1], rz}
}
