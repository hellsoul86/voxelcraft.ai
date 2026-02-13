package runtime

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

func SensorOn(
	pos modelpkg.Vec3i,
	blockNameAt func(modelpkg.Vec3i) string,
	itemIDsAt func(modelpkg.Vec3i) []string,
	loadItem func(string) (ItemEntry, bool),
	containerHasAvailable func(modelpkg.Vec3i) bool,
) bool {
	if blockNameAt == nil || blockNameAt(pos) != "SENSOR" {
		return false
	}

	hasLiveItemAt := func(p modelpkg.Vec3i) bool {
		if itemIDsAt == nil || loadItem == nil {
			return false
		}
		return HasLiveItem(itemIDsAt(p), loadItem)
	}

	for _, d := range SensorNeighborOffsets() {
		p := modelpkg.Vec3i{X: pos.X + d.X, Y: pos.Y + d.Y, Z: pos.Z + d.Z}
		if hasLiveItemAt(p) {
			return true
		}
		if containerHasAvailable != nil && containerHasAvailable(p) {
			return true
		}
	}
	return false
}
