package instants

import (
	"fmt"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	chatpkg "voxelcraft.ai/internal/sim/world/feature/session/chat"
	eatpkg "voxelcraft.ai/internal/sim/world/feature/session/eat"
	memorypkg "voxelcraft.ai/internal/sim/world/feature/session/memory"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type ActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type SayRateLimits struct {
	SayWindowTicks       int
	SayMax               int
	MarketSayWindowTicks int
	MarketSayMax         int
}

type WhisperLimits struct {
	WindowTicks int
	Max         int
}

type WorldEnv interface {
	IsOrgMember(agentID, orgID string) bool
	PermissionsFor(agentID string, pos modelpkg.Vec3i) map[string]bool
	BroadcastChat(nowTick uint64, from *modelpkg.Agent, channel, text string)
	AgentByID(agentID string) *modelpkg.Agent
}

func HandleSay(env WorldEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, allowTrade bool, limits SayRateLimits) {
	if ok, code, msg := ValidateSayInput(inst.Text); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	ch, ok := chatpkg.NormalizeChatChannel(inst.Channel)
	if !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid channel"))
		return
	}
	if ch == "CITY" {
		if a.OrgID == "" || env == nil || !env.IsOrgMember(a.ID, a.OrgID) {
			a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "not in org"))
			return
		}
	}
	if ch == "MARKET" {
		if !allowTrade {
			a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "market disabled in this world"))
			return
		}
		perms := map[string]bool{}
		if env != nil {
			perms = env.PermissionsFor(a.ID, a.Pos)
		}
		if !perms["can_trade"] {
			a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "market chat not allowed here"))
			return
		}
	}

	rl := chatpkg.LimitSpec(ch, chatpkg.RateLimits{
		SayWindowTicks:       uint64(limits.SayWindowTicks),
		SayMax:               limits.SayMax,
		MarketSayWindowTicks: uint64(limits.MarketSayWindowTicks),
		MarketSayMax:         limits.MarketSayMax,
	})
	if ok, cd := a.RateLimitAllow(rl.Kind, nowTick, rl.Window, rl.Max); !ok {
		ev := ar(nowTick, inst.ID, false, "E_RATE_LIMIT", rl.RateErrMsg)
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}

	if env != nil {
		env.BroadcastChat(nowTick, a, ch, inst.Text)
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleWhisper(env WorldEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, limits WhisperLimits) {
	if ok, cd := a.RateLimitAllow("WHISPER", nowTick, uint64(limits.WindowTicks), limits.Max); !ok {
		ev := ar(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many WHISPER")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	if ok, code, msg := ValidateWhisperInput(inst.To, inst.Text); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "missing world env"))
		return
	}
	to := env.AgentByID(inst.To)
	if to == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	to.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "CHAT",
		"from":    a.ID,
		"channel": "WHISPER",
		"text":    inst.Text,
	})
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleEat(ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, itemDefs map[string]catalogs.ItemDef) {
	if inst.ItemID == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing item_id"))
		return
	}
	n := eatpkg.NormalizeConsumeCount(inst.Count)
	def, ok := itemDefs[inst.ItemID]
	if !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown item"))
		return
	}
	if !eatpkg.IsFood(def.Kind, def.EdibleHP) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "item not edible"))
		return
	}
	if a.Inventory[inst.ItemID] < n {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing food"))
		return
	}
	for i := 0; i < n; i++ {
		a.Inventory[inst.ItemID]--
		if a.Inventory[inst.ItemID] <= 0 {
			delete(a.Inventory, inst.ItemID)
		}
	}
	next := eatpkg.ApplyFood(eatpkg.State{
		HP:           a.HP,
		Hunger:       a.Hunger,
		StaminaMilli: a.StaminaMilli,
	}, def.EdibleHP, n)
	a.HP = next.HP
	a.Hunger = next.Hunger
	a.StaminaMilli = next.StaminaMilli
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleSaveMemory(ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := ValidateSaveMemoryInput(inst.Key); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	mem := map[string]string{}
	for k, v := range a.Memory {
		mem[k] = v.Value
	}
	// Enforce a very small budget (64KB total).
	if memorypkg.OverMemoryBudget(mem, inst.Key, inst.Value, 64*1024) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_RESOURCE", "memory budget exceeded"))
		return
	}
	a.MemorySave(inst.Key, inst.Value, inst.TTLTicks, nowTick)
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleLoadMemory(ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	kvs := a.MemoryLoad(inst.Prefix, inst.Limit, nowTick)
	a.PendingMemory = kvs
	a.AddEvent(ar(nowTick, inst.ID, true, "", fmt.Sprintf("loaded %d keys", len(kvs))))
}
