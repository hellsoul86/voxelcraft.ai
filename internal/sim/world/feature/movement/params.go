package movement

func NormalizeTarget(x, z int) (int, int, int) {
	return x, 0, z
}

func ClampFollowDistance(distance float64) float64 {
	if distance <= 0 {
		return 2.0
	}
	if distance > 32 {
		return 32
	}
	return distance
}
