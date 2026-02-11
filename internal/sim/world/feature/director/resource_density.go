package director

func ComputeResourceDensity(targets []string, blockIndex map[string]uint16, chunks [][]uint16) map[string]float64 {
	out := map[string]float64{}
	for _, name := range targets {
		out[name] = 0
	}
	if len(targets) == 0 || len(chunks) == 0 || len(blockIndex) == 0 {
		return out
	}

	idToName := map[uint16]string{}
	for _, name := range targets {
		if id, ok := blockIndex[name]; ok {
			idToName[id] = name
		}
	}
	if len(idToName) == 0 {
		return out
	}

	counts := map[string]int{}
	total := 0
	for _, blocks := range chunks {
		if len(blocks) == 0 {
			continue
		}
		for _, b := range blocks {
			total++
			if name, ok := idToName[b]; ok {
				counts[name]++
			}
		}
	}
	if total == 0 {
		return out
	}
	denom := float64(total)
	for _, name := range targets {
		out[name] = float64(counts[name]) / denom
	}
	return out
}
