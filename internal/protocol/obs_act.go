package protocol

type ObsMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	Tick            uint64 `json:"tick"`
	AgentID         string `json:"agent_id"`

	World      WorldObs      `json:"world"`
	Self       SelfObs       `json:"self"`
	Inventory  []ItemStack   `json:"inventory"`
	Equipment  EquipmentObs  `json:"equipment"`
	LocalRules LocalRulesObs `json:"local_rules"`

	Voxels   VoxelsObs   `json:"voxels"`
	Entities []EntityObs `json:"entities"`
	Events   []Event     `json:"events"`
	Tasks    []TaskObs   `json:"tasks"`

	FunScore *FunScoreObs `json:"fun_score,omitempty"`

	PublicBoards []BoardObs `json:"public_boards,omitempty"`
	Memory       []MemoryKV `json:"memory,omitempty"`
}

type WorldObs struct {
	TimeOfDay           float64 `json:"time_of_day"` // 0..1
	Weather             string  `json:"weather"`
	SeasonDay           int     `json:"season_day"`
	Biome               string  `json:"biome"`
	ActiveEvent         string  `json:"active_event,omitempty"`
	ActiveEventEndsTick uint64  `json:"active_event_ends_tick,omitempty"`
}

type SelfObs struct {
	Pos     [3]int   `json:"pos"`
	Yaw     int      `json:"yaw"`
	HP      int      `json:"hp"`
	Hunger  int      `json:"hunger"`
	Stamina float64  `json:"stamina"`
	Status  []string `json:"status"`

	Reputation ReputationObs `json:"reputation,omitempty"`
}

type ReputationObs struct {
	Trade  float64 `json:"trade"`
	Build  float64 `json:"build"`
	Social float64 `json:"social"`
	Law    float64 `json:"law"`
}

type ItemStack struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
}

type EquipmentObs struct {
	MainHand string   `json:"main_hand"`
	Armor    []string `json:"armor"`
}

type LocalRulesObs struct {
	LandID      string             `json:"land_id,omitempty"`
	Owner       string             `json:"owner,omitempty"`
	Role        string             `json:"role,omitempty"` // "WILD","OWNER","MEMBER","VISITOR"
	Permissions map[string]bool    `json:"permissions"`
	Tax         map[string]float64 `json:"tax,omitempty"`

	MaintenanceDueTick uint64 `json:"maintenance_due_tick,omitempty"`
	MaintenanceStage   int    `json:"maintenance_stage,omitempty"`
}

type VoxelsObs struct {
	Center   [3]int         `json:"center"`
	Radius   int            `json:"radius"`
	Encoding string         `json:"encoding"` // "RLE" or "DELTA"
	Data     string         `json:"data,omitempty"`
	Ops      []VoxelDeltaOp `json:"ops,omitempty"`
}

type VoxelDeltaOp struct {
	D [3]int `json:"d"` // delta from center (dx,dy,dz)
	B uint16 `json:"b"` // block palette id
}

type EntityObs struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"` // "AGENT", "CHEST", ...
	Pos            [3]int   `json:"pos"`
	Tags           []string `json:"tags,omitempty"`
	ReputationHint float64  `json:"reputation_hint,omitempty"`

	// Optional payload for specialized entity types (e.g. "ITEM").
	Item  string `json:"item,omitempty"`
	Count int    `json:"count,omitempty"`
}

type Event map[string]interface{}

type TaskObs struct {
	TaskID   string  `json:"task_id"`
	Kind     string  `json:"kind"`
	Progress float64 `json:"progress"`
	Target   [3]int  `json:"target,omitempty"`
	EtaTicks int     `json:"eta_ticks,omitempty"`
}

type BoardObs struct {
	BoardID  string      `json:"board_id"`
	TopPosts []BoardPost `json:"top_posts"`
}

type BoardPost struct {
	PostID  string `json:"post_id"`
	Author  string `json:"author"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type MemoryKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type FunScoreObs struct {
	Novelty    int `json:"novelty"`
	Creation   int `json:"creation"`
	Social     int `json:"social"`
	Influence  int `json:"influence"`
	Narrative  int `json:"narrative"`
	RiskRescue int `json:"risk_rescue"`
}

// ACT (client -> server)
type ActMsg struct {
	Type            string       `json:"type"`
	ProtocolVersion string       `json:"protocol_version"`
	Tick            uint64       `json:"tick"`
	AgentID         string       `json:"agent_id"`
	Instants        []InstantReq `json:"instants,omitempty"`
	Tasks           []TaskReq    `json:"tasks,omitempty"`
	Cancel          []string     `json:"cancel,omitempty"`
}

type InstantReq struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Channel string `json:"channel,omitempty"`
	Text    string `json:"text,omitempty"`
	To      string `json:"to,omitempty"`

	Offer   [][]interface{} `json:"offer,omitempty"`   // [["PLANK",10], ...]
	Request [][]interface{} `json:"request,omitempty"` // same shape
	TradeID string          `json:"trade_id,omitempty"`

	Key      string `json:"key,omitempty"`
	Value    string `json:"value,omitempty"`
	TTLTicks int    `json:"ttl_ticks,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Limit    int    `json:"limit,omitempty"`

	BoardID string `json:"board_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Body    string `json:"body,omitempty"`

	TargetID string `json:"target_id,omitempty"` // e.g. container id

	TerminalID    string      `json:"terminal_id,omitempty"`
	ContractID    string      `json:"contract_id,omitempty"`
	ContractKind  string      `json:"contract_kind,omitempty"`
	Requirements  []ItemStack `json:"requirements,omitempty"`
	Reward        []ItemStack `json:"reward,omitempty"`
	Deposit       []ItemStack `json:"deposit,omitempty"`
	DeadlineTick  uint64      `json:"deadline_tick,omitempty"`
	DurationTicks int         `json:"duration_ticks,omitempty"`
	BlueprintID   string      `json:"blueprint_id,omitempty"`
	Anchor        [3]int      `json:"anchor,omitempty"`
	Rotation      int         `json:"rotation,omitempty"`

	LandID   string          `json:"land_id,omitempty"`
	Policy   map[string]bool `json:"policy,omitempty"`
	MemberID string          `json:"member_id,omitempty"`
	NewOwner string          `json:"new_owner,omitempty"`
	Radius   int             `json:"radius,omitempty"`

	OrgID   string `json:"org_id,omitempty"`
	OrgKind string `json:"org_kind,omitempty"`
	OrgName string `json:"org_name,omitempty"`
	ItemID  string `json:"item_id,omitempty"`
	Count   int    `json:"count,omitempty"`

	TemplateID string                 `json:"template_id,omitempty"`
	Params     map[string]interface{} `json:"params,omitempty"`
	LawID      string                 `json:"law_id,omitempty"`
	Choice     string                 `json:"choice,omitempty"`
}

type TaskReq struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Target    [3]int  `json:"target,omitempty"`
	Tolerance float64 `json:"tolerance,omitempty"`
	Distance  float64 `json:"distance,omitempty"`

	TargetID string `json:"target_id,omitempty"`
	Src      string `json:"src_container,omitempty"`
	Dst      string `json:"dst_container,omitempty"`

	BlockPos    [3]int `json:"block_pos,omitempty"`
	RecipeID    string `json:"recipe_id,omitempty"`
	Count       int    `json:"count,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	BlueprintID string `json:"blueprint_id,omitempty"`
	Anchor      [3]int `json:"anchor,omitempty"`
	Rotation    int    `json:"rotation,omitempty"`
	Radius      int    `json:"radius,omitempty"`
}
