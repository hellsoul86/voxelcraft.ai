package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"voxelcraft.ai/internal/protocol"
)

type SessionConfig struct {
	Key         string
	WorldWSURL  string
	ResumeToken string
	AgentIDHint string
}

type sessionUpdate struct {
	ResumeToken     string
	AgentID         string
	LastConnectedAt time.Time
	LastObsTick     uint64
}

type onUpdateFn func(key string, upd sessionUpdate)

type Session struct {
	cfg      SessionConfig
	onUpdate onUpdateFn

	mu sync.RWMutex

	startOnce sync.Once
	closeOnce sync.Once
	stop      chan struct{}
	done      chan struct{}

	connected       bool
	lastConnectedAt time.Time
	lastErr         string

	conn    *websocket.Conn
	writeMu sync.Mutex

	agentID         string
	resumeToken     string
	protocolVersion string
	welcome         protocol.WelcomeMsg

	catalogs map[string]catalogEntry

	lastObsTick      uint64
	lastObsID        string
	lastObsWorldID   string
	lastEventsCursor uint64
	lastObsRaw       json.RawMessage

	obsNotify         chan struct{}
	ackNotify         chan struct{}
	acksByActID       map[string]protocol.AckMsg
	eventBatchWaiters map[string]chan protocol.EventBatchMsg

	eventRing []EventResult

	lastUsedAt time.Time
}

type catalogEntry struct {
	Digest string
	Data   json.RawMessage
}

type catalogWire struct {
	Type            string          `json:"type"`
	ProtocolVersion string          `json:"protocol_version"`
	Name            string          `json:"name"`
	Digest          string          `json:"digest"`
	Part            int             `json:"part"`
	TotalParts      int             `json:"total_parts"`
	Data            json.RawMessage `json:"data"`
}

type obsTickOnly struct {
	Tick    uint64 `json:"tick"`
	AgentID string `json:"agent_id"`
}

func NewSession(cfg SessionConfig, onUpdate onUpdateFn) *Session {
	if cfg.Key == "" {
		cfg.Key = "default"
	}
	s := &Session{
		cfg:               cfg,
		onUpdate:          onUpdate,
		stop:              make(chan struct{}),
		done:              make(chan struct{}),
		agentID:           cfg.AgentIDHint,
		resumeToken:       cfg.ResumeToken,
		catalogs:          map[string]catalogEntry{},
		obsNotify:         make(chan struct{}, 1),
		ackNotify:         make(chan struct{}, 1),
		acksByActID:       map[string]protocol.AckMsg{},
		eventBatchWaiters: map[string]chan protocol.EventBatchMsg{},
		protocolVersion:   protocol.Version,
		lastUsedAt:        time.Now(),
	}
	return s
}

func (s *Session) Start() {
	s.startOnce.Do(func() {
		go s.run()
	})
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		// Ensure any blocking ReadMessage wakes up promptly.
		s.Disconnect()
		<-s.done
	})
}

func (s *Session) Disconnect() {
	s.mu.Lock()
	c := s.conn
	s.conn = nil
	s.connected = false
	s.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

func (s *Session) LastUsedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUsedAt
}

func (s *Session) touch() {
	s.mu.Lock()
	s.lastUsedAt = time.Now()
	s.mu.Unlock()
}

