package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Config struct {
	WorldWSURL  string
	StateFile   string
	MaxSessions int
}

type Manager struct {
	cfg Config

	mu       sync.Mutex
	sessions map[string]*Session
	state    map[string]persistedSession

	closed bool
}

func NewManager(cfg Config) (*Manager, error) {
	if cfg.WorldWSURL == "" {
		return nil, fmt.Errorf("empty world ws url")
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 256
	}

	st, err := loadStateFile(cfg.StateFile)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		cfg:      cfg,
		sessions: map[string]*Session{},
		state:    st,
	}
	return m, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()

	for _, s := range sessions {
		s.Close()
	}
	return nil
}

func (m *Manager) GetStatus(ctx context.Context, sessionKey string) (Status, error) {
	s, err := m.getOrCreateSession(sessionKey)
	if err != nil {
		return Status{}, err
	}
	_ = ctx
	return s.Status(), nil
}

func (m *Manager) GetObs(ctx context.Context, sessionKey string, opts GetObsOpts) (ObsResult, error) {
	s, err := m.getOrCreateSession(sessionKey)
	if err != nil {
		return ObsResult{}, err
	}
	s.ResumeReconnect()
	return s.GetObs(ctx, opts)
}

func (m *Manager) GetCatalog(ctx context.Context, sessionKey, name string) (CatalogResult, error) {
	s, err := m.getOrCreateSession(sessionKey)
	if err != nil {
		return CatalogResult{}, err
	}
	s.ResumeReconnect()
	return s.GetCatalog(ctx, name)
}

func (m *Manager) GetEvents(ctx context.Context, sessionKey string, sinceCursor uint64, limit int) (GetEventsResult, error) {
	s, err := m.getOrCreateSession(sessionKey)
	if err != nil {
		return GetEventsResult{}, err
	}
	s.ResumeReconnect()
	return s.GetEvents(ctx, sinceCursor, limit)
}

func (m *Manager) Act(ctx context.Context, sessionKey string, args ActArgs) (ActResult, error) {
	s, err := m.getOrCreateSession(sessionKey)
	if err != nil {
		return ActResult{}, err
	}
	s.ResumeReconnect()
	return s.Act(ctx, args)
}

func (m *Manager) Disconnect(ctx context.Context, sessionKey string) error {
	s, err := m.getOrCreateSession(sessionKey)
	if err != nil {
		return err
	}
	_ = ctx
	s.DisconnectAndPause()
	return nil
}

func (m *Manager) ListWorlds(ctx context.Context, sessionKey string) ([]WorldInfo, error) {
	s, err := m.getOrCreateSession(sessionKey)
	if err != nil {
		return nil, err
	}
	s.ResumeReconnect()
	_ = ctx
	return s.ListWorlds(), nil
}

func (m *Manager) getOrCreateSession(key string) (*Session, error) {
	if key == "" {
		key = "default"
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, fmt.Errorf("bridge manager closed")
	}

	if s := m.sessions[key]; s != nil {
		return s, nil
	}

	// Enforce max sessions (simple LRU by lastUsedAt).
	if len(m.sessions) >= m.cfg.MaxSessions {
		var oldestKey string
		var oldest time.Time
		for k, s := range m.sessions {
			t := s.LastUsedAt()
			if oldestKey == "" || t.Before(oldest) {
				oldestKey = k
				oldest = t
			}
		}
		if oldestKey != "" {
			m.sessions[oldestKey].Close()
			delete(m.sessions, oldestKey)
		}
	}

	ps := m.state[key]
	s := NewSession(SessionConfig{
		Key:         key,
		WorldWSURL:  m.cfg.WorldWSURL,
		ResumeToken: ps.ResumeToken,
		AgentIDHint: ps.AgentID,
	}, m.onSessionUpdate)
	m.sessions[key] = s
	s.Start()
	return s, nil
}

func (m *Manager) onSessionUpdate(key string, upd sessionUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	ps := m.state[key]
	if upd.ResumeToken != "" {
		ps.ResumeToken = upd.ResumeToken
	}
	if upd.AgentID != "" {
		ps.AgentID = upd.AgentID
	}
	if !upd.LastConnectedAt.IsZero() {
		ps.LastConnectedAt = upd.LastConnectedAt.UTC().Format(time.RFC3339Nano)
	}
	if upd.LastObsTick != 0 {
		ps.LastObsTick = upd.LastObsTick
	}
	m.state[key] = ps

	// Persist whole file; updates are rare (WELCOME only), so this is fine.
	keys := make([]string, 0, len(m.state))
	for k := range m.state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := map[string]persistedSession{}
	for _, k := range keys {
		out[k] = m.state[k]
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	_ = writeFileAtomic(m.cfg.StateFile, append(b, '\n'))
}
