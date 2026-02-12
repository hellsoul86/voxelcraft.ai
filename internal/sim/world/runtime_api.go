package world

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	adminhandlerspkg "voxelcraft.ai/internal/sim/world/feature/admin/handlers"
	adminrequestspkg "voxelcraft.ai/internal/sim/world/feature/admin/requests"
	transfereventspkg "voxelcraft.ai/internal/sim/world/feature/transfer/events"
	transferorgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
	transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"
)

func (w *World) SetTickLogger(l TickLogger)                    { w.tickLogger = l }
func (w *World) SetAuditLogger(l AuditLogger)                  { w.auditLogger = l }
func (w *World) SetSnapshotSink(ch chan<- snapshot.SnapshotV1) { w.snapshotSink = ch }

func (w *World) ExportSnapshot(nowTick uint64) snapshot.SnapshotV1 {
	return w.exportSnapshot(nowTick)
}

// ImportSnapshot replaces the current in-memory world state with the snapshot.
// It sets the world's tick to snapshotTick+1 (the next tick to simulate).
//
// This must be called only when the world is stopped or from the world loop goroutine.
func (w *World) ImportSnapshot(s snapshot.SnapshotV1) error {
	return w.importSnapshotV1(s)
}

func (w *World) Inbox() chan<- ActionEnvelope { return w.inbox }
func (w *World) Join() chan<- JoinRequest     { return w.join }
func (w *World) Attach() chan<- AttachRequest { return w.attach }
func (w *World) Leave() chan<- string         { return w.leave }

func (w *World) ObserverJoin() chan<- ObserverJoinRequest           { return w.observerJoin }
func (w *World) ObserverSubscribe() chan<- ObserverSubscribeRequest { return w.observerSub }
func (w *World) ObserverLeave() chan<- string                       { return w.observerLeave }

func (w *World) CurrentTick() uint64 { return w.tick.Load() }

func (w *World) systemMovement(nowTick uint64) { w.systemMovementImpl(nowTick) }
func (w *World) systemWork(nowTick uint64)     { w.systemWorkImpl(nowTick) }

// ---- Debug/Test Helpers ----
//
// These helpers exist to allow black-box tests in sibling packages (e.g. internal/sim/worldtest)
// to set up deterministic preconditions without reaching into world internals.
//
// They are NOT safe to call concurrently with Run(). Prefer using them only in tests that drive
// the world via StepOnce(), from a single goroutine.

func (w *World) DebugSetAgentPos(agentID string, pos Vec3i) bool {
	if w == nil || agentID == "" {
		return false
	}
	a := w.agents[agentID]
	if a == nil {
		return false
	}
	pos.Y = 0
	a.Pos = pos
	return true
}

func (w *World) DebugClearAgentEvents(agentID string) bool {
	if w == nil || agentID == "" {
		return false
	}
	a := w.agents[agentID]
	if a == nil {
		return false
	}
	a.Events = nil
	return true
}

func (w *World) DebugSetAgentVitals(agentID string, hp, hunger, staminaMilli int) bool {
	if w == nil || agentID == "" {
		return false
	}
	a := w.agents[agentID]
	if a == nil {
		return false
	}
	if hp >= 0 {
		a.HP = hp
	}
	if hunger >= 0 {
		a.Hunger = hunger
	}
	if staminaMilli >= 0 {
		a.StaminaMilli = staminaMilli
	}
	return true
}

func (w *World) DebugAddInventory(agentID string, item string, delta int) bool {
	if w == nil || agentID == "" {
		return false
	}
	a := w.agents[agentID]
	if a == nil {
		return false
	}
	it := strings.TrimSpace(item)
	if it == "" {
		return false
	}
	if w.catalogs == nil {
		return false
	}
	if _, ok := w.catalogs.Items.Defs[it]; !ok {
		return false
	}
	if a.Inventory == nil {
		a.Inventory = map[string]int{}
	}
	if delta == 0 {
		return true
	}
	next := a.Inventory[it] + delta
	if next <= 0 {
		delete(a.Inventory, it)
		return true
	}
	a.Inventory[it] = next
	return true
}

// DebugSetBlock sets a single world tile directly (2D: y must be 0).
// It does not write audit entries and does not update derived runtime meta (containers/signs/etc).
func (w *World) DebugSetBlock(pos Vec3i, blockName string) error {
	if w == nil {
		return errors.New("nil world")
	}
	if pos.Y != 0 {
		return fmt.Errorf("2D world requires y==0: %+v", pos)
	}
	bid, ok := w.catalogs.Blocks.Index[blockName]
	if !ok {
		return fmt.Errorf("unknown block: %q", blockName)
	}
	if !w.chunks.inBounds(pos) {
		return fmt.Errorf("out of bounds: %+v", pos)
	}
	w.chunks.SetBlock(pos, bid)
	return nil
}

