package multiworld

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world"
)

type Session struct {
	AgentID      string
	CurrentWorld string
	DeltaVoxels  bool
	Out          chan []byte
}

type Runtime struct {
	Spec  WorldSpec
	World *world.World
}

const (
	stateVersion            = 2
	worldRequestTimeout     = 3 * time.Second
	worldLeaveSendTimeout   = 300 * time.Millisecond
	worldInjectEventTimeout = 2 * time.Second
)

type persistedResidency struct {
	AgentToWorld  map[string]string `json:"agent_to_world"`
	ResumeToWorld map[string]string `json:"resume_to_world"`
}

type persistedState struct {
	Version       int                         `json:"version"`
	AgentToWorld  map[string]string           `json:"agent_to_world"`
	ResumeToWorld map[string]string           `json:"resume_to_world"`
	OrgMeta       map[string]persistedOrgMeta `json:"org_meta,omitempty"`
	SwitchMetrics []persistedSwitchMetric     `json:"switch_metrics,omitempty"`
	// Backward-compat for older state dumps.
	SwitchTotals []persistedSwitchMetric `json:"switch_totals,omitempty"`
}

type persistedOrgMeta struct {
	OrgID       string            `json:"org_id"`
	Kind        string            `json:"kind,omitempty"`
	Name        string            `json:"name,omitempty"`
	CreatedTick uint64            `json:"created_tick,omitempty"`
	MetaVersion uint64            `json:"meta_version,omitempty"`
	Members     map[string]string `json:"members,omitempty"`
}

type persistedSwitchMetric struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Result string `json:"result"`
	Count  uint64 `json:"count"`
}

type OrgMeta struct {
	OrgID       string
	Kind        world.OrgKind
	Name        string
	CreatedTick uint64
	MetaVersion uint64
	Members     map[string]world.OrgRole
}

type Manager struct {
	mu sync.RWMutex

	runtimes  map[string]*Runtime
	worldsCfg map[string]WorldSpec
	entries   map[string]map[string]EntryPointSpec
	routes    map[string][]SwitchRouteSpec
	manifest  []protocol.WorldRef
	defaultID string
	stateFile string

	agentToWorld  map[string]string
	resumeToWorld map[string]string
	switchTotals  map[switchMetricKey]uint64
	globalOrgMeta map[string]OrgMeta

	persistDebounce    time.Duration
	persistCh          chan struct{}
	persistFlush       chan chan struct{}
	persistStop        chan struct{}
	persistWG          sync.WaitGroup
	orgRefreshDebounce time.Duration
	orgRefreshCh       chan struct{}
	orgRefreshStop     chan struct{}
	orgRefreshWG       sync.WaitGroup
	closeOnce          sync.Once
}

type switchMetricKey struct {
	From   string
	To     string
	Result string
}

type SwitchMetric struct {
	From   string
	To     string
	Result string
	Count  uint64
}

func NewManager(cfg Config, runtimes map[string]*Runtime, stateFile string) (*Manager, error) {
	if len(runtimes) == 0 {
		return nil, fmt.Errorf("empty runtimes")
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	for _, spec := range cfg.Worlds {
		rt := runtimes[spec.ID]
		if rt == nil || rt.World == nil {
			return nil, fmt.Errorf("missing runtime for world %s", spec.ID)
		}
	}
	worldsCfg := map[string]WorldSpec{}
	entries := map[string]map[string]EntryPointSpec{}
	for _, spec := range cfg.Worlds {
		worldsCfg[spec.ID] = spec
		byID := map[string]EntryPointSpec{}
		for _, ep := range spec.EntryPoints {
			byID[ep.ID] = ep
		}
		entries[spec.ID] = byID
	}
	routes := map[string][]SwitchRouteSpec{}
	for _, r := range cfg.SwitchRoutes {
		k := routeKey(r.FromWorld, r.ToWorld)
		routes[k] = append(routes[k], r)
	}
	for k := range routes {
		sort.Slice(routes[k], func(i, j int) bool {
			if routes[k][i].FromEntryID != routes[k][j].FromEntryID {
				return routes[k][i].FromEntryID < routes[k][j].FromEntryID
			}
			return routes[k][i].ToEntryID < routes[k][j].ToEntryID
		})
	}

	m := &Manager{
		runtimes:           runtimes,
		worldsCfg:          worldsCfg,
		entries:            entries,
		routes:             routes,
		manifest:           cfg.Manifest(),
		defaultID:          cfg.DefaultWorldID,
		stateFile:          stateFile,
		agentToWorld:       map[string]string{},
		resumeToWorld:      map[string]string{},
		switchTotals:       map[switchMetricKey]uint64{},
		globalOrgMeta:      map[string]OrgMeta{},
		persistDebounce:    200 * time.Millisecond,
		persistCh:          make(chan struct{}, 1),
		persistFlush:       make(chan chan struct{}, 8),
		persistStop:        make(chan struct{}),
		orgRefreshDebounce: 150 * time.Millisecond,
		orgRefreshCh:       make(chan struct{}, 1),
		orgRefreshStop:     make(chan struct{}),
	}
	m.loadState()
	m.persistWG.Add(1)
	go m.persistLoop()
	m.orgRefreshWG.Add(1)
	go m.orgRefreshLoop()
	return m, nil
}

func routeKey(from, to string) string {
	return from + "->" + to
}

func (m *Manager) WorldIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.runtimes))
	for id := range m.runtimes {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) Runtime(id string) *Runtime {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runtimes[id]
}

