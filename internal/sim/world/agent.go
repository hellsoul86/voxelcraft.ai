package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
)

type Agent struct {
	ID    string
	Name  string
	OrgID string // at most one org membership (MVP)

	// ResumeToken is a transport-level token used for reconnects.
	// It is intentionally NOT included in snapshots/digests.
	ResumeToken string

	Pos Vec3i
	Yaw int

	HP           int
	Hunger       int
	StaminaMilli int // 0..1000

	RepTrade  int // 0..1000
	RepBuild  int // 0..1000
	RepSocial int // 0..1000
	RepLaw    int // 0..1000

	Fun FunScore

	Inventory map[string]int
	Equipment Equipment

	MoveTask *tasks.MovementTask
	WorkTask *tasks.WorkTask

	Events []protocol.Event

	// Rate limiting windows (per action type).
	rl map[string]*rateWindow

	// Fun-score anti-exploit state.
	funDecay    map[string]*funDecayWindow
	seenBiomes  map[string]bool
	seenRecipes map[string]bool
	seenEvents  map[string]bool

	// Memory KV (private to this agent).
	Memory map[string]memoryEntry
	// If set in this tick, OBS will include it.
	PendingMemory []protocol.MemoryKV
}

type Equipment struct {
	MainHand string
	Armor    [4]string
}

type rateWindow struct {
	StartTick uint64
	Count     int
	Window    uint64
	Max       int
}

func (a *Agent) initDefaults() {
	if a.Inventory == nil {
		a.Inventory = map[string]int{}
	}
	if a.StaminaMilli == 0 {
		a.StaminaMilli = 1000
	}
	if a.HP == 0 {
		a.HP = 20
	}
	if a.Hunger == 0 {
		a.Hunger = 20
	}
	if a.RepTrade == 0 {
		a.RepTrade = 500
	}
	if a.RepBuild == 0 {
		a.RepBuild = 500
	}
	if a.RepSocial == 0 {
		a.RepSocial = 500
	}
	if a.RepLaw == 0 {
		a.RepLaw = 500
	}
	if a.Equipment.MainHand == "" {
		a.Equipment.MainHand = "NONE"
	}
	for i := 0; i < 4; i++ {
		if a.Equipment.Armor[i] == "" {
			a.Equipment.Armor[i] = "NONE"
		}
	}
	if a.rl == nil {
		a.rl = map[string]*rateWindow{}
	}
	if a.funDecay == nil {
		a.funDecay = map[string]*funDecayWindow{}
	}
	if a.seenBiomes == nil {
		a.seenBiomes = map[string]bool{}
	}
	if a.seenRecipes == nil {
		a.seenRecipes = map[string]bool{}
	}
	if a.seenEvents == nil {
		a.seenEvents = map[string]bool{}
	}
	if a.Memory == nil {
		a.Memory = map[string]memoryEntry{}
	}
}

func (a *Agent) InventoryList() []protocol.ItemStack {
	out := make([]protocol.ItemStack, 0, len(a.Inventory))
	for item, c := range a.Inventory {
		if c <= 0 {
			continue
		}
		out = append(out, protocol.ItemStack{Item: item, Count: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Item < out[j].Item })
	return out
}

func (a *Agent) AddEvent(e protocol.Event) {
	a.Events = append(a.Events, e)
}

func (a *Agent) TakeEvents() []protocol.Event {
	ev := a.Events
	a.Events = nil
	return ev
}

func (a *Agent) RateLimitAllow(kind string, nowTick uint64, window uint64, max int) (ok bool, cooldownTicks uint64) {
	w, ok := a.rl[kind]
	if !ok {
		w = &rateWindow{StartTick: nowTick, Window: window, Max: max}
		a.rl[kind] = w
	}
	w.Window = window
	w.Max = max
	// Defensive: treat invalid windows as "allow" rather than panicking/diverging.
	if w.Window == 0 || w.Max <= 0 {
		return true, 0
	}
	if nowTick-w.StartTick >= w.Window {
		w.StartTick = nowTick
		w.Count = 0
	}
	w.Count++
	if w.Count <= w.Max {
		return true, 0
	}
	// Remaining ticks until the window resets (next tick >= StartTick+Window).
	return false, (w.StartTick + w.Window) - nowTick
}

func (a *Agent) MemorySave(key, value string, ttlTicks int, nowTick uint64) {
	// ttlTicks is already in ticks; store absolute expiry tick.
	exp := uint64(0)
	if ttlTicks > 0 {
		exp = nowTick + uint64(ttlTicks)
	}
	a.Memory[key] = memoryEntry{
		Value:      value,
		ExpiryTick: exp,
	}
}

func (a *Agent) MemoryLoad(prefix string, limit int, nowTick uint64) []protocol.MemoryKV {
	if limit <= 0 || limit > 256 {
		limit = 64
	}
	keys := make([]string, 0, len(a.Memory))
	for k := range a.Memory {
		if prefix == "" || (len(k) >= len(prefix) && k[:len(prefix)] == prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]protocol.MemoryKV, 0, min(limit, len(keys)))
	for _, k := range keys {
		e := a.Memory[k]
		if e.ExpiryTick != 0 && nowTick >= e.ExpiryTick {
			delete(a.Memory, k)
			continue
		}
		out = append(out, protocol.MemoryKV{Key: k, Value: e.Value})
		if len(out) >= limit {
			break
		}
	}
	return out
}

type memoryEntry struct {
	Value      string
	ExpiryTick uint64
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
