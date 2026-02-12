package limits

func ClampBlocksPerTick(limit int) int {
	if limit <= 0 {
		return 2
	}
	return limit
}