func (m *Manager) Manifest() []protocol.WorldRef {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]protocol.WorldRef, len(m.manifest))
	copy(out, m.manifest)
	return out
}

// RefreshOrgMeta collects organization metadata from all worlds and merges it into
// the manager-level global org view. This is a read-only operation for worlds.
func (m *Manager) RefreshOrgMeta(ctx context.Context) error {
	worldIDs := m.WorldIDs()
	merged := map[string]OrgMeta{}
	var firstErr error
	for _, worldID := range worldIDs {
		rt := m.Runtime(worldID)
		if rt == nil || rt.World == nil {
			continue
		}
		orgs, err := rt.World.RequestOrgMetaSnapshot(ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("world %s: %w", worldID, err)
			}
			continue
		}
		for _, org := range orgs {
			if stringsTrim(org.OrgID) == "" {
				continue
			}
			candidate := OrgMeta{
				OrgID:       org.OrgID,
				Kind:        org.Kind,
				Name:        org.Name,
				CreatedTick: org.CreatedTick,
				MetaVersion: org.MetaVersion,
				Members:     map[string]world.OrgRole{},
			}
			for aid, role := range org.Members {
				if stringsTrim(aid) == "" || role == "" {
					continue
				}
				candidate.Members[aid] = role
			}
			meta := merged[org.OrgID]
			switch {
			case stringsTrim(meta.OrgID) == "":
				merged[org.OrgID] = candidate
			case candidate.MetaVersion > meta.MetaVersion:
				merged[org.OrgID] = candidate
			case candidate.MetaVersion == meta.MetaVersion:
				merged[org.OrgID] = mergeOrgMetaSameVersion(meta, candidate)
			}
		}
	}

	m.mu.Lock()
	changed := false
	for orgID, meta := range merged {
		if !orgMetaEqual(m.globalOrgMeta[orgID], meta) {
			m.globalOrgMeta[orgID] = meta
			changed = true
		}
	}
	for orgID := range m.globalOrgMeta {
		if _, ok := merged[orgID]; !ok {
			delete(m.globalOrgMeta, orgID)
			changed = true
		}
	}
	if changed {
		m.schedulePersistLocked()
	}
	m.mu.Unlock()

	meta := m.snapshotOrgTransfers()
	for _, worldID := range worldIDs {
		rt := m.Runtime(worldID)
		if rt == nil || rt.World == nil {
			continue
		}
		if err := rt.World.RequestUpsertOrgMeta(ctx, meta); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("world %s upsert: %w", worldID, err)
		}
	}

	return firstErr
}

func (m *Manager) SwitchMetrics() []SwitchMetric {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SwitchMetric, 0, len(m.switchTotals))
	for k, n := range m.switchTotals {
		out = append(out, SwitchMetric{
			From:   k.From,
			To:     k.To,
			Result: k.Result,
			Count:  n,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].To != out[j].To {
			return out[i].To < out[j].To
		}
		return out[i].Result < out[j].Result
	})
	return out
}

func (m *Manager) Join(name string, delta bool, out chan []byte, worldPreference string) (Session, world.JoinResponse, error) {
	target := m.pickWorld(worldPreference)
	rt := m.runtime(target)
	if rt == nil {
		return Session{}, world.JoinResponse{}, fmt.Errorf("default world not found: %s", target)
	}

	respCh := make(chan world.JoinResponse, 1)
	req := world.JoinRequest{
		Name:        name,
		DeltaVoxels: delta,
		Out:         out,
		Resp:        respCh,
	}
	ctx, cancel := m.requestCtx(context.Background())
	defer cancel()
	resp, err := m.sendJoinRequest(ctx, rt, req)
	if err != nil {
		return Session{}, world.JoinResponse{}, fmt.Errorf("join request failed: %w", err)
	}
	if resp.Welcome.AgentID == "" {
		return Session{}, world.JoinResponse{}, fmt.Errorf("join failed")
	}
	resp.Welcome.CurrentWorldID = target
	resp.Welcome.WorldManifest = m.Manifest()

	s := Session{
		AgentID:      resp.Welcome.AgentID,
		CurrentWorld: target,
		DeltaVoxels:  delta,
		Out:          out,
	}
	m.updateResidency(resp.Welcome.AgentID, target, resp.Welcome.ResumeToken)
	return s, resp, nil
}