func (w *World) DebugGetBlock(pos Vec3i) (uint16, error) {
	if w == nil {
		return 0, errors.New("nil world")
	}
	if pos.Y != 0 {
		return 0, fmt.Errorf("2D world requires y==0: %+v", pos)
	}
	if !w.chunks.inBounds(pos) {
		return 0, fmt.Errorf("out of bounds: %+v", pos)
	}
	return w.chunks.GetBlock(pos), nil
}

// CheckBlueprintPlaced is a stable helper for tests/automation.
// It reports whether the blueprint blocks match the world at the given anchor/rotation.
func (w *World) CheckBlueprintPlaced(blueprintID string, anchor [3]int, rotation int) bool {
	if w == nil {
		return false
	}
	return w.checkBlueprintPlaced(blueprintID, Vec3i{X: anchor[0], Y: anchor[1], Z: anchor[2]}, rotation)
}

type EventCursorItem = transfereventspkg.CursorItem

type injectEventReq struct {
	AgentID string
	Event   protocol.Event
}

func (w *World) RequestEventsAfter(ctx context.Context, agentID string, sinceCursor uint64, limit int) ([]EventCursorItem, uint64, error) {
	if w == nil || w.eventsReq == nil {
		return nil, sinceCursor, errors.New("event query not available")
	}
	req := transfereventspkg.Req{
		AgentID:     agentID,
		SinceCursor: sinceCursor,
		Limit:       limit,
		Resp:        make(chan transfereventspkg.Resp, 1),
	}
	select {
	case w.eventsReq <- req:
	case <-ctx.Done():
		return nil, sinceCursor, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return nil, sinceCursor, errors.New(resp.Err)
		}
		return resp.Items, resp.NextCursor, nil
	case <-ctx.Done():
		return nil, sinceCursor, ctx.Err()
	}
}