func (s *Session) Status() Status {
	s.touch()
	s.mu.RLock()
	defer s.mu.RUnlock()

	dig := map[string]string{}
	if s.welcome.Catalogs.BlockPalette.Digest != "" {
		dig["block_palette"] = s.welcome.Catalogs.BlockPalette.Digest
	}
	if s.welcome.Catalogs.ItemPalette.Digest != "" {
		dig["item_palette"] = s.welcome.Catalogs.ItemPalette.Digest
	}
	if s.welcome.Catalogs.TuningDigest != "" {
		dig["tuning"] = s.welcome.Catalogs.TuningDigest
	}
	if s.welcome.Catalogs.RecipesDigest != "" {
		dig["recipes"] = s.welcome.Catalogs.RecipesDigest
	}
	if s.welcome.Catalogs.BlueprintsDigest != "" {
		dig["blueprints"] = s.welcome.Catalogs.BlueprintsDigest
	}
	if s.welcome.Catalogs.LawTemplatesDigest != "" {
		dig["law_templates"] = s.welcome.Catalogs.LawTemplatesDigest
	}
	if s.welcome.Catalogs.EventsDigest != "" {
		dig["events"] = s.welcome.Catalogs.EventsDigest
	}

	return Status{
		Connected:        s.connected,
		AgentID:          s.agentID,
		ResumeToken:      s.resumeToken,
		WorldWSURL:       s.cfg.WorldWSURL,
		ProtocolVersion:  s.protocolVersion,
		LastObsTick:      s.lastObsTick,
		LastObsID:        s.lastObsID,
		LastEventsCursor: s.lastEventsCursor,
		CurrentWorldID:   s.welcome.CurrentWorldID,
		CatalogDigests:   dig,
		LastError:        s.lastErr,
	}
}

func (s *Session) ListWorlds() []WorldInfo {
	s.touch()
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]WorldInfo, 0, len(s.welcome.WorldManifest))
	for _, w := range s.welcome.WorldManifest {
		out = append(out, WorldInfo{
			WorldID:          w.WorldID,
			WorldType:        w.WorldType,
			EntryPointID:     w.EntryPointID,
			RequiresPermit:   w.RequiresPermit,
			SwitchCooldown:   w.SwitchCooldown,
			ResetEveryTicks:  w.ResetEveryTicks,
			ResetNoticeTicks: w.ResetNoticeTicks,
		})
	}
	return out
}

func (s *Session) GetObs(ctx context.Context, opts GetObsOpts) (ObsResult, error) {
	s.touch()
	if opts.Mode == "" {
		opts.Mode = ObsModeSummary
	}
	timeout := time.Duration(opts.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	var tick uint64
	var raw json.RawMessage
	var agentID string

	if opts.WaitNewTick {
		start := s.latestObsTick()
		var err error
		tick, raw, agentID, err = s.waitObsAfter(ctx, start, timeout)
		if err != nil {
			return ObsResult{}, err
		}
	} else {
		tick, raw, agentID = s.latestObs()
	}

	if tick == 0 || len(raw) == 0 {
		return ObsResult{Tick: 0, AgentID: s.agentID, ObsID: "", EventsCursor: s.lastEventsCursor, Obs: nil}, nil
	}

	switch opts.Mode {
	case ObsModeFull:
		var meta struct {
			ObsID        string `json:"obs_id"`
			EventsCursor uint64 `json:"events_cursor"`
		}
		_ = json.Unmarshal(raw, &meta)
		return ObsResult{Tick: tick, AgentID: agentID, ObsID: meta.ObsID, EventsCursor: meta.EventsCursor, Obs: raw}, nil
	case ObsModeNoVoxels:
		var o protocol.ObsMsg
		if err := json.Unmarshal(raw, &o); err != nil {
			return ObsResult{}, fmt.Errorf("parse obs: %w", err)
		}
		o.Voxels.Data = ""
		o.Voxels.Ops = nil
		b, _ := json.Marshal(o)
		return ObsResult{Tick: tick, AgentID: agentID, ObsID: o.ObsID, EventsCursor: o.EventsCursor, Obs: b}, nil
	case ObsModeSummary:
		var o protocol.ObsMsg
		if err := json.Unmarshal(raw, &o); err != nil {
			return ObsResult{}, fmt.Errorf("parse obs: %w", err)
		}
		ev := o.Events
		if len(ev) > 20 {
			ev = ev[len(ev)-20:]
		}
		sum := obsSummary{
			Type:            o.Type,
			ProtocolVersion: o.ProtocolVersion,
			Tick:            o.Tick,
			AgentID:         o.AgentID,
			WorldID:         o.WorldID,
			WorldClock:      o.WorldClock,
			World:           o.World,
			Self:            o.Self,
			Inventory:       o.Inventory,
			LocalRules:      o.LocalRules,
			Entities:        o.Entities,
			Events:          ev,
			Tasks:           o.Tasks,
			FunScore:        o.FunScore,
			PublicBoards:    o.PublicBoards,
		}
		b, _ := json.Marshal(sum)
		return ObsResult{Tick: tick, AgentID: agentID, ObsID: o.ObsID, EventsCursor: o.EventsCursor, Obs: b}, nil
	default:
		return ObsResult{}, fmt.Errorf("unknown mode: %s", opts.Mode)
	}
}

type obsSummary struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	Tick            uint64 `json:"tick"`
	AgentID         string `json:"agent_id"`
	WorldID         string `json:"world_id,omitempty"`
	WorldClock      uint64 `json:"world_clock,omitempty"`

	World      protocol.WorldObs      `json:"world"`
	Self       protocol.SelfObs       `json:"self"`
	Inventory  []protocol.ItemStack   `json:"inventory"`
	LocalRules protocol.LocalRulesObs `json:"local_rules"`

	Entities []protocol.EntityObs `json:"entities"`
	Events   []protocol.Event     `json:"events"`
	Tasks    []protocol.TaskObs   `json:"tasks"`

	FunScore *protocol.FunScoreObs `json:"fun_score,omitempty"`

	PublicBoards []protocol.BoardObs `json:"public_boards,omitempty"`
}

