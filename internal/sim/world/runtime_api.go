package world

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/feature/admin"
	"voxelcraft.ai/internal/sim/world/feature/transfer"
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

type EventCursorItem = transfer.EventCursorItem

type injectEventReq struct {
	AgentID string
	Event   protocol.Event
}

func (w *World) RequestEventsAfter(ctx context.Context, agentID string, sinceCursor uint64, limit int) ([]EventCursorItem, uint64, error) {
	if w == nil || w.eventsReq == nil {
		return nil, sinceCursor, errors.New("event query not available")
	}
	req := transfer.EventsReq{
		AgentID:     agentID,
		SinceCursor: sinceCursor,
		Limit:       limit,
		Resp:        make(chan transfer.EventsResp, 1),
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

func (w *World) handleEventsReq(req transfer.EventsReq) {
	resp := transfer.EventsResp{NextCursor: req.SinceCursor}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()
	a := w.agents[req.AgentID]
	if a == nil {
		resp.Err = "agent not found"
		return
	}
	items, next := a.EventsAfter(req.SinceCursor, req.Limit)
	resp.Items = make([]transfer.EventCursorItem, 0, len(items))
	for _, it := range items {
		resp.Items = append(resp.Items, transfer.EventCursorItem{Cursor: it.Cursor, Event: it.Event})
	}
	resp.NextCursor = next
}

type adminSnapshotReq struct {
	Resp chan adminSnapshotResp
}

type adminSnapshotResp struct {
	Tick uint64
	Err  string
}

type adminResetReq struct {
	Resp chan adminResetResp
}

type adminResetResp struct {
	Tick uint64
	Err  string
}

type agentPosReq struct {
	AgentID string
	Resp    chan agentPosResp
}

type agentPosResp struct {
	Pos Vec3i
	Err string
}

type orgMetaReq struct {
	Resp chan orgMetaResp
}

type orgMetaResp struct {
	Orgs []OrgTransfer
	Err  string
}

type orgMetaUpsertReq struct {
	Orgs []OrgTransfer
	Resp chan orgMetaUpsertResp
}

type orgMetaUpsertResp struct {
	Err string
}

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
	cur := w.tick.Load()
	snapTick := admin.SnapshotTick(cur)

	errStr := ""
	if w.snapshotSink == nil {
		errStr = "snapshot sink not configured"
	} else {
		snap := w.ExportSnapshot(snapTick)
		select {
		case w.snapshotSink <- snap:
		default:
			errStr = "snapshot sink backpressure"
		}
	}

	resp := adminSnapshotResp{Tick: snapTick, Err: errStr}
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- resp:
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

	cur := w.tick.Load()
	archiveTick := admin.ArchiveTick(cur)

	errStr := ""
	if w.snapshotSink != nil {
		snap := w.ExportSnapshot(archiveTick)
		select {
		case w.snapshotSink <- snap:
		default:
			errStr = "snapshot sink backpressure"
		}
	}
	if errStr == "" {
		newSeason := w.seasonIndex(cur) + 1
		w.resetWorldForNewSeason(cur, newSeason, archiveTick)
		w.auditEvent(cur, "SYSTEM", "WORLD_RESET", Vec3i{}, "ADMIN_RESET", map[string]any{
			"world_id":     w.cfg.ID,
			"archive_tick": archiveTick,
			"new_seed":     w.cfg.Seed,
		})
	}

	resp := adminResetResp{Tick: cur, Err: errStr}
	for _, r := range reqs {
		if r.Resp == nil {
			continue
		}
		select {
		case r.Resp <- resp:
		default:
		}
	}
}

// RequestAgentPos returns the current position for an agent from the world loop goroutine.
func (w *World) RequestAgentPos(ctx context.Context, agentID string) (Vec3i, error) {
	if w == nil || w.agentPosReq == nil {
		return Vec3i{}, errors.New("agent position query not available")
	}
	req := agentPosReq{
		AgentID: agentID,
		Resp:    make(chan agentPosResp, 1),
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
		return resp.Pos, nil
	case <-ctx.Done():
		return Vec3i{}, ctx.Err()
	}
}

func (w *World) handleAgentPosReq(req agentPosReq) {
	resp := agentPosResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()
	a := w.agents[req.AgentID]
	if a == nil {
		resp.Err = "agent not found"
		return
	}
	resp.Pos = a.Pos
}

// RequestOrgMetaSnapshot returns a world-local snapshot of organization metadata.
// Treasury is intentionally excluded; this API is used for cross-world identity sync.
func (w *World) RequestOrgMetaSnapshot(ctx context.Context) ([]OrgTransfer, error) {
	if w == nil || w.orgMetaReq == nil {
		return nil, errors.New("org metadata query not available")
	}
	req := orgMetaReq{Resp: make(chan orgMetaResp, 1)}
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
		return resp.Orgs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *World) handleOrgMetaReq(req orgMetaReq) {
	resp := orgMetaResp{}
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
	meta := make(map[string]transfer.OrgMeta, len(w.orgs))
	for id, org := range w.orgs {
		if org == nil || id == "" {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			members[aid] = string(role)
		}
		meta[id] = transfer.OrgMeta{
			OrgID:       org.OrgID,
			Kind:        string(org.Kind),
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     members,
		}
	}
	sorted := transfer.SortedOrgMeta(meta)
	out := make([]OrgTransfer, 0, len(sorted))
	for _, org := range sorted {
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
	resp.Orgs = out
}

// RequestUpsertOrgMeta applies manager-authoritative org metadata into this world.
// It updates identity/membership only; treasury remains world-local.
func (w *World) RequestUpsertOrgMeta(ctx context.Context, orgs []OrgTransfer) error {
	if w == nil || w.orgMetaUpsert == nil {
		return errors.New("org metadata upsert not available")
	}
	req := orgMetaUpsertReq{
		Orgs: orgs,
		Resp: make(chan orgMetaUpsertResp, 1),
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

func (w *World) handleOrgMetaUpsertReq(req orgMetaUpsertReq) {
	resp := orgMetaUpsertResp{}
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

	incoming := map[string]transfer.OrgMeta{}
	for _, org := range req.Orgs {
		if org.OrgID == "" {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			members[aid] = string(role)
		}
		incoming[org.OrgID] = transfer.OrgMeta{
			OrgID:       org.OrgID,
			Kind:        string(org.Kind),
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     members,
		}
	}

	for _, src := range transfer.SortedOrgMeta(incoming) {
		dst := w.orgByID(src.OrgID)
		if dst == nil {
			dst = &Organization{
				OrgID:           src.OrgID,
				Kind:            OrgKind(src.Kind),
				Name:            src.Name,
				CreatedTick:     src.CreatedTick,
				MetaVersion:     src.MetaVersion,
				Members:         map[string]OrgRole{},
				Treasury:        map[string]int{},
				TreasuryByWorld: map[string]map[string]int{},
			}
			w.orgs[src.OrgID] = dst
		}
		curMembers := map[string]string{}
		for aid, role := range dst.Members {
			curMembers[aid] = string(role)
		}
		current := transfer.OrgMeta{
			OrgID:       dst.OrgID,
			Kind:        string(dst.Kind),
			Name:        dst.Name,
			CreatedTick: dst.CreatedTick,
			MetaVersion: dst.MetaVersion,
			Members:     curMembers,
		}
		merged, accepted := transfer.MergeOrgMeta(current, src)
		if !accepted {
			_ = w.orgTreasury(dst)
			continue
		}
		dst.Kind = OrgKind(merged.Kind)
		dst.Name = merged.Name
		dst.CreatedTick = merged.CreatedTick
		dst.MetaVersion = merged.MetaVersion
		members := make(map[string]OrgRole, len(merged.Members))
		for aid, role := range merged.Members {
			members[aid] = OrgRole(role)
		}
		dst.Members = members
		_ = w.orgTreasury(dst)
	}

	orgs := map[string]transfer.OrgMeta{}
	for orgID, org := range w.orgs {
		if org == nil {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			members[aid] = string(role)
		}
		orgs[orgID] = transfer.OrgMeta{
			OrgID:   orgID,
			Members: members,
		}
	}
	ownerByAgent := transfer.OwnerByAgent(orgs)
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