func (m *Manager) Attach(resumeToken string, delta bool, out chan []byte) (Session, world.JoinResponse, error) {
	worldID := m.worldByResumeToken(resumeToken)
	try := []string{}
	if worldID != "" {
		try = append(try, worldID)
	}
	ids := m.WorldIDs()
	for _, id := range ids {
		if id == worldID {
			continue
		}
		try = append(try, id)
	}
	for _, id := range try {
		rt := m.runtime(id)
		if rt == nil {
			continue
		}
		respCh := make(chan world.JoinResponse, 1)
		req := world.AttachRequest{
			ResumeToken: resumeToken,
			DeltaVoxels: delta,
			Out:         out,
			Resp:        respCh,
		}
		ctx, cancel := m.requestCtx(context.Background())
		resp, err := m.sendAttachRequest(ctx, rt, req)
		cancel()
		if err != nil {
			continue
		}
		if resp.Welcome.AgentID == "" {
			continue
		}
		resp.Welcome.CurrentWorldID = id
		resp.Welcome.WorldManifest = m.Manifest()
		s := Session{
			AgentID:      resp.Welcome.AgentID,
			CurrentWorld: id,
			DeltaVoxels:  delta,
			Out:          out,
		}
		m.updateResidency(resp.Welcome.AgentID, id, resp.Welcome.ResumeToken)
		return s, resp, nil
	}
	return Session{}, world.JoinResponse{}, errors.New("resume token not found")
}

func (m *Manager) Leave(s Session) {
	rt := m.runtime(s.CurrentWorld)
	if rt != nil {
		timer := time.NewTimer(worldLeaveSendTimeout)
		defer timer.Stop()
		select {
		case rt.World.Leave() <- s.AgentID:
		case <-timer.C:
		}
	}
}

func (m *Manager) RouteAct(ctx context.Context, s *Session, act protocol.ActMsg) (string, error) {
	if s == nil {
		return "", errors.New("nil session")
	}
	if act.ExpectedWorldID != "" && act.ExpectedWorldID != s.CurrentWorld {
		return s.CurrentWorld, m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, "ACT", false, protocol.ErrWorldBusy, "expected_world_id mismatch"))
	}

	switchRef := ""
	switchTarget := ""
	switchEntry := ""
	filteredInstants := make([]protocol.InstantReq, 0, len(act.Instants))
	for _, inst := range act.Instants {
		if inst.Type == "SWITCH_WORLD" {
			switchRef = inst.ID
			switchTarget = inst.TargetWorldID
			switchEntry = inst.EntryPointID
			continue
		}
		filteredInstants = append(filteredInstants, inst)
	}
	if switchTarget != "" {
		if err := m.switchWorld(ctx, s, switchTarget, switchEntry, switchRef); err != nil {
			return s.CurrentWorld, err
		}
		return s.CurrentWorld, nil
	}

	act.Instants = filteredInstants
	rt := m.runtime(s.CurrentWorld)
	if rt == nil {
		return s.CurrentWorld, fmt.Errorf("world not found: %s", s.CurrentWorld)
	}
	if err := m.sendActionEnvelope(ctx, rt, world.ActionEnvelope{AgentID: s.AgentID, Act: act}); err != nil {
		return s.CurrentWorld, m.injectActionResult(context.Background(), s.CurrentWorld, s.AgentID, actionResult(0, "ACT", false, protocol.ErrWorldBusy, "world inbox busy"))
	}
	if hasOrgMutation(filteredInstants) {
		m.scheduleOrgRefresh()
	}
	return s.CurrentWorld, nil
}

