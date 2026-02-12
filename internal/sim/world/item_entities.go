package world

import (
	"fmt"

	itemspkg "voxelcraft.ai/internal/sim/world/feature/entities/items"
)

// ItemEntity is a dropped item stack in the world (e.g. from respawn drops).
// It is part of the authoritative sim state and must be snapshot/digest'd.
type ItemEntity struct {
	EntityID    string
	Pos         Vec3i
	Item        string
	Count       int
	CreatedTick uint64
	ExpiresTick uint64
}

func (e *ItemEntity) ID() string { return e.EntityID }

const itemEntityTTLTicksDefault = 6000 // ~1 in-game day at 5Hz/6000 day ticks

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
	if item == "" || count <= 0 {
		return ""
	}

	// Merge into an existing entity at the same position when possible.
	if ids := w.itemsAt[pos]; len(ids) > 0 {
		mergeID, ok := itemspkg.FindMergeTarget(ids, item, func(id string) (itemspkg.Entry, bool) {
			e := w.items[id]
			if e == nil {
				return itemspkg.Entry{}, false
			}
			return itemspkg.Entry{
				ID:          e.EntityID,
				Item:        e.Item,
				Count:       e.Count,
				ExpiresTick: e.ExpiresTick,
			}, true
		})
		if ok {
			e := w.items[mergeID]
			e.Count += count
			exp := nowTick + itemEntityTTLTicksDefault
			if exp > e.ExpiresTick {
				e.ExpiresTick = exp
			}
			w.auditEvent(nowTick, actor, "ITEM_SPAWN", pos, reason, map[string]any{
				"entity_id": e.EntityID,
				"item":      item,
				"count":     count,
				"merged":    true,
			})
			return e.EntityID
		}
	}

	id := w.newItemEntityID()
	e := &ItemEntity{
		EntityID:    id,
		Pos:         pos,
		Item:        item,
		Count:       count,
		CreatedTick: nowTick,
		ExpiresTick: nowTick + itemEntityTTLTicksDefault,
	}
	w.items[id] = e
	w.itemsAt[pos] = append(w.itemsAt[pos], id)
	w.auditEvent(nowTick, actor, "ITEM_SPAWN", pos, reason, map[string]any{
		"entity_id": id,
		"item":      item,
		"count":     count,
		"merged":    false,
	})
	return id
}

func (w *World) removeItemEntity(nowTick uint64, actor string, id string, reason string) {
	e := w.items[id]
	if e == nil {
		return
	}
	delete(w.items, id)
	ids := itemspkg.RemoveID(w.itemsAt[e.Pos], id)
	if len(ids) == 0 {
		delete(w.itemsAt, e.Pos)
	} else {
		w.itemsAt[e.Pos] = ids
	}
	w.auditEvent(nowTick, actor, "ITEM_DESPAWN", e.Pos, reason, map[string]any{
		"entity_id": id,
		"item":      e.Item,
		"count":     e.Count,
	})
}

func (w *World) moveItemEntity(nowTick uint64, actor string, id string, to Vec3i, reason string) {
	e := w.items[id]
	if e == nil {
		return
	}
	from := e.Pos
	if from == to {
		return
	}

	// Remove from old position list.
	ids := itemspkg.RemoveID(w.itemsAt[from], id)
	if len(ids) == 0 {
		delete(w.itemsAt, from)
	} else {
		w.itemsAt[from] = ids
	}

	// Add to new position list.
	w.itemsAt[to] = append(w.itemsAt[to], id)
	e.Pos = to

	w.auditEvent(nowTick, actor, "ITEM_MOVE", from, reason, map[string]any{
		"entity_id": id,
		"to":        to.ToArray(),
		"item":      e.Item,
		"count":     e.Count,
	})
}

func (w *World) cleanupExpiredItemEntities(nowTick uint64) {
	if len(w.items) == 0 {
		return
	}
	ids := make([]string, 0, len(w.items))
	for id := range w.items {
		ids = append(ids, id)
	}
	expired := itemspkg.SortedExpired(ids, func(id string) (itemspkg.Entry, bool) {
		e := w.items[id]
		if e == nil {
			return itemspkg.Entry{}, false
		}
		return itemspkg.Entry{
			ID:          e.EntityID,
			Item:        e.Item,
			Count:       e.Count,
			ExpiresTick: e.ExpiresTick,
		}, true
	}, nowTick)
	if len(expired) == 0 {
		return
	}
	for _, id := range expired {
		w.removeItemEntity(nowTick, "WORLD", id, "EXPIRE")
	}
}