func (s *Session) GetCatalog(ctx context.Context, name string) (CatalogResult, error) {
	s.touch()
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return CatalogResult{}, fmt.Errorf("empty catalog name")
	}
	// Fast path.
	if c, ok := s.catalog(name); ok {
		return CatalogResult{Name: name, Digest: c.Digest, Data: c.Data}, nil
	}

	// Allow a short wait for handshake catalogs to arrive.
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return CatalogResult{}, ctx.Err()
		case <-deadline.C:
			if c, ok := s.catalog(name); ok {
				return CatalogResult{Name: name, Digest: c.Digest, Data: c.Data}, nil
			}
			return CatalogResult{}, fmt.Errorf("catalog not available: %s", name)
		case <-tick.C:
			if c, ok := s.catalog(name); ok {
				return CatalogResult{Name: name, Digest: c.Digest, Data: c.Data}, nil
			}
		}
	}
}

func (s *Session) Act(ctx context.Context, args ActArgs) (ActResult, error) {
	s.touch()

	instants := []protocol.InstantReq{}
	if len(args.Instants) > 0 {
		if err := json.Unmarshal(args.Instants, &instants); err != nil {
			return ActResult{}, fmt.Errorf("parse instants: %w", err)
		}
	}
	tasks := []protocol.TaskReq{}
	if len(args.Tasks) > 0 {
		if err := json.Unmarshal(args.Tasks, &tasks); err != nil {
			return ActResult{}, fmt.Errorf("parse tasks: %w", err)
		}
	}

	// Auto-generate missing IDs.
	base := time.Now().UnixMilli()
	seq := 0
	for i := range instants {
		if strings.TrimSpace(instants[i].ID) == "" {
			seq++
			instants[i].ID = fmt.Sprintf("I_%d_%d", base, seq)
		}
	}
	for i := range tasks {
		if strings.TrimSpace(tasks[i].ID) == "" {
			seq++
			tasks[i].ID = fmt.Sprintf("K_%d_%d", base, seq)
		}
	}

	// Ensure we have at least one OBS tick so we can send a non-stale ACT.
	tickUsed, agentID, err := s.waitForFirstObs(ctx, 2*time.Second)
	if err != nil {
		return ActResult{}, err
	}
	obsID, worldID := s.latestObsMeta()

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: s.latestProtocolVersion(),
		ActID:           strings.TrimSpace(args.ActID),
		BasedOnObsID:    strings.TrimSpace(args.BasedOnObsID),
		IdempotencyKey:  strings.TrimSpace(args.IdempotencyKey),
		Tick:            tickUsed,
		AgentID:         agentID,
		ExpectedWorldID: strings.TrimSpace(args.ExpectedWorldID),
		Instants:        instants,
		Tasks:           tasks,
		Cancel:          args.Cancel,
	}
	if act.ProtocolVersion == "1.1" {
		if act.ActID == "" {
			act.ActID = fmt.Sprintf("ACT_%d_%d", time.Now().UnixMilli(), tickUsed)
		}
		if act.BasedOnObsID == "" {
			act.BasedOnObsID = obsID
		}
		if act.IdempotencyKey == "" {
			act.IdempotencyKey = act.ActID
		}
		if act.ExpectedWorldID == "" {
			act.ExpectedWorldID = worldID
		}
	}
	b, _ := json.Marshal(act)

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()
	if conn == nil {
		return ActResult{}, fmt.Errorf("not connected")
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
		return ActResult{}, err
	}
	res := ActResult{Sent: true, TickUsed: tickUsed, AgentID: agentID, ActID: act.ActID}
	if act.ProtocolVersion == "1.1" && act.ActID != "" {
		waitMS := args.WaitAckMS
		if waitMS <= 0 {
			waitMS = 2000
		}
		ack, err := s.waitAck(ctx, act.ActID, time.Duration(waitMS)*time.Millisecond)
		if err != nil {
			return ActResult{}, err
		}
		if !ack.Accepted {
			return ActResult{}, fmt.Errorf("act rejected code=%s message=%s", ack.Code, ack.Message)
		}
		res.Ack = ack
	}
	return res, nil
}

