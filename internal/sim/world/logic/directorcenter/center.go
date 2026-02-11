package directorcenter

type Pos struct {
	X int
	Y int
	Z int
}

func PickEventCenter(seed int64, boundary int, nowTick uint64, eventID string, occupied func(Pos) bool) Pos {
	if boundary <= 0 {
		boundary = 4000
	}
	margin := 64
	span := boundary*2 - margin*2
	if span <= 0 {
		margin = 0
		span = boundary * 2
		if span <= 0 {
			span = 1
		}
	}

	eh := HashEventID(eventID)
	for attempt := 0; attempt < 32; attempt++ {
		hx := hash3(seed, eh, int(nowTick), attempt*2)
		hz := hash3(seed, eh, int(nowTick), attempt*2+1)
		x := -boundary + margin + int(hx%uint64(span))
		z := -boundary + margin + int(hz%uint64(span))

		p := Pos{X: x, Y: 0, Z: z}
		if occupied != nil && occupied(p) {
			continue
		}
		return p
	}

	// Fallback (deterministic).
	return Pos{X: 0, Y: 0, Z: 0}
}

func HashEventID(id string) int {
	// FNV-1a 64-bit, folded to int.
	var h uint64 = 1469598103934665603
	for i := 0; i < len(id); i++ {
		h ^= uint64(id[i])
		h *= 1099511628211
	}
	return int(uint32(h))
}

func mix64(z uint64) uint64 {
	z += 0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func hash3(seed int64, x, y, z int) uint64 {
	ux := uint64(uint32(int32(x)))
	uy := uint64(uint32(int32(y)))
	uz := uint64(uint32(int32(z)))
	v := uint64(seed) ^ (ux * 0x9e3779b97f4a7c15) ^ (uy * 0xc2b2ae3d27d4eb4f) ^ (uz * 0xbf58476d1ce4e5b9)
	return mix64(v)
}
