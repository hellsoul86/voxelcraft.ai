package bridge

import "encoding/json"

// Status is returned by voxelcraft.get_status.
type Status struct {
	Connected     bool              `json:"connected"`
	AgentID       string            `json:"agent_id,omitempty"`
	ResumeToken   string            `json:"resume_token,omitempty"`
	WorldWSURL    string            `json:"world_ws_url"`
	LastObsTick   uint64            `json:"last_obs_tick"`
	CatalogDigests map[string]string `json:"catalog_digests,omitempty"`
	LastError     string            `json:"last_error,omitempty"`
}

type GetObsMode string

const (
	ObsModeFull    GetObsMode = "full"
	ObsModeNoVoxels GetObsMode = "no_voxels"
	ObsModeSummary GetObsMode = "summary"
)

type GetObsOpts struct {
	Mode        GetObsMode `json:"mode"`
	WaitNewTick bool       `json:"wait_new_tick"`
	TimeoutMS   int        `json:"timeout_ms"`
}

type ObsResult struct {
	Tick    uint64          `json:"tick"`
	AgentID string          `json:"agent_id"`
	Obs     json.RawMessage `json:"obs"`
}

type CatalogResult struct {
	Name   string          `json:"name"`
	Digest string          `json:"digest"`
	Data   json.RawMessage `json:"data"`
}

type ActArgs struct {
	Instants json.RawMessage `json:"instants,omitempty"`
	Tasks    json.RawMessage `json:"tasks,omitempty"`
	Cancel   []string        `json:"cancel,omitempty"`
}

type ActResult struct {
	Sent     bool   `json:"sent"`
	TickUsed uint64 `json:"tick_used"`
	AgentID  string `json:"agent_id"`
}

