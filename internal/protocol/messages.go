package protocol

// HELLO (client -> server)
type HelloMsg struct {
	Type            string            `json:"type"`
	ProtocolVersion string            `json:"protocol_version"`
	AgentName       string            `json:"agent_name"`
	Capabilities    HelloCapabilities `json:"capabilities"`
	Auth            *HelloAuth        `json:"auth,omitempty"`
	WorldPreference string            `json:"world_preference,omitempty"`
}

type HelloCapabilities struct {
	DeltaVoxels bool `json:"delta_voxels,omitempty"`
	MaxQueue    int  `json:"max_queue,omitempty"`
}

type HelloAuth struct {
	Token string `json:"token,omitempty"`
}

// WELCOME (server -> client)
type WelcomeMsg struct {
	Type            string         `json:"type"`
	ProtocolVersion string         `json:"protocol_version"`
	AgentID         string         `json:"agent_id"`
	ResumeToken     string         `json:"resume_token"`
	WorldParams     WorldParams    `json:"world_params"`
	Catalogs        CatalogDigests `json:"catalogs"`
	CurrentWorldID  string         `json:"current_world_id,omitempty"`
	WorldManifest   []WorldRef     `json:"world_manifest,omitempty"`
}

type WorldRef struct {
	WorldID          string `json:"world_id"`
	WorldType        string `json:"world_type,omitempty"`
	EntryPointID     string `json:"entry_point_id,omitempty"`
	RequiresPermit   bool   `json:"requires_permit,omitempty"`
	SwitchCooldown   int    `json:"switch_cooldown_ticks,omitempty"`
	ResetEveryTicks  int    `json:"reset_every_ticks,omitempty"`
	ResetNoticeTicks int    `json:"reset_notice_ticks,omitempty"`
}

type WorldParams struct {
	TickRateHz int    `json:"tick_rate_hz"`
	ChunkSize  [3]int `json:"chunk_size"`
	Height     int    `json:"height"`
	ObsRadius  int    `json:"obs_radius"`
	DayTicks   int    `json:"day_ticks"`
	Seed       int64  `json:"seed"`
}

type CatalogDigests struct {
	BlockPalette       DigestRef `json:"block_palette"`
	ItemPalette        DigestRef `json:"item_palette"`
	RecipesDigest      string    `json:"recipes_digest"`
	BlueprintsDigest   string    `json:"blueprints_digest"`
	LawTemplatesDigest string    `json:"law_templates_digest"`
	EventsDigest       string    `json:"events_digest"`
	TuningDigest       string    `json:"tuning_digest,omitempty"`
}

type DigestRef struct {
	Digest string `json:"digest"`
	Count  int    `json:"count"`
}

// CATALOG (server -> client): a chunk of catalog data.
// For MVP we send each catalog as a single part.
type CatalogMsg struct {
	Type            string      `json:"type"`
	ProtocolVersion string      `json:"protocol_version"`
	Name            string      `json:"name"`   // e.g. "block_palette"
	Digest          string      `json:"digest"` // sha256 hex
	Part            int         `json:"part"`
	TotalParts      int         `json:"total_parts"`
	Data            interface{} `json:"data"`
}