func (m *Manager) switchWorld(ctx context.Context, s *Session, target, entryPointID, ref string) error {
	if stringsTrim(target) == "" {
		m.recordSwitch(s.CurrentWorld, target, "invalid_target")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldNotFound, "missing target world"))
	}
	srcID := s.CurrentWorld
	if srcID == target {
		m.recordSwitch(srcID, target, "noop")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, true, "", "already in target world"))
	}
	src := m.runtime(srcID)
	dst := m.runtime(target)
	if src == nil || dst == nil {
		m.recordSwitch(srcID, target, "world_not_found")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldNotFound, "target world not found"))
	}

	timeoutCtx, cancel := m.requestCtx(ctx)
	defer cancel()

	pos, err := src.World.RequestAgentPos(timeoutCtx, s.AgentID)
	if err != nil {
		m.recordSwitch(srcID, target, "source_busy")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldBusy, err.Error()))
	}
	route, srcEntry, dstEntry, ok := m.selectRoute(srcID, target, entryPointID, pos)
	if !ok {
		m.recordSwitch(srcID, target, "denied")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldDenied, "entry point required"))
	}
	if route.RequiresPermit || dst.Spec.RequiresPermit {
		m.recordSwitch(srcID, target, "denied")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldDenied, "permit required"))
	}
	if !withinEntry(pos, srcEntry) {
		m.recordSwitch(srcID, target, "denied")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldDenied, "entry point required"))
	}

	transfer, err := src.World.RequestTransferOut(timeoutCtx, s.AgentID)
	if err != nil {
		m.recordSwitch(srcID, target, "source_busy")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldBusy, err.Error()))
	}

	nowDst := dst.World.CurrentTick()
	if transfer.WorldSwitchCooldownUntilTick != 0 && nowDst < transfer.WorldSwitchCooldownUntilTick {
		// Roll back to source world on cooldown rejection.
		_ = src.World.RequestTransferIn(timeoutCtx, transfer, s.Out, s.DeltaVoxels)
		m.recordSwitch(srcID, target, "cooldown")
		return m.injectActionResult(ctx, s.CurrentWorld, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldCooldown, "switch cooldown active"))
	}

	m.mergeOrgMetaFromTransfer(transfer.Org)
	m.attachOrgMetaToTransfer(&transfer)
	// 1.0 entry placement: attach directly to configured destination entry point.
	transfer.Pos = world.Vec3i{X: dstEntry.X, Y: 0, Z: dstEntry.Z}
	transfer.FromWorldID = srcID
	transfer.CurrentWorldID = target
	transfer.FromEntryPointID = srcEntry.ID
	transfer.ToEntryPointID = dstEntry.ID
	transfer.WorldSwitchCooldownUntilTick = nowDst + uint64(max(0, dst.Spec.SwitchCooldownTicks))

	if err := dst.World.RequestTransferIn(timeoutCtx, transfer, s.Out, s.DeltaVoxels); err != nil {
		// Attempt rollback to source to avoid orphaning the agent.
		_ = src.World.RequestTransferIn(timeoutCtx, transfer, s.Out, s.DeltaVoxels)
		m.recordSwitch(srcID, target, "target_busy")
		return m.injectActionResult(ctx, srcID, s.AgentID, actionResult(0, ref, false, protocol.ErrWorldBusy, "switch failed: "+err.Error()))
	}

	s.CurrentWorld = target
	m.updateResidency(s.AgentID, target, "")
	m.recordSwitch(srcID, target, "ok")
	_ = m.injectActionResult(ctx, target, s.AgentID, protocol.Event{
		"type":          "ACTION_RESULT",
		"ref":           ref,
		"ok":            true,
		"world_id":      target,
		"from":          srcID,
		"from_entry_id": srcEntry.ID,
		"to_entry_id":   dstEntry.ID,
	})
	return nil
}

func (m *Manager) selectRoute(fromWorld, toWorld, requestedFromEntry string, pos world.Vec3i) (SwitchRouteSpec, EntryPointSpec, EntryPointSpec, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	routes := m.routes[routeKey(fromWorld, toWorld)]
	if len(routes) == 0 {
		return SwitchRouteSpec{}, EntryPointSpec{}, EntryPointSpec{}, false
	}
	if requestedFromEntry != "" {
		for _, r := range routes {
			if r.FromEntryID != requestedFromEntry {
				continue
			}
			fromEP, okFrom := m.entries[fromWorld][r.FromEntryID]
			toEP, okTo := m.entries[toWorld][r.ToEntryID]
			if okFrom && okTo && fromEP.Enabled && toEP.Enabled {
				return r, fromEP, toEP, true
			}
		}
		return SwitchRouteSpec{}, EntryPointSpec{}, EntryPointSpec{}, false
	}
	// No explicit entry selected by the caller: choose a route whose source entry
	// currently contains the agent position, so multi-entry worlds do not depend on
	// lexical route ordering.
	for _, r := range routes {
		fromEP, okFrom := m.entries[fromWorld][r.FromEntryID]
		toEP, okTo := m.entries[toWorld][r.ToEntryID]
		if okFrom && okTo && fromEP.Enabled && toEP.Enabled && withinEntry(pos, fromEP) {
			return r, fromEP, toEP, true
		}
	}
	// Backward-compatible fallback: first enabled route.
	for _, r := range routes {
		fromEP, okFrom := m.entries[fromWorld][r.FromEntryID]
		toEP, okTo := m.entries[toWorld][r.ToEntryID]
		if okFrom && okTo && fromEP.Enabled && toEP.Enabled {
			return r, fromEP, toEP, true
		}
	}
	return SwitchRouteSpec{}, EntryPointSpec{}, EntryPointSpec{}, false
}

func withinEntry(pos world.Vec3i, ep EntryPointSpec) bool {
	dx := pos.X - ep.X
	if dx < 0 {
		dx = -dx
	}
	dz := pos.Z - ep.Z
	if dz < 0 {
		dz = -dz
	}
	return dx <= ep.Radius && dz <= ep.Radius
}

