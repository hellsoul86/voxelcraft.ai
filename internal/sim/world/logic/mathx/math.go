package mathx

func FloorDiv(a, b int) int {
	// b > 0
	q := a / b
	r := a % b
	if r < 0 {
		q--
	}
	return q
}

func Mod(a, b int) int {
	// b > 0
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}

func AbsInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func mix64(z uint64) uint64 {
	z += 0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func Hash2(seed int64, x, z int) uint64 {
	ux := uint64(uint32(int32(x)))
	uz := uint64(uint32(int32(z)))
	v := uint64(seed) ^ (ux * 0x9e3779b97f4a7c15) ^ (uz * 0xbf58476d1ce4e5b9)
	return mix64(v)
}

func Hash3(seed int64, x, y, z int) uint64 {
	ux := uint64(uint32(int32(x)))
	uy := uint64(uint32(int32(y)))
	uz := uint64(uint32(int32(z)))
	v := uint64(seed) ^ (ux * 0x9e3779b97f4a7c15) ^ (uy * 0xc2b2ae3d27d4eb4f) ^ (uz * 0xbf58476d1ce4e5b9)
	return mix64(v)
}
