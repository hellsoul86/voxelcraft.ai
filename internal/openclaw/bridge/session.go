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

	agentID     string
	resumeToken string
	welcome     protocol.WelcomeMsg

	catalogs map[string]catalogEntry

	lastObsTick uint64
	lastObsRaw  json.RawMessage

	obsNotify chan struct{}

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
		cfg:         cfg,
		onUpdate:    onUpdate,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		agentID:     cfg.AgentIDHint,
		resumeToken: cfg.ResumeToken,
		catalogs:    map[string]catalogEntry{},
		obsNotify:   make(chan struct{}, 1),
		lastUsedAt:  time.Now(),
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
		Connected:      s.connected,
		AgentID:        s.agentID,
		ResumeToken:    s.resumeToken,
		WorldWSURL:     s.cfg.WorldWSURL,
		LastObsTick:    s.lastObsTick,
		CurrentWorldID: s.welcome.CurrentWorldID,
		CatalogDigests: dig,
		LastError:      s.lastErr,
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
		return ObsResult{Tick: 0, AgentID: s.agentID, Obs: nil}, nil
	}

	switch opts.Mode {
	case ObsModeFull:
		return ObsResult{Tick: tick, AgentID: agentID, Obs: raw}, nil
	case ObsModeNoVoxels:
		var o protocol.ObsMsg
		if err := json.Unmarshal(raw, &o); err != nil {
			return ObsResult{}, fmt.Errorf("parse obs: %w", err)
		}
		o.Voxels.Data = ""
		o.Voxels.Ops = nil
		b, _ := json.Marshal(o)
		return ObsResult{Tick: tick, AgentID: agentID, Obs: b}, nil
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
		return ObsResult{Tick: tick, AgentID: agentID, Obs: b}, nil
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

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            tickUsed,
		AgentID:         agentID,
		Instants:        instants,
		Tasks:           tasks,
		Cancel:          args.Cancel,
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
	return ActResult{Sent: true, TickUsed: tickUsed, AgentID: agentID}, nil
}

func (s *Session) latestObs() (tick uint64, raw json.RawMessage, agentID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastObsTick, append(json.RawMessage(nil), s.lastObsRaw...), s.agentID
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
		Type:            protocol.TypeHello,
		ProtocolVersion: protocol.Version,
		AgentName:       s.cfg.Key,
		Capabilities: protocol.HelloCapabilities{
			DeltaVoxels: true,
			MaxQueue:    64,
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

		case protocol.TypeObs:
			var o obsTickOnly
			if err := json.Unmarshal(msg, &o); err != nil {
				continue
			}
			s.mu.Lock()
			s.lastObsTick = o.Tick
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
