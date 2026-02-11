package world

import (
	"fmt"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/feature/session"
)

func handleInstantSay(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.Text == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing text"))
		return
	}
	ch, ok := session.NormalizeChatChannel(inst.Channel)
	if !ok {
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

	rl := session.ChatLimitSpec(ch, session.ChatRateLimits{
		SayWindowTicks:       uint64(w.cfg.RateLimits.SayWindowTicks),
		SayMax:               w.cfg.RateLimits.SayMax,
		MarketSayWindowTicks: uint64(w.cfg.RateLimits.MarketSayWindowTicks),
		MarketSayMax:         w.cfg.RateLimits.MarketSayMax,
	})
	if ok, cd := a.RateLimitAllow(rl.Kind, nowTick, rl.Window, rl.Max); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", rl.RateErrMsg)
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
	n := session.NormalizeConsumeCount(inst.Count)
	def, ok := w.catalogs.Items.Defs[inst.ItemID]
	if !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown item"))
		return
	}
	if !session.IsFood(def.Kind, def.EdibleHP) {
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
	}
	next := session.ApplyFood(session.EatState{
		HP:           a.HP,
		Hunger:       a.Hunger,
		StaminaMilli: a.StaminaMilli,
	}, def.EdibleHP, n)
	a.HP = next.HP
	a.Hunger = next.Hunger
	a.StaminaMilli = next.StaminaMilli
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantSaveMemory(a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.Key == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing key"))
		return
	}
	mem := map[string]string{}
	for k, v := range a.Memory {
		mem[k] = v.Value
	}
	// Enforce a very small budget (64KB total).
	if session.OverMemoryBudget(mem, inst.Key, inst.Value, 64*1024) {
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
