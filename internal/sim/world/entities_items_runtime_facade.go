package world

import (
	"fmt"

	itemspkg "voxelcraft.ai/internal/sim/world/feature/entities/items"
)

const itemEntityTTLTicksDefault = itemspkg.EntityTTLTicksDefault

func (w *World) newItemEntityID() string {
	n := w.nextItemNum.Add(1)
	return fmt.Sprintf("IT%06d", n)
}

func (w *World) canPickupItemEntity(agentID string, pos Vec3i) bool {
	land := w.landAt(pos)
	if land == nil {
		return true
	}
	// Maintenance downgrade: once protection is lost, treat as "wild" for pickup.
	if land.MaintenanceStage >= 2 && !w.isLandMember(agentID, land) {
		return true
	}
	return w.isLandMember(agentID, land)
}

func (w *World) spawnItemEntity(nowTick uint64, actor string, pos Vec3i, item string, count int, reason string) string {
	return itemspkg.Spawn(
		nowTick,
		actor,
		pos,
		item,
		count,
		reason,
		w.items,
		w.itemsAt,
		w.newItemEntityID,
		w.auditEvent,
	)
}

func (w *World) removeItemEntity(nowTick uint64, actor string, id string, reason string) {
	itemspkg.Remove(nowTick, actor, id, reason, w.items, w.itemsAt, w.auditEvent)
}

func (w *World) moveItemEntity(nowTick uint64, actor string, id string, to Vec3i, reason string) {
	itemspkg.Move(nowTick, actor, id, to, reason, w.items, w.itemsAt, w.auditEvent)
}

func (w *World) cleanupExpiredItemEntities(nowTick uint64) {
	itemspkg.CleanupExpired(nowTick, w.items, w.itemsAt, w.removeItemEntity)
}
