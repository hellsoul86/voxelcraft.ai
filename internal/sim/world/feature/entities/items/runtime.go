package items

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

const EntityTTLTicksDefault = 6000 // ~1 in-game day at 5Hz/6000 day ticks

type AuditFunc func(nowTick uint64, actor, action string, pos modelpkg.Vec3i, reason string, details map[string]any)

func Spawn(
	nowTick uint64,
	actor string,
	pos modelpkg.Vec3i,
	item string,
	count int,
	reason string,
	items map[string]*modelpkg.ItemEntity,
	itemsAt map[modelpkg.Vec3i][]string,
	newID func() string,
	audit AuditFunc,
) string {
	if item == "" || count <= 0 {
		return ""
	}

	// Merge into an existing entity at the same position when possible.
	if ids := itemsAt[pos]; len(ids) > 0 {
		mergeID, ok := FindMergeTarget(ids, item, func(id string) (Entry, bool) {
			e := items[id]
			if e == nil {
				return Entry{}, false
			}
			return Entry{
				ID:          e.EntityID,
				Item:        e.Item,
				Count:       e.Count,
				ExpiresTick: e.ExpiresTick,
			}, true
		})
		if ok {
			e := items[mergeID]
			e.Count += count
			exp := nowTick + EntityTTLTicksDefault
			if exp > e.ExpiresTick {
				e.ExpiresTick = exp
			}
			if audit != nil {
				audit(nowTick, actor, "ITEM_SPAWN", pos, reason, map[string]any{
					"entity_id": e.EntityID,
					"item":      item,
					"count":     count,
					"merged":    true,
				})
			}
			return e.EntityID
		}
	}

	if newID == nil {
		return ""
	}
	id := newID()
	e := &modelpkg.ItemEntity{
		EntityID:    id,
		Pos:         pos,
		Item:        item,
		Count:       count,
		CreatedTick: nowTick,
		ExpiresTick: nowTick + EntityTTLTicksDefault,
	}
	items[id] = e
	itemsAt[pos] = append(itemsAt[pos], id)
	if audit != nil {
		audit(nowTick, actor, "ITEM_SPAWN", pos, reason, map[string]any{
			"entity_id": id,
			"item":      item,
			"count":     count,
			"merged":    false,
		})
	}
	return id
}

func Remove(
	nowTick uint64,
	actor string,
	id string,
	reason string,
	items map[string]*modelpkg.ItemEntity,
	itemsAt map[modelpkg.Vec3i][]string,
	audit AuditFunc,
) {
	e := items[id]
	if e == nil {
		return
	}
	delete(items, id)
	ids := RemoveID(itemsAt[e.Pos], id)
	if len(ids) == 0 {
		delete(itemsAt, e.Pos)
	} else {
		itemsAt[e.Pos] = ids
	}
	if audit != nil {
		audit(nowTick, actor, "ITEM_DESPAWN", e.Pos, reason, map[string]any{
			"entity_id": id,
			"item":      e.Item,
			"count":     e.Count,
		})
	}
}

func Move(
	nowTick uint64,
	actor string,
	id string,
	to modelpkg.Vec3i,
	reason string,
	items map[string]*modelpkg.ItemEntity,
	itemsAt map[modelpkg.Vec3i][]string,
	audit AuditFunc,
) {
	e := items[id]
	if e == nil {
		return
	}
	from := e.Pos
	if from == to {
		return
	}

	// Remove from old position list.
	ids := RemoveID(itemsAt[from], id)
	if len(ids) == 0 {
		delete(itemsAt, from)
	} else {
		itemsAt[from] = ids
	}

	// Add to new position list.
	itemsAt[to] = append(itemsAt[to], id)
	e.Pos = to

	if audit != nil {
		audit(nowTick, actor, "ITEM_MOVE", from, reason, map[string]any{
			"entity_id": id,
			"to":        to.ToArray(),
			"item":      e.Item,
			"count":     e.Count,
		})
	}
}

func CleanupExpired(
	nowTick uint64,
	items map[string]*modelpkg.ItemEntity,
	itemsAt map[modelpkg.Vec3i][]string,
	remove func(nowTick uint64, actor string, id string, reason string),
) {
	if len(items) == 0 || remove == nil {
		return
	}
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	expired := SortedExpired(ids, func(id string) (Entry, bool) {
		e := items[id]
		if e == nil {
			return Entry{}, false
		}
		return Entry{
			ID:          e.EntityID,
			Item:        e.Item,
			Count:       e.Count,
			ExpiresTick: e.ExpiresTick,
		}, true
	}, nowTick)
	for _, id := range expired {
		remove(nowTick, "WORLD", id, "EXPIRE")
	}
}
