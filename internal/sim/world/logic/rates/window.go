package rates

func Allow(nowTick uint64, startTick uint64, count int, window uint64, max int) (newStart uint64, newCount int, ok bool, cooldownTicks uint64) {
	newStart = startTick
	newCount = count
	if window == 0 || max <= 0 {
		return newStart, newCount, true, 0
	}

	if nowTick-newStart >= window {
		newStart = nowTick
		newCount = 0
	}
	newCount++
	if newCount <= max {
		return newStart, newCount, true, 0
	}
	return newStart, newCount, false, (newStart + window) - nowTick
}
