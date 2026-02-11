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
