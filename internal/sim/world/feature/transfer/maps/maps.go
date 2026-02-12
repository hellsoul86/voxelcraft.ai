package maps

func CopyPositiveIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return map[string]int{}
	}
	dst := make(map[string]int, len(src))
	for k, v := range src {
		if k == "" || v <= 0 {
			continue
		}
		dst[k] = v
	}
	return dst
}

func CopyMap[K comparable, V any](src map[K]V, keep func(K, V) bool) map[K]V {
	if len(src) == 0 {
		return map[K]V{}
	}
	dst := make(map[K]V, len(src))
	for k, v := range src {
		if keep != nil && !keep(k, v) {
			continue
		}
		dst[k] = v
	}
	return dst
}