func (s *Session) latestObs() (tick uint64, raw json.RawMessage, agentID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastObsTick, append(json.RawMessage(nil), s.lastObsRaw...), s.agentID
}

func (s *Session) latestObsMeta() (obsID string, worldID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastObsWorldID != "" {
		return s.lastObsID, s.lastObsWorldID
	}
	return s.lastObsID, s.welcome.CurrentWorldID
}

func (s *Session) latestProtocolVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.protocolVersion == "" {
		return protocol.Version
	}
	return s.protocolVersion
}

func (s *Session) latestObsTick() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastObsTick
}

func (s *Session) waitForFirstObs(ctx context.Context, timeout time.Duration) (tick uint64, agentID string, err error) {
	start := s.latestObsTick()
	if start != 0 {
		s.mu.RLock()
		agentID = s.agentID
		s.mu.RUnlock()
		return start, agentID, nil
	}
	t, _, aid, err := s.waitObsAfter(ctx, 0, timeout)
	return t, aid, err
}

func (s *Session) waitObsAfter(ctx context.Context, start uint64, timeout time.Duration) (tick uint64, raw json.RawMessage, agentID string, err error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		t, r, aid := s.latestObs()
		if t > start && t != 0 && len(r) > 0 {
			return t, r, aid, nil
		}
		select {
		case <-ctx.Done():
			return 0, nil, "", ctx.Err()
		case <-deadline.C:
			// One last check after timeout.
			t, r, aid = s.latestObs()
			if t > start && t != 0 && len(r) > 0 {
				return t, r, aid, nil
			}
			return 0, nil, "", fmt.Errorf("timeout waiting for obs")
		case <-s.obsNotify:
		}
	}
}

func (s *Session) waitAck(ctx context.Context, actID string, timeout time.Duration) (protocol.AckMsg, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		if ack, ok := s.lookupAck(actID); ok {
			return ack, nil
		}
		select {
		case <-ctx.Done():
			return protocol.AckMsg{}, ctx.Err()
		case <-deadline.C:
			if ack, ok := s.lookupAck(actID); ok {
				return ack, nil
			}
			return protocol.AckMsg{}, fmt.Errorf("timeout waiting for ack")
		case <-s.ackNotify:
		}
	}
}

func (s *Session) lookupAck(actID string) (protocol.AckMsg, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ack, ok := s.acksByActID[actID]
	return ack, ok
}

func (s *Session) catalog(name string) (catalogEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.catalogs[name]
	if !ok || c.Digest == "" || len(c.Data) == 0 {
		return catalogEntry{}, false
	}
	return c, true
}

