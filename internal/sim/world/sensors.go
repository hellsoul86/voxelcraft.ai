package world

// sensorOn reports whether the sensor block at pos currently outputs an "ON" signal.
//
// MVP behavior (no configuration UI yet):
// - ON if there is any non-empty dropped item entity on the sensor block or adjacent to it.
// - ON if there is any adjacent container with at least 1 available item (inventory minus reserved).
func (w *World) sensorOn(pos Vec3i) bool {
	if w == nil {
		return false
	}
	if w.blockName(w.chunks.GetBlock(pos)) != "SENSOR" {
		return false
	}

	dirs := []Vec3i{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: -1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: 0, Y: -1, Z: 0},
		{X: 0, Y: 0, Z: 1},
		{X: 0, Y: 0, Z: -1},
	}

	hasLiveItemAt := func(p Vec3i) bool {
		ids := w.itemsAt[p]
		if len(ids) == 0 {
			return false
		}
		for _, id := range ids {
			e := w.items[id]
			if e != nil && e.Item != "" && e.Count > 0 {
				return true
			}
		}
		return false
	}

	for _, d := range dirs {
		p := Vec3i{X: pos.X + d.X, Y: pos.Y + d.Y, Z: pos.Z + d.Z}
		if hasLiveItemAt(p) {
			return true
		}
		if c := w.containers[p]; c != nil && len(c.Inventory) > 0 {
			for item := range c.Inventory {
				if c.availableCount(item) > 0 {
					return true
				}
			}
		}
	}

	return false
}
