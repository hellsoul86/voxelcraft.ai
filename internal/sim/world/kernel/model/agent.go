package model

import (
	"math"
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	"voxelcraft.ai/internal/sim/world/logic/rates"
)

type Agent struct {
	ID    string
	Name  string
	OrgID string // at most one org membership (MVP)

	CurrentWorldID               string
	WorldSwitchCooldownUntilTick uint64

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
	// Monotonic count of events delivered to this agent via OBS.
	EventCursor uint64
	// Retained event history for reliable cursor-based fetch.
	EventLog []eventLogEntry

	// Rate limiting windows (per action type).
	rl map[string]*rateWindow

	// Fun-score anti-exploit state.
	funDecay    map[string]*funDecayWindow
	seenBiomes  map[string]bool
	seenRecipes map[string]bool
	seenEvents  map[string]bool

	// Memory KV (private to this agent).
	Memory map[string]MemoryEntry
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

// RateWindowSnapshot is a minimal persisted representation of a rate window.
// It intentionally does not include window/max (those are configured via tuning).
type RateWindowSnapshot struct {
	StartTick uint64
	Count     int
}

type funDecayWindow struct {
	StartTick uint64
	Count     int
}

// FunDecaySnapshot is a minimal persisted representation of a fun decay window.
type FunDecaySnapshot struct {
	StartTick uint64
	Count     int
}

type MemoryEntry struct {
	Value      string
	ExpiryTick uint64
}

type eventLogEntry struct {
	Cursor uint64
	Event  protocol.Event
}

func (a *Agent) InitDefaults() {
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
		a.Memory = map[string]MemoryEntry{}
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
	a.EventCursor++
	a.EventLog = append(a.EventLog, eventLogEntry{Cursor: a.EventCursor, Event: e})
	if len(a.EventLog) > 4096 {
		a.EventLog = append([]eventLogEntry(nil), a.EventLog[len(a.EventLog)-4096:]...)
	}
}

func (a *Agent) TakeEvents() []protocol.Event {
	ev := a.Events
	a.Events = nil
	return ev
}

func (a *Agent) EventsAfter(cursor uint64, limit int) ([]eventLogEntry, uint64) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	out := make([]eventLogEntry, 0, limit)
	next := cursor
	for _, e := range a.EventLog {
		if e.Cursor <= cursor {
			continue
		}
		out = append(out, e)
		next = e.Cursor
		if len(out) >= limit {
			break
		}
	}
	return out, next
}

func (a *Agent) RateLimitAllow(kind string, nowTick uint64, window uint64, max int) (ok bool, cooldownTicks uint64) {
	if a.rl == nil {
		a.rl = map[string]*rateWindow{}
	}
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
	start, count, allow, cool := rates.Allow(nowTick, w.StartTick, w.Count, w.Window, w.Max)
	w.StartTick = start
	w.Count = count
	return allow, cool
}

