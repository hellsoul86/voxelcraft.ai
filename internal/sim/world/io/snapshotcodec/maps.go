package snapshotcodec

func PositiveMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	dst := map[string]int{}
	for k, v := range src {
		if k != "" && v > 0 {
			dst[k] = v
		}
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func PositiveNestedMap(src map[string]map[string]int) map[string]map[string]int {
	if len(src) == 0 {
		return nil
	}
	out := map[string]map[string]int{}
	for k, v := range src {
		if k == "" || len(v) == 0 {
			continue
		}
		m := PositiveMap(v)
		if len(m) == 0 {
			continue
		}
		out[k] = m
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