func (m *Manager) injectActionResult(ctx context.Context, worldID, agentID string, ev protocol.Event) error {
	rt := m.runtime(worldID)
	if rt == nil {
		return errors.New("world not found")
	}
	now := rt.World.CurrentTick()
	if _, ok := ev["t"]; !ok {
		ev["t"] = now
	}
	reqCtx, cancel := m.injectCtx(ctx)
	defer cancel()
	return rt.World.RequestInjectEvent(reqCtx, agentID, ev)
}

func (m *Manager) requestCtx(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, worldRequestTimeout)
}

func (m *Manager) injectCtx(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, worldInjectEventTimeout)
}

func (m *Manager) sendJoinRequest(ctx context.Context, rt *Runtime, req world.JoinRequest) (world.JoinResponse, error) {
	select {
	case rt.World.Join() <- req:
	case <-ctx.Done():
		return world.JoinResponse{}, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		return resp, nil
	case <-ctx.Done():
		return world.JoinResponse{}, ctx.Err()
	}
}

func (m *Manager) sendAttachRequest(ctx context.Context, rt *Runtime, req world.AttachRequest) (world.JoinResponse, error) {
	select {
	case rt.World.Attach() <- req:
	case <-ctx.Done():
		return world.JoinResponse{}, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		return resp, nil
	case <-ctx.Done():
		return world.JoinResponse{}, ctx.Err()
	}
}

func (m *Manager) sendActionEnvelope(ctx context.Context, rt *Runtime, env world.ActionEnvelope) error {
	reqCtx, cancel := m.requestCtx(ctx)
	defer cancel()
	select {
	case rt.World.Inbox() <- env:
		return nil
	case <-reqCtx.Done():
		return reqCtx.Err()
	}
}

func (m *Manager) runtime(id string) *Runtime {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runtimes[id]
}

