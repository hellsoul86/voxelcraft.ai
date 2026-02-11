package world

import (
	"fmt"
	"strings"

	"voxelcraft.ai/internal/protocol"
)

func handleInstantSay(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.Text == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing text"))
		return
	}
	ch := strings.ToUpper(strings.TrimSpace(inst.Channel))
	if ch == "" {
		ch = "LOCAL"
	}
	switch ch {
	case "LOCAL", "CITY", "MARKET":
	default:
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid channel"))
		return
	}
	if ch == "CITY" {
		if a.OrgID == "" || !w.isOrgMember(a.ID, a.OrgID) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not in org"))
			return
		}
	}
	if ch == "MARKET" {
		if !w.cfg.AllowTrade {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "market disabled in this world"))
			return
		}
		if _, perms := w.permissionsFor(a.ID, a.Pos); !perms["can_trade"] {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "market chat not allowed here"))
			return
		}
	}

	rlKind := "SAY"
	window := uint64(w.cfg.RateLimits.SayWindowTicks)
	max := w.cfg.RateLimits.SayMax
	msg := "too many SAY"
	if ch == "MARKET" {
		rlKind = "SAY_MARKET"
		window = uint64(w.cfg.RateLimits.MarketSayWindowTicks)
		max = w.cfg.RateLimits.MarketSayMax
		msg = "too many SAY (MARKET)"
	}
	if ok, cd := a.RateLimitAllow(rlKind, nowTick, window, max); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", msg)
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}

	w.broadcastChat(nowTick, a, ch, inst.Text)
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantWhisper(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, cd := a.RateLimitAllow("WHISPER", nowTick, uint64(w.cfg.RateLimits.WhisperWindowTicks), w.cfg.RateLimits.WhisperMax); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many WHISPER")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	if inst.To == "" || inst.Text == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing to/text"))
		return
	}
	to := w.agents[inst.To]
	if to == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	to.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "CHAT",
		"from":    a.ID,
		"channel": "WHISPER",
		"text":    inst.Text,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantEat(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.ItemID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing item_id"))
		return
	}
	n := inst.Count
	if n <= 0 {
		n = 1
	}
	def, ok := w.catalogs.Items.Defs[inst.ItemID]
	if !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown item"))
		return
	}
	if def.Kind != "FOOD" || def.EdibleHP <= 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "item not edible"))
		return
	}
	if a.Inventory[inst.ItemID] < n {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing food"))
		return
	}
	for i := 0; i < n; i++ {
		a.Inventory[inst.ItemID]--
		if a.Inventory[inst.ItemID] <= 0 {
			delete(a.Inventory, inst.ItemID)
		}
		a.HP += def.EdibleHP
		if a.HP > 20 {
			a.HP = 20
		}
		hg := def.EdibleHP * 2
		if hg < 1 {
			hg = 1
		}
		a.Hunger += hg
		if a.Hunger > 20 {
			a.Hunger = 20
		}
		a.StaminaMilli += def.EdibleHP * 50
		if a.StaminaMilli > 1000 {
			a.StaminaMilli = 1000
		}
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantSaveMemory(a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.Key == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing key"))
		return
	}
	// Enforce a very small budget (64KB total).
	if overMemoryBudget(a.Memory, inst.Key, inst.Value, 64*1024) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "memory budget exceeded"))
		return
	}
	a.MemorySave(inst.Key, inst.Value, inst.TTLTicks, nowTick)
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantLoadMemory(a *Agent, inst protocol.InstantReq, nowTick uint64) {
	kvs := a.MemoryLoad(inst.Prefix, inst.Limit, nowTick)
	a.PendingMemory = kvs
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", fmt.Sprintf("loaded %d keys", len(kvs))))
}
