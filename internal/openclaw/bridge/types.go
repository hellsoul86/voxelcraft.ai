package bridge

import "encoding/json"

// Status is returned by voxelcraft.get_status.
type Status struct {
	Connected      bool              `json:"connected"`
	AgentID        string            `json:"agent_id,omitempty"`
	ResumeToken    string            `json:"resume_token,omitempty"`
	WorldWSURL     string            `json:"world_ws_url"`
	LastObsTick    uint64            `json:"last_obs_tick"`
	CurrentWorldID string            `json:"current_world_id,omitempty"`
	CatalogDigests map[string]string `json:"catalog_digests,omitempty"`
	LastError      string            `json:"last_error,omitempty"`
}

type WorldInfo struct {
	WorldID          string `json:"world_id"`
	WorldType        string `json:"world_type,omitempty"`
	EntryPointID     string `json:"entry_point_id,omitempty"`
	RequiresPermit   bool   `json:"requires_permit,omitempty"`
	SwitchCooldown   int    `json:"switch_cooldown_ticks,omitempty"`
	ResetEveryTicks  int    `json:"reset_every_ticks,omitempty"`
	ResetNoticeTicks int    `json:"reset_notice_ticks,omitempty"`
}

type GetObsMode string

const (
	ObsModeFull     GetObsMode = "full"
	ObsModeNoVoxels GetObsMode = "no_voxels"
	ObsModeSummary  GetObsMode = "summary"
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