func (m *Manager) pickWorld(pref string) string {
	p := stringsTrim(pref)
	if p == "" {
		return m.defaultID
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.runtimes[p]; ok {
		return p
	}
	return m.defaultID
}

func (m *Manager) worldByResumeToken(token string) string {
	token = stringsTrim(token)
	if token == "" {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.resumeToWorld[token]
}

func (m *Manager) AgentWorld(agentID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agentToWorld[agentID]
}

func (m *Manager) MoveAgentWorld(ctx context.Context, agentID, targetWorldID string) error {
	if stringsTrim(agentID) == "" || stringsTrim(targetWorldID) == "" {
		return fmt.Errorf("missing agent_id/target_world_id")
	}
	srcID := m.AgentWorld(agentID)
	if srcID == "" {
		return fmt.Errorf("agent not found in residency map")
	}
	if srcID == targetWorldID {
		m.recordSwitch(srcID, targetWorldID, "admin_noop")
		return nil
	}
	src := m.runtime(srcID)
	dst := m.runtime(targetWorldID)
	if src == nil || dst == nil {
		m.recordSwitch(srcID, targetWorldID, "admin_world_not_found")
		return fmt.Errorf("source or target world not found")
	}
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	transfer, err := src.World.RequestTransferOut(tctx, agentID)
	if err != nil {
		m.recordSwitch(srcID, targetWorldID, "admin_source_busy")
		return err
	}
	m.mergeOrgMetaFromTransfer(transfer.Org)
	m.attachOrgMetaToTransfer(&transfer)
	transfer.FromWorldID = srcID
	transfer.CurrentWorldID = targetWorldID
	transfer.WorldSwitchCooldownUntilTick = dst.World.CurrentTick() + uint64(max(0, dst.Spec.SwitchCooldownTicks))
	if err := dst.World.RequestTransferIn(tctx, transfer, nil, false); err != nil {
		_ = src.World.RequestTransferIn(tctx, transfer, nil, false)
		m.recordSwitch(srcID, targetWorldID, "admin_target_busy")
		return err
	}
	m.updateResidency(agentID, targetWorldID, "")
	m.recordSwitch(srcID, targetWorldID, "admin_ok")
	return nil
}

func (m *Manager) recordSwitch(from, to, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if from == "" {
		from = "UNKNOWN"
	}
	if to == "" {
		to = "UNKNOWN"
	}
	if result == "" {
		result = "unknown"
	}
	k := switchMetricKey{From: from, To: to, Result: result}
	m.switchTotals[k]++
	m.schedulePersistLocked()
}

func (m *Manager) updateResidency(agentID, worldID, resumeToken string) {
	if agentID == "" || worldID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentToWorld[agentID] = worldID
	if stringsTrim(resumeToken) != "" {
		m.resumeToWorld[resumeToken] = worldID
	}
	m.schedulePersistLocked()
}

func (m *Manager) mergeOrgMetaFromTransfer(org *world.OrgTransfer) {
	if org == nil || stringsTrim(org.OrgID) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	meta := m.globalOrgMeta[org.OrgID]
	candidate := OrgMeta{
		OrgID:       org.OrgID,
		Kind:        org.Kind,
		Name:        org.Name,
		CreatedTick: org.CreatedTick,
		MetaVersion: org.MetaVersion,
		Members:     map[string]world.OrgRole{},
	}
	for aid, role := range org.Members {
		if stringsTrim(aid) == "" || role == "" {
			continue
		}
		candidate.Members[aid] = role
	}
	switch {
	case stringsTrim(meta.OrgID) == "":
		m.globalOrgMeta[org.OrgID] = candidate
	case candidate.MetaVersion > meta.MetaVersion:
		m.globalOrgMeta[org.OrgID] = candidate
	case candidate.MetaVersion == meta.MetaVersion:
		// Transfer path can observe concurrent world updates at the same revision;
		// keep a converged superset for memberships.
		if candidate.Kind != "" {
			meta.Kind = candidate.Kind
		}
		if candidate.Name != "" {
			meta.Name = candidate.Name
		}
		if meta.CreatedTick == 0 || (candidate.CreatedTick != 0 && candidate.CreatedTick < meta.CreatedTick) {
			meta.CreatedTick = candidate.CreatedTick
		}
		if meta.Members == nil {
			meta.Members = map[string]world.OrgRole{}
		}
		for aid, role := range candidate.Members {
			meta.Members[aid] = role
		}
		m.globalOrgMeta[org.OrgID] = meta
	}
	m.schedulePersistLocked()
}

func (m *Manager) attachOrgMetaToTransfer(t *world.AgentTransfer) {
	if t == nil || stringsTrim(t.OrgID) == "" {
		return
	}
	m.mu.RLock()
	meta, ok := m.globalOrgMeta[t.OrgID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	members := map[string]world.OrgRole{}
	for aid, role := range meta.Members {
		if stringsTrim(aid) == "" || role == "" {
			continue
		}
		members[aid] = role
	}
	t.Org = &world.OrgTransfer{
		OrgID:       meta.OrgID,
		Kind:        meta.Kind,
		Name:        meta.Name,
		CreatedTick: meta.CreatedTick,
		MetaVersion: meta.MetaVersion,
		Members:     members,
	}
}

func (m *Manager) snapshotOrgTransfers() []world.OrgTransfer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	orgIDs := make([]string, 0, len(m.globalOrgMeta))
	for orgID := range m.globalOrgMeta {
		orgIDs = append(orgIDs, orgID)
	}
	sort.Strings(orgIDs)
	out := make([]world.OrgTransfer, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		meta := m.globalOrgMeta[orgID]
		members := map[string]world.OrgRole{}
		memberIDs := make([]string, 0, len(meta.Members))
		for aid := range meta.Members {
			memberIDs = append(memberIDs, aid)
		}
		sort.Strings(memberIDs)
		for _, aid := range memberIDs {
			role := meta.Members[aid]
			if stringsTrim(aid) == "" || role == "" {
				continue
			}
			members[aid] = role
		}
		out = append(out, world.OrgTransfer{
			OrgID:       meta.OrgID,
			Kind:        meta.Kind,
			Name:        meta.Name,
			CreatedTick: meta.CreatedTick,
			MetaVersion: meta.MetaVersion,
			Members:     members,
		})
	}
	return out
}

func (m *Manager) loadState() {
	if m.stateFile == "" {
		return
	}
	if m.tryLoadStateFile(m.stateFile) {
		return
	}
	legacy := filepath.Join(filepath.Dir(m.stateFile), "agent_residency.json")
	_ = m.tryLoadStateFile(legacy)
}

func (m *Manager) tryLoadStateFile(path string) bool {
	if stringsTrim(path) == "" {
		return false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var st persistedState
	if err := json.Unmarshal(b, &st); err == nil {
		loaded := false
		for k, v := range st.AgentToWorld {
			if stringsTrim(k) == "" || stringsTrim(v) == "" {
				continue
			}
			m.agentToWorld[k] = v
			loaded = true
		}
		for k, v := range st.ResumeToWorld {
			if stringsTrim(k) == "" || stringsTrim(v) == "" {
				continue
			}
			m.resumeToWorld[k] = v
			loaded = true
		}
		for orgID, om := range st.OrgMeta {
			if stringsTrim(orgID) == "" {
				continue
			}
			meta := OrgMeta{
				OrgID:       orgID,
				Kind:        world.OrgKind(om.Kind),
				Name:        om.Name,
				CreatedTick: om.CreatedTick,
				MetaVersion: om.MetaVersion,
				Members:     map[string]world.OrgRole{},
			}
			for aid, role := range om.Members {
				if stringsTrim(aid) == "" || stringsTrim(role) == "" {
					continue
				}
				meta.Members[aid] = world.OrgRole(role)
			}
			m.globalOrgMeta[orgID] = meta
			loaded = true
		}
		allSwitchMetrics := append([]persistedSwitchMetric{}, st.SwitchMetrics...)
		allSwitchMetrics = append(allSwitchMetrics, st.SwitchTotals...)
		for _, sm := range allSwitchMetrics {
			if stringsTrim(sm.From) == "" || stringsTrim(sm.To) == "" || stringsTrim(sm.Result) == "" || sm.Count == 0 {
				continue
			}
			m.switchTotals[switchMetricKey{From: sm.From, To: sm.To, Result: sm.Result}] = sm.Count
			loaded = true
		}
		if loaded {
			return true
		}
	}

	// Backward compatibility: old residency-only format.
	var legacy persistedResidency
	if err := json.Unmarshal(b, &legacy); err != nil {
		return false
	}
	loaded := false
	if legacy.AgentToWorld != nil {
		for k, v := range legacy.AgentToWorld {
			if k != "" && v != "" {
				m.agentToWorld[k] = v
				loaded = true
			}
		}
	}
	if legacy.ResumeToWorld != nil {
		for k, v := range legacy.ResumeToWorld {
			if k != "" && v != "" {
				m.resumeToWorld[k] = v
				loaded = true
			}
		}
	}
	return loaded
}

func (m *Manager) schedulePersistLocked() {
	if m.stateFile == "" || m.persistCh == nil {
		return
	}
	select {
	case m.persistCh <- struct{}{}:
	default:
	}
}

func (m *Manager) persistLoop() {
	defer m.persistWG.Done()
	var timer *time.Timer
	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
	}
	for {
		var timerCh <-chan time.Time
		if timer != nil {
			timerCh = timer.C
		}
		select {
		case <-m.persistStop:
			stopTimer()
			m.persistNow()
			return
		case <-m.persistCh:
			if timer == nil {
				timer = time.NewTimer(m.persistDebounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(m.persistDebounce)
			}
		case ack := <-m.persistFlush:
			stopTimer()
			m.persistNow()
			if ack != nil {
				close(ack)
			}
		case <-timerCh:
			stopTimer()
			m.persistNow()
		}
	}
}

func (m *Manager) Close() {
	m.closeOnce.Do(func() {
		if m.persistStop != nil {
			close(m.persistStop)
		}
		m.persistWG.Wait()
		if m.orgRefreshStop != nil {
			close(m.orgRefreshStop)
		}
		m.orgRefreshWG.Wait()
	})
}

func (m *Manager) FlushState(ctx context.Context) error {
	if m.stateFile == "" || m.persistFlush == nil {
		return nil
	}
	ack := make(chan struct{})
	select {
	case m.persistFlush <- ack:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-ack:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) persistNow() {
	st := m.snapshotState()
	m.writeState(st)
}

func (m *Manager) snapshotState() persistedState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshotStateLocked()
}

func (m *Manager) snapshotStateLocked() persistedState {
	st := persistedState{
		Version:       stateVersion,
		AgentToWorld:  map[string]string{},
		ResumeToWorld: map[string]string{},
		OrgMeta:       map[string]persistedOrgMeta{},
		SwitchMetrics: []persistedSwitchMetric{},
	}
	for k, v := range m.agentToWorld {
		if k != "" && v != "" {
			st.AgentToWorld[k] = v
		}
	}
	for k, v := range m.resumeToWorld {
		if k != "" && v != "" {
			st.ResumeToWorld[k] = v
		}
	}
	orgIDs := make([]string, 0, len(m.globalOrgMeta))
	for orgID := range m.globalOrgMeta {
		orgIDs = append(orgIDs, orgID)
	}
	sort.Strings(orgIDs)
	for _, orgID := range orgIDs {
		meta := m.globalOrgMeta[orgID]
		pm := persistedOrgMeta{
			OrgID:       meta.OrgID,
			Kind:        string(meta.Kind),
			Name:        meta.Name,
			CreatedTick: meta.CreatedTick,
			MetaVersion: meta.MetaVersion,
			Members:     map[string]string{},
		}
		memberIDs := make([]string, 0, len(meta.Members))
		for aid := range meta.Members {
			memberIDs = append(memberIDs, aid)
		}
		sort.Strings(memberIDs)
		for _, aid := range memberIDs {
			role := meta.Members[aid]
			if stringsTrim(aid) == "" || role == "" {
				continue
			}
			pm.Members[aid] = string(role)
		}
		st.OrgMeta[orgID] = pm
	}
	for k, n := range m.switchTotals {
		if n == 0 {
			continue
		}
		st.SwitchMetrics = append(st.SwitchMetrics, persistedSwitchMetric{
			From:   k.From,
			To:     k.To,
			Result: k.Result,
			Count:  n,
		})
	}
	sort.Slice(st.SwitchMetrics, func(i, j int) bool {
		if st.SwitchMetrics[i].From != st.SwitchMetrics[j].From {
			return st.SwitchMetrics[i].From < st.SwitchMetrics[j].From
		}
		if st.SwitchMetrics[i].To != st.SwitchMetrics[j].To {
			return st.SwitchMetrics[i].To < st.SwitchMetrics[j].To
		}
		return st.SwitchMetrics[i].Result < st.SwitchMetrics[j].Result
	})
	return st
}

func (m *Manager) writeState(st persistedState) {
	if m.stateFile == "" {
		return
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	_ = os.MkdirAll(filepath.Dir(m.stateFile), 0o755)
	tmp := m.stateFile + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, m.stateFile)
}

func actionResult(t uint64, ref string, ok bool, code, message string) protocol.Event {
	if !protocol.IsKnownCode(code) {
		code = protocol.ErrInternal
		if message == "" {
			message = "unknown error code"
		}
	}
	ev := protocol.Event{
		"t":    t,
		"type": "ACTION_RESULT",
		"ref":  ref,
		"ok":   ok,
	}
	if code != "" {
		ev["code"] = code
	}
	if message != "" {
		ev["message"] = message
	}
	return ev
}

func simpleHash(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func stringsTrim(s string) string {
	out := s
	for len(out) > 0 && (out[0] == ' ' || out[0] == '\t' || out[0] == '\n' || out[0] == '\r') {
		out = out[1:]
	}
	for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t' || out[len(out)-1] == '\n' || out[len(out)-1] == '\r') {
		out = out[:len(out)-1]
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func orgMetaEqual(a, b OrgMeta) bool {
	if a.OrgID != b.OrgID || a.Kind != b.Kind || a.Name != b.Name || a.CreatedTick != b.CreatedTick {
		return false
	}
	if len(a.Members) != len(b.Members) {
		return false
	}
	for aid, role := range a.Members {
		if b.Members[aid] != role {
			return false
		}
	}
	return true
}

func chooseNewerOrgMeta(current, candidate OrgMeta) bool {
	if stringsTrim(candidate.OrgID) == "" {
		return false
	}
	if stringsTrim(current.OrgID) == "" {
		return true
	}
	if candidate.MetaVersion != current.MetaVersion {
		return candidate.MetaVersion > current.MetaVersion
	}
	// Deterministic tie-break for concurrent same-version writes.
	return orgMetaDigest(candidate) > orgMetaDigest(current)
}

func mergeOrgMetaSameVersion(a, b OrgMeta) OrgMeta {
	if stringsTrim(a.OrgID) == "" {
		return b
	}
	if stringsTrim(b.OrgID) == "" {
		return a
	}
	out := a
	if out.Members == nil {
		out.Members = map[string]world.OrgRole{}
	}
	if out.Kind == "" && b.Kind != "" {
		out.Kind = b.Kind
	}
	if stringsTrim(out.Name) == "" && stringsTrim(b.Name) != "" {
		out.Name = b.Name
	}
	if out.CreatedTick == 0 || (b.CreatedTick != 0 && b.CreatedTick < out.CreatedTick) {
		out.CreatedTick = b.CreatedTick
	}
	for aid, role := range b.Members {
		if stringsTrim(aid) == "" || role == "" {
			continue
		}
		out.Members[aid] = role
	}
	return out
}

func orgMetaDigest(m OrgMeta) string {
	memberIDs := make([]string, 0, len(m.Members))
	for aid := range m.Members {
		memberIDs = append(memberIDs, aid)
	}
	sort.Strings(memberIDs)
	out := m.OrgID + "|" + string(m.Kind) + "|" + m.Name
	for _, aid := range memberIDs {
		role := m.Members[aid]
		out += "|" + aid + ":" + string(role)
	}
	return out
}

func hasOrgMutation(instants []protocol.InstantReq) bool {
	for _, inst := range instants {
		switch inst.Type {
		case "CREATE_ORG", "JOIN_ORG", "LEAVE_ORG":
			return true
		}
	}
	return false
}

func (m *Manager) scheduleOrgRefresh() {
	if m.orgRefreshCh == nil {
		return
	}
	select {
	case m.orgRefreshCh <- struct{}{}:
	default:
	}
}

func (m *Manager) orgRefreshLoop() {
	defer m.orgRefreshWG.Done()
	var timer *time.Timer
	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
	}
	for {
		var timerCh <-chan time.Time
		if timer != nil {
			timerCh = timer.C
		}
		select {
		case <-m.orgRefreshStop:
			stopTimer()
			return
		case <-m.orgRefreshCh:
			if timer == nil {
				timer = time.NewTimer(m.orgRefreshDebounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(m.orgRefreshDebounce)
			}
		case <-timerCh:
			stopTimer()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = m.RefreshOrgMeta(ctx)
			cancel()
		}
	}
}