func (a *Agent) RateWindowsSnapshot() map[string]RateWindowSnapshot {
	if len(a.rl) == 0 {
		return nil
	}
	out := map[string]RateWindowSnapshot{}
	for k, rw := range a.rl {
		if k == "" || rw == nil {
			continue
		}
		if rw.Count <= 0 {
			continue
		}
		out[k] = RateWindowSnapshot{StartTick: rw.StartTick, Count: rw.Count}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *Agent) LoadRateWindowsSnapshot(src map[string]RateWindowSnapshot) {
	if len(src) == 0 {
		a.rl = nil
		return
	}
	a.rl = map[string]*rateWindow{}
	for k, rw := range src {
		if k == "" || rw.Count <= 0 {
			continue
		}
		a.rl[k] = &rateWindow{StartTick: rw.StartTick, Count: rw.Count}
	}
	if len(a.rl) == 0 {
		a.rl = nil
	}
}

func (a *Agent) ResetRateLimits() {
	a.rl = map[string]*rateWindow{}
}

func (a *Agent) FunDecayDelta(key string, base int, nowTick uint64, windowTicks uint64, baseMult float64) int {
	if key == "" || base <= 0 {
		return 0
	}
	if a.funDecay == nil {
		a.funDecay = map[string]*funDecayWindow{}
	}
	dw := a.funDecay[key]
	if dw == nil {
		dw = &funDecayWindow{StartTick: nowTick}
		a.funDecay[key] = dw
	}
	window := windowTicks
	if window == 0 {
		window = 3000
	}
	if nowTick-dw.StartTick >= window {
		dw.StartTick = nowTick
		dw.Count = 0
	}
	dw.Count++
	if baseMult <= 0 || baseMult > 1.0 {
		baseMult = 0.70
	}
	mult := math.Pow(baseMult, float64(dw.Count-1))
	delta := int(math.Round(float64(base) * mult))
	if delta <= 0 {
		return 0
	}
	return delta
}

func (a *Agent) FunDecaySnapshot() map[string]FunDecaySnapshot {
	if len(a.funDecay) == 0 {
		return nil
	}
	out := map[string]FunDecaySnapshot{}
	for k, d := range a.funDecay {
		if k == "" || d == nil {
			continue
		}
		out[k] = FunDecaySnapshot{StartTick: d.StartTick, Count: d.Count}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *Agent) LoadFunDecaySnapshot(src map[string]FunDecaySnapshot) {
	if len(src) == 0 {
		a.funDecay = nil
		return
	}
	a.funDecay = map[string]*funDecayWindow{}
	for k, v := range src {
		if k == "" || v.Count <= 0 {
			continue
		}
		a.funDecay[k] = &funDecayWindow{StartTick: v.StartTick, Count: v.Count}
	}
	if len(a.funDecay) == 0 {
		a.funDecay = nil
	}
}

func (a *Agent) ResetFunTracking() {
	a.funDecay = map[string]*funDecayWindow{}
	a.seenBiomes = map[string]bool{}
	a.seenRecipes = map[string]bool{}
	a.seenEvents = map[string]bool{}
}

func (a *Agent) MarkBiomeSeen(b string) bool {
	if b == "" {
		return false
	}
	if a.seenBiomes == nil {
		a.seenBiomes = map[string]bool{}
	}
	if a.seenBiomes[b] {
		return false
	}
	a.seenBiomes[b] = true
	return true
}

func (a *Agent) SeenBiomesSorted() []string {
	if len(a.seenBiomes) == 0 {
		return nil
	}
	out := make([]string, 0, len(a.seenBiomes))
	for b, ok := range a.seenBiomes {
		if ok && b != "" {
			out = append(out, b)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *Agent) SetSeenBiomes(list []string) {
	if len(list) == 0 {
		a.seenBiomes = map[string]bool{}
		return
	}
	a.seenBiomes = map[string]bool{}
	for _, b := range list {
		if b == "" {
			continue
		}
		a.seenBiomes[b] = true
	}
}

func (a *Agent) MarkRecipeSeen(id string) bool {
	if id == "" {
		return false
	}
	if a.seenRecipes == nil {
		a.seenRecipes = map[string]bool{}
	}
	if a.seenRecipes[id] {
		return false
	}
	a.seenRecipes[id] = true
	return true
}

func (a *Agent) SeenRecipesSorted() []string {
	if len(a.seenRecipes) == 0 {
		return nil
	}
	out := make([]string, 0, len(a.seenRecipes))
	for r, ok := range a.seenRecipes {
		if ok && r != "" {
			out = append(out, r)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *Agent) SetSeenRecipes(list []string) {
	if len(list) == 0 {
		a.seenRecipes = map[string]bool{}
		return
	}
	a.seenRecipes = map[string]bool{}
	for _, r := range list {
		if r == "" {
			continue
		}
		a.seenRecipes[r] = true
	}
}

func (a *Agent) MarkEventSeen(id string) bool {
	if id == "" {
		return false
	}
	if a.seenEvents == nil {
		a.seenEvents = map[string]bool{}
	}
	if a.seenEvents[id] {
		return false
	}
	a.seenEvents[id] = true
	return true
}

func (a *Agent) SeenEventsSorted() []string {
	if len(a.seenEvents) == 0 {
		return nil
	}
	out := make([]string, 0, len(a.seenEvents))
	for e, ok := range a.seenEvents {
		if ok && e != "" {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *Agent) SetSeenEvents(list []string) {
	if len(list) == 0 {
		a.seenEvents = map[string]bool{}
		return
	}
	a.seenEvents = map[string]bool{}
	for _, e := range list {
		if e == "" {
			continue
		}
		a.seenEvents[e] = true
	}
}

func (a *Agent) MemorySave(key, value string, ttlTicks int, nowTick uint64) {
	// ttlTicks is already in ticks; store absolute expiry tick.
	exp := uint64(0)
	if ttlTicks > 0 {
		exp = nowTick + uint64(ttlTicks)
	}
	if a.Memory == nil {
		a.Memory = map[string]MemoryEntry{}
	}
	a.Memory[key] = MemoryEntry{
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
	out := make([]protocol.MemoryKV, 0, minInt(limit, len(keys)))
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