func (w *World) RequestInjectEvent(ctx context.Context, agentID string, ev protocol.Event) error {
	if w == nil || w.injectEvent == nil {
		return errors.New("inject event not available")
	}
	req := injectEventReq{AgentID: agentID, Event: ev}
	select {
	case w.injectEvent <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *World) RequestTransferOut(ctx context.Context, agentID string) (AgentTransfer, error) {
	if w == nil || w.transferOut == nil {
		return AgentTransfer{}, errors.New("transfer out not available")
	}
	req := transferOutReq{
		AgentID: agentID,
		Resp:    make(chan transferOutResp, 1),
	}
	select {
	case w.transferOut <- req:
	case <-ctx.Done():
		return AgentTransfer{}, ctx.Err()
	}
	select {
	case r := <-req.Resp:
		if r.Err != "" {
			return AgentTransfer{}, errors.New(r.Err)
		}
		return r.Transfer, nil
	case <-ctx.Done():
		return AgentTransfer{}, ctx.Err()
	}
}

func (w *World) RequestTransferIn(ctx context.Context, t AgentTransfer, out chan []byte, delta bool) error {
	if w == nil || w.transferIn == nil {
		return errors.New("transfer in not available")
	}
	req := transferInReq{
		Transfer:    t,
		Out:         out,
		DeltaVoxels: delta,
		Resp:        make(chan transferInResp, 1),
	}
	select {
	case w.transferIn <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case r := <-req.Resp:
		if r.Err != "" {
			return errors.New(r.Err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *World) handleEventsReq(req transfereventspkg.Req) {
	resp := transferruntimepkg.HandleEventsReq(req, func(agentID string, sinceCursor uint64, limit int) ([]transfereventspkg.CursorItem, uint64, bool) {
		a := w.agents[agentID]
		if a == nil {
			return nil, sinceCursor, false
		}
		items, next := a.EventsAfter(sinceCursor, limit)
		out := make([]transfereventspkg.CursorItem, 0, len(items))
		for _, it := range items {
			out = append(out, transfereventspkg.CursorItem{Cursor: it.Cursor, Event: it.Event})
		}
		return out, next, true
	})
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}

type adminSnapshotReq = adminrequestspkg.SnapshotReq
type adminSnapshotResp = adminrequestspkg.SnapshotResp

type adminResetReq = adminrequestspkg.ResetReq
type adminResetResp = adminrequestspkg.ResetResp

// RequestSnapshot asks the world loop goroutine to enqueue a snapshot.
// It is safe to call from other goroutines (e.g. HTTP handlers).
func (w *World) RequestSnapshot(ctx context.Context) (tick uint64, err error) {
	if w == nil || w.admin == nil {
		return 0, errors.New("admin snapshot not available")
	}
	resp := make(chan adminSnapshotResp, 1)
	req := adminSnapshotReq{Resp: resp}

	select {
	case w.admin <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	select {
	case r := <-resp:
		if r.Err != "" {
			return r.Tick, errors.New(r.Err)
		}
		return r.Tick, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (w *World) handleAdminSnapshotRequests(reqs []adminSnapshotReq) {
	if w == nil || len(reqs) == 0 {
		return
	}
	result := adminhandlerspkg.HandleSnapshot(adminhandlerspkg.SnapshotInput{
		CurrentTick: w.tick.Load(),
		HasSink:     w.snapshotSink != nil,
		Enqueue: func(snapshotTick uint64) bool {
			snap := w.ExportSnapshot(snapshotTick)
			select {
			case w.snapshotSink <- snap:
				return true
			default:
				return false
			}
		},
	})
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- adminSnapshotResp{Tick: result.Tick, Err: result.Err}:
		default:
			// Client timed out; don't block the sim loop.
		}
	}
}

// RequestReset asks the world loop goroutine to perform an immediate world reset.
// It is safe to call from other goroutines (e.g. admin HTTP handlers).
func (w *World) RequestReset(ctx context.Context) (tick uint64, err error) {
	if w == nil || w.adminReset == nil {
		return 0, errors.New("admin reset not available")
	}
	resp := make(chan adminResetResp, 1)
	req := adminResetReq{Resp: resp}

	select {
	case w.adminReset <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	select {
	case r := <-resp:
		if r.Err != "" {
			return r.Tick, errors.New(r.Err)
		}
		return r.Tick, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (w *World) handleAdminResetRequests(reqs []adminResetReq) {
	if w == nil || len(reqs) == 0 {
		return
	}
	result := adminhandlerspkg.HandleReset(adminhandlerspkg.ResetInput{
		CurrentTick: w.tick.Load(),
		HasSink:     w.snapshotSink != nil,
		Enqueue: func(archiveTick uint64) bool {
			snap := w.ExportSnapshot(archiveTick)
			select {
			case w.snapshotSink <- snap:
				return true
			default:
				return false
			}
		},
		OnReset: func(curTick uint64, archiveTick uint64) {
			newSeason := w.seasonIndex(curTick) + 1
			w.resetWorldForNewSeason(curTick, newSeason, archiveTick)
			w.auditEvent(curTick, "SYSTEM", "WORLD_RESET", Vec3i{}, "ADMIN_RESET", map[string]any{
				"world_id":     w.cfg.ID,
				"archive_tick": archiveTick,
				"new_seed":     w.cfg.Seed,
			})
		},
	})
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- adminResetResp{Tick: result.Tick, Err: result.Err}:
		default:
		}
	}
}

// RequestAgentPos returns the current position for an agent from the world loop goroutine.
func (w *World) RequestAgentPos(ctx context.Context, agentID string) (Vec3i, error) {
	if w == nil || w.agentPosReq == nil {
		return Vec3i{}, errors.New("agent position query not available")
	}
	req := transferruntimepkg.AgentPosReq{
		AgentID: agentID,
		Resp:    make(chan transferruntimepkg.AgentPosResp, 1),
	}
	select {
	case w.agentPosReq <- req:
	case <-ctx.Done():
		return Vec3i{}, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return Vec3i{}, errors.New(resp.Err)
		}
		return Vec3i{X: resp.Pos[0], Y: resp.Pos[1], Z: resp.Pos[2]}, nil
	case <-ctx.Done():
		return Vec3i{}, ctx.Err()
	}
}

func (w *World) handleAgentPosReq(req transferruntimepkg.AgentPosReq) {
	resp := transferruntimepkg.HandleAgentPosReq(req, func(agentID string) ([3]int, bool) {
		a := w.agents[agentID]
		if a == nil {
			return [3]int{}, false
		}
		return a.Pos.ToArray(), true
	})
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}

// RequestOrgMetaSnapshot returns a world-local snapshot of organization metadata.
// Treasury is intentionally excluded; this API is used for cross-world identity sync.
func (w *World) RequestOrgMetaSnapshot(ctx context.Context) ([]OrgTransfer, error) {
	if w == nil || w.orgMetaReq == nil {
		return nil, errors.New("org metadata query not available")
	}
	req := transferruntimepkg.OrgMetaReq{Resp: make(chan transferruntimepkg.OrgMetaResp, 1)}
	select {
	case w.orgMetaReq <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return nil, errors.New(resp.Err)
		}
		out := make([]OrgTransfer, 0, len(resp.Orgs))
		for _, org := range resp.Orgs {
			members := make(map[string]OrgRole, len(org.Members))
			for aid, role := range org.Members {
				members[aid] = OrgRole(role)
			}
			out = append(out, OrgTransfer{
				OrgID:       org.OrgID,
				Kind:        OrgKind(org.Kind),
				Name:        org.Name,
				CreatedTick: org.CreatedTick,
				MetaVersion: org.MetaVersion,
				Members:     members,
			})
		}
		return out, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *World) handleOrgMetaReq(req transferruntimepkg.OrgMetaReq) {
	resp := transferruntimepkg.OrgMetaResp{}
	if w == nil {
		resp.Err = "world unavailable"
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
		return
	}
	states := make([]transferorgpkg.State, 0, len(w.orgs))
	for id, org := range w.orgs {
		if org == nil || id == "" {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			members[aid] = string(role)
		}
		states = append(states, transferorgpkg.State{
			OrgID:       org.OrgID,
			Kind:        string(org.Kind),
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     members,
		})
	}
	resp = transferruntimepkg.BuildOrgMetaResp(states)
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}

// RequestUpsertOrgMeta applies manager-authoritative org metadata into this world.
// It updates identity/membership only; treasury remains world-local.
func (w *World) RequestUpsertOrgMeta(ctx context.Context, orgs []OrgTransfer) error {
	if w == nil || w.orgMetaUpsert == nil {
		return errors.New("org metadata upsert not available")
	}
	incoming := make([]transferorgpkg.State, 0, len(orgs))
	for _, org := range orgs {
		if org.OrgID == "" {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			members[aid] = string(role)
		}
		incoming = append(incoming, transferorgpkg.State{
			OrgID:       org.OrgID,
			Kind:        string(org.Kind),
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     members,
		})
	}
	req := transferruntimepkg.OrgMetaUpsertReq{
		Orgs: incoming,
		Resp: make(chan transferruntimepkg.OrgMetaUpsertResp, 1),
	}
	select {
	case w.orgMetaUpsert <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return errors.New(resp.Err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *World) handleOrgMetaUpsertReq(req transferruntimepkg.OrgMetaUpsertReq) {
	resp := transferruntimepkg.OrgMetaUpsertResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()
	if w == nil {
		resp.Err = "world unavailable"
		return
	}

	existingStates := make([]transferorgpkg.State, 0, len(w.orgs))
	for orgID, org := range w.orgs {
		if org == nil {
			continue
		}
		curMembers := map[string]string{}
		for aid, role := range org.Members {
			curMembers[aid] = string(role)
		}
		existingStates = append(existingStates, transferorgpkg.State{
			OrgID:       orgID,
			Kind:        string(org.Kind),
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     curMembers,
		})
	}
	mergedStates, ownerByAgent := transferruntimepkg.BuildOrgMetaMerge(existingStates, req.Orgs)
	for _, src := range mergedStates {
		dst := w.orgByID(src.OrgID)
		if dst == nil {
			dst = &Organization{
				OrgID:           src.OrgID,
				Treasury:        map[string]int{},
				TreasuryByWorld: map[string]map[string]int{},
			}
			w.orgs[src.OrgID] = dst
		}
		dst.Kind = OrgKind(src.Kind)
		dst.Name = src.Name
		dst.CreatedTick = src.CreatedTick
		dst.MetaVersion = src.MetaVersion
		nextMembers := make(map[string]OrgRole, len(src.Members))
		for aid, role := range src.Members {
			nextMembers[aid] = OrgRole(role)
		}
		dst.Members = nextMembers
		_ = w.orgTreasury(dst)
	}

	for _, a := range w.agents {
		if a == nil {
			continue
		}
		if orgID, ok := ownerByAgent[a.ID]; ok {
			a.OrgID = orgID
			continue
		}
		if a.OrgID == "" {
			continue
		}
		org := w.orgByID(a.OrgID)
		if org == nil || org.Members == nil || org.Members[a.ID] == "" {
			a.OrgID = ""
		}
	}
}