func (s *Session) run() {
	defer close(s.done)

	backoff := 200 * time.Millisecond
	for {
		select {
		case <-s.stop:
			s.Disconnect()
			return
		default:
		}

		if err := s.connectAndReadLoop(); err != nil {
			s.mu.Lock()
			s.connected = false
			s.lastErr = err.Error()
			s.mu.Unlock()
			select {
			case <-s.stop:
				s.Disconnect()
				return
			case <-time.After(backoff):
			}
			if backoff < 5*time.Second {
				backoff *= 2
				if backoff > 5*time.Second {
					backoff = 5 * time.Second
				}
			}
			continue
		}
		// Clean exit.
		return
	}
}

func (s *Session) connectAndReadLoop() error {
	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := d.Dial(s.cfg.WorldWSURL, http.Header{})
	if err != nil {
		return err
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	hello := protocol.HelloMsg{
		Type:              protocol.TypeHello,
		ProtocolVersion:   "1.1",
		SupportedVersions: []string{"1.1", "1.0"},
		AgentName:         s.cfg.Key,
		Capabilities: protocol.HelloCapabilities{
			DeltaVoxels: true,
			MaxQueue:    64,
		},
		ClientCapabilities: protocol.HelloClientCapabilities{
			DeltaVoxels: true,
			AckRequired: true,
			EventCursor: true,
		},
	}
	s.mu.RLock()
	rt := strings.TrimSpace(s.resumeToken)
	s.mu.RUnlock()
	if rt != "" {
		hello.Auth = &protocol.HelloAuth{Token: rt}
	}

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(hello); err != nil {
		_ = conn.Close()
		return err
	}

	s.mu.Lock()
	s.conn = conn
	s.lastErr = ""
	s.mu.Unlock()

	for {
		select {
		case <-s.stop:
			_ = conn.Close()
			return nil
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			_ = conn.Close()
			return err
		}
		base, err := protocol.DecodeBase(msg)
		if err != nil {
			continue
		}
		switch base.Type {
		case protocol.TypeWelcome:
			var w protocol.WelcomeMsg
			if err := json.Unmarshal(msg, &w); err != nil {
				continue
			}
			if !protocol.IsSupportedVersion(w.ProtocolVersion) {
				continue
			}
			now := time.Now()
			s.mu.Lock()
			s.welcome = w
			s.agentID = w.AgentID
			s.resumeToken = w.ResumeToken
			if w.SelectedVersion != "" {
				s.protocolVersion = w.SelectedVersion
			} else if w.ProtocolVersion != "" {
				s.protocolVersion = w.ProtocolVersion
			}
			s.connected = true
			s.lastConnectedAt = now
			s.mu.Unlock()
			if s.onUpdate != nil {
				s.onUpdate(s.cfg.Key, sessionUpdate{
					ResumeToken:     w.ResumeToken,
					AgentID:         w.AgentID,
					LastConnectedAt: now,
				})
			}

		case protocol.TypeCatalog:
			var c catalogWire
			if err := json.Unmarshal(msg, &c); err != nil {
				continue
			}
			if !protocol.IsSupportedVersion(c.ProtocolVersion) {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(c.Name))
			if name == "" {
				continue
			}
			s.mu.Lock()
			s.catalogs[name] = catalogEntry{Digest: c.Digest, Data: append(json.RawMessage(nil), c.Data...)}
			s.mu.Unlock()

		case protocol.TypeAck:
			var ack protocol.AckMsg
			if err := json.Unmarshal(msg, &ack); err != nil {
				continue
			}
			if ack.AckFor == "" {
				continue
			}
			s.mu.Lock()
			s.acksByActID[ack.AckFor] = ack
			if len(s.acksByActID) > 4096 {
				trim := map[string]protocol.AckMsg{ack.AckFor: ack}
				s.acksByActID = trim
			}
			s.mu.Unlock()
			select {
			case s.ackNotify <- struct{}{}:
			default:
			}

		case protocol.TypeEventBatch:
			var batch protocol.EventBatchMsg
			if err := json.Unmarshal(msg, &batch); err != nil {
				continue
			}
			if batch.ReqID == "" {
				continue
			}
			s.mu.Lock()
			ch := s.eventBatchWaiters[batch.ReqID]
			delete(s.eventBatchWaiters, batch.ReqID)
			s.mu.Unlock()
			if ch != nil {
				select {
				case ch <- batch:
				default:
				}
			}

		case protocol.TypeObs:
			var o protocol.ObsMsg
			if err := json.Unmarshal(msg, &o); err != nil {
				continue
			}
			s.mu.Lock()
			s.lastObsTick = o.Tick
			s.lastObsID = o.ObsID
			if o.WorldID != "" {
				s.lastObsWorldID = o.WorldID
			}
			base := s.lastEventsCursor
			if o.EventsCursor > 0 && o.EventsCursor >= uint64(len(o.Events)) {
				base = o.EventsCursor - uint64(len(o.Events))
			}
			cursor := base
			for _, ev := range o.Events {
				cursor++
				b, _ := json.Marshal(ev)
				s.eventRing = append(s.eventRing, EventResult{Cursor: cursor, Event: b})
			}
			if o.EventsCursor > 0 {
				s.lastEventsCursor = o.EventsCursor
			} else {
				s.lastEventsCursor = cursor
			}
			if len(s.eventRing) > 4096 {
				s.eventRing = append([]EventResult(nil), s.eventRing[len(s.eventRing)-4096:]...)
			}
			s.lastObsRaw = append(json.RawMessage(nil), msg...)
			// agent_id may be empty if client is very early; keep latest welcome agent_id.
			if o.AgentID != "" {
				s.agentID = o.AgentID
			}
			s.mu.Unlock()
			select {
			case s.obsNotify <- struct{}{}:
			default:
			}
		}
	}
}

func (s *Session) catalogsList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.catalogs))
	for k := range s.catalogs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Session) GetEvents(ctx context.Context, sinceCursor uint64, limit int) (GetEventsResult, error) {
	s.touch()
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if s.latestProtocolVersion() == "1.1" {
		if res, ok := s.getEventsRemote(ctx, sinceCursor, limit); ok {
			return res, nil
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EventResult, 0, limit)
	for _, ev := range s.eventRing {
		if ev.Cursor <= sinceCursor {
			continue
		}
		out = append(out, ev)
		if len(out) >= limit {
			break
		}
	}
	next := sinceCursor
	if len(out) > 0 {
		next = out[len(out)-1].Cursor
	}
	return GetEventsResult{Events: out, NextCursor: next}, nil
}

func (s *Session) getEventsRemote(ctx context.Context, sinceCursor uint64, limit int) (GetEventsResult, bool) {
	reqID := fmt.Sprintf("ev_%d_%d", time.Now().UnixMilli(), sinceCursor)
	req := protocol.EventBatchReqMsg{
		Type:            protocol.TypeEventBatchReq,
		ProtocolVersion: "1.1",
		ReqID:           reqID,
		SinceCursor:     sinceCursor,
		Limit:           limit,
	}
	ch := make(chan protocol.EventBatchMsg, 1)
	s.mu.Lock()
	s.eventBatchWaiters[reqID] = ch
	conn := s.conn
	s.mu.Unlock()

	cleanup := func() {
		s.mu.Lock()
		delete(s.eventBatchWaiters, reqID)
		s.mu.Unlock()
	}
	if conn == nil {
		cleanup()
		return GetEventsResult{}, false
	}

	s.writeMu.Lock()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err := conn.WriteJSON(req)
	s.writeMu.Unlock()
	if err != nil {
		cleanup()
		return GetEventsResult{}, false
	}

	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	select {
	case <-ctx.Done():
		cleanup()
		return GetEventsResult{}, false
	case <-timeout.C:
		cleanup()
		return GetEventsResult{}, false
	case batch := <-ch:
		out := make([]EventResult, 0, len(batch.Events))
		for _, e := range batch.Events {
			b, _ := json.Marshal(e.Event)
			out = append(out, EventResult{Cursor: e.Cursor, Event: b})
		}
		return GetEventsResult{Events: out, NextCursor: batch.NextCursor}, true
	}
}
