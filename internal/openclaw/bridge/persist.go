package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type persistedSession struct {
	ResumeToken      string `json:"resume_token,omitempty"`
	AgentID          string `json:"agent_id,omitempty"`
	LastConnectedAt  string `json:"last_connected_at,omitempty"`
	LastObsTick      uint64 `json:"last_obs_tick,omitempty"`
}

func loadStateFile(path string) (map[string]persistedSession, error) {
	if path == "" {
		return map[string]persistedSession{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]persistedSession{}, nil
		}
		return nil, err
	}
	var m map[string]persistedSession
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	if m == nil {
		m = map[string]persistedSession{}
	}
	return m, nil
}

func (m persistedSession) connectedTime() time.Time {
	if m.LastConnectedAt == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, m.LastConnectedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

func writeFileAtomic(path string, b []byte) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

