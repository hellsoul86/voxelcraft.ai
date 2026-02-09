package snapshot

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

type Header struct {
	Version int    `json:"version"`
	WorldID string `json:"world_id"`
	Tick    uint64 `json:"tick"`
}

type SnapshotV1 struct {
	Header Header `json:"header"`

	Seed      int64 `json:"seed"`
	TickRate  int   `json:"tick_rate_hz"`
	DayTicks  int   `json:"day_ticks"`
	ObsRadius int   `json:"obs_radius"`
	Height    int   `json:"height"`
	BoundaryR int   `json:"boundary_r"`

	Weather          string `json:"weather"`
	WeatherUntilTick uint64 `json:"weather_until_tick"`
	ActiveEventID    string `json:"active_event_id"`
	ActiveEventEnds  uint64 `json:"active_event_ends_tick"`

	Chunks     []ChunkV1     `json:"chunks"`
	Agents     []AgentV1     `json:"agents"`
	Claims     []ClaimV1     `json:"claims"`
	Containers []ContainerV1 `json:"containers"`
	Trades     []TradeV1     `json:"trades"`
	Boards     []BoardV1     `json:"boards"`
	Contracts  []ContractV1  `json:"contracts"`
	Laws       []LawV1       `json:"laws"`
	Orgs       []OrgV1       `json:"orgs"`

	Structures []StructureV1 `json:"structures,omitempty"`

	Stats *StatsV1 `json:"stats,omitempty"`

	Counters CountersV1 `json:"counters"`
}

type CountersV1 struct {
	NextAgent    uint64 `json:"next_agent"`
	NextTask     uint64 `json:"next_task"`
	NextLand     uint64 `json:"next_land"`
	NextTrade    uint64 `json:"next_trade"`
	NextPost     uint64 `json:"next_post"`
	NextContract uint64 `json:"next_contract"`
	NextLaw      uint64 `json:"next_law"`
	NextOrg      uint64 `json:"next_org"`
}

type ChunkV1 struct {
	CX     int      `json:"cx"`
	CZ     int      `json:"cz"`
	Height int      `json:"height"`
	Blocks []uint16 `json:"blocks"`
}

type AgentV1 struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	OrgID string `json:"org_id,omitempty"`
	Pos   [3]int `json:"pos"`
	Yaw   int    `json:"yaw"`

	HP            int            `json:"hp"`
	Hunger        int            `json:"hunger"`
	StaminaMilli  int            `json:"stamina_milli"`
	RepTrade      int            `json:"rep_trade"`
	RepBuild      int            `json:"rep_build"`
	RepSocial     int            `json:"rep_social"`
	RepLaw        int            `json:"rep_law"`
	FunNovelty    int            `json:"fun_novelty"`
	FunCreation   int            `json:"fun_creation"`
	FunSocial     int            `json:"fun_social"`
	FunInfluence  int            `json:"fun_influence"`
	FunNarrative  int            `json:"fun_narrative"`
	FunRiskRescue int            `json:"fun_risk_rescue"`
	Inventory     map[string]int `json:"inventory"`

	SeenBiomes  []string              `json:"seen_biomes,omitempty"`
	SeenRecipes []string              `json:"seen_recipes,omitempty"`
	SeenEvents  []string              `json:"seen_events,omitempty"`
	FunDecay    map[string]FunDecayV1 `json:"fun_decay,omitempty"`

	MoveTask *MovementTaskV1 `json:"move_task,omitempty"`
	WorkTask *WorkTaskV1     `json:"work_task,omitempty"`
}

type FunDecayV1 struct {
	StartTick uint64 `json:"start_tick"`
	Count     int    `json:"count"`
}

type MovementTaskV1 struct {
	TaskID      string  `json:"task_id"`
	Kind        string  `json:"kind"`
	Target      [3]int  `json:"target"`
	Tolerance   float64 `json:"tolerance"`
	StartPos    [3]int  `json:"start_pos"`
	StartedTick uint64  `json:"started_tick"`
}

type WorkTaskV1 struct {
	TaskID string `json:"task_id"`
	Kind   string `json:"kind"`

	BlockPos [3]int `json:"block_pos,omitempty"`

	RecipeID string `json:"recipe_id,omitempty"`
	ItemID   string `json:"item_id,omitempty"`
	Count    int    `json:"count,omitempty"`

	BlueprintID string `json:"blueprint_id,omitempty"`
	Anchor      [3]int `json:"anchor,omitempty"`
	Rotation    int    `json:"rotation,omitempty"`
	BuildIndex  int    `json:"build_index,omitempty"`

	TargetID     string `json:"target_id,omitempty"`
	SrcContainer string `json:"src_container,omitempty"`
	DstContainer string `json:"dst_container,omitempty"`

	StartedTick uint64 `json:"started_tick"`
	WorkTicks   int    `json:"work_ticks"`
}

type ClaimV1 struct {
	LandID string       `json:"land_id"`
	Owner  string       `json:"owner"`
	Anchor [3]int       `json:"anchor"`
	Radius int          `json:"radius"`
	Flags  ClaimFlagsV1 `json:"flags"`

	Members []string `json:"members,omitempty"`

	MarketTax     float64 `json:"market_tax,omitempty"`
	CurfewEnabled bool    `json:"curfew_enabled,omitempty"`
	CurfewStart   float64 `json:"curfew_start,omitempty"`
	CurfewEnd     float64 `json:"curfew_end,omitempty"`

	FineBreakEnabled  bool   `json:"fine_break_enabled,omitempty"`
	FineBreakItem     string `json:"fine_break_item,omitempty"`
	FineBreakPerBlock int    `json:"fine_break_per_block,omitempty"`

	AccessPassEnabled bool   `json:"access_pass_enabled,omitempty"`
	AccessTicketItem  string `json:"access_ticket_item,omitempty"`
	AccessTicketCost  int    `json:"access_ticket_cost,omitempty"`

	MaintenanceDueTick uint64 `json:"maintenance_due_tick,omitempty"`
	MaintenanceStage   int    `json:"maintenance_stage,omitempty"`
}

type ClaimFlagsV1 struct {
	AllowBuild  bool `json:"allow_build"`
	AllowBreak  bool `json:"allow_break"`
	AllowDamage bool `json:"allow_damage"`
	AllowTrade  bool `json:"allow_trade"`
}

type ContainerV1 struct {
	Type      string                    `json:"type"`
	Pos       [3]int                    `json:"pos"`
	Inventory map[string]int            `json:"inventory"`
	Reserved  map[string]int            `json:"reserved"`
	Owed      map[string]map[string]int `json:"owed"`
}

type TradeV1 struct {
	TradeID     string         `json:"trade_id"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	Offer       map[string]int `json:"offer"`
	Request     map[string]int `json:"request"`
	CreatedTick uint64         `json:"created_tick"`
}

type BoardV1 struct {
	BoardID string        `json:"board_id"`
	Posts   []BoardPostV1 `json:"posts"`
}

type BoardPostV1 struct {
	PostID string `json:"post_id"`
	Author string `json:"author"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Tick   uint64 `json:"tick"`
}

type ContractV1 struct {
	ContractID   string         `json:"contract_id"`
	TerminalPos  [3]int         `json:"terminal_pos"`
	Poster       string         `json:"poster"`
	Acceptor     string         `json:"acceptor"`
	Kind         string         `json:"kind"`
	State        string         `json:"state"`
	Requirements map[string]int `json:"requirements"`
	Reward       map[string]int `json:"reward"`
	Deposit      map[string]int `json:"deposit"`
	BlueprintID  string         `json:"blueprint_id"`
	Anchor       [3]int         `json:"anchor"`
	Rotation     int            `json:"rotation"`
	CreatedTick  uint64         `json:"created_tick"`
	DeadlineTick uint64         `json:"deadline_tick"`
}

type LawV1 struct {
	LawID      string            `json:"law_id"`
	LandID     string            `json:"land_id"`
	TemplateID string            `json:"template_id"`
	Title      string            `json:"title"`
	Params     map[string]string `json:"params"`
	Status     string            `json:"status"`

	ProposedBy     string            `json:"proposed_by"`
	ProposedTick   uint64            `json:"proposed_tick"`
	NoticeEndsTick uint64            `json:"notice_ends_tick"`
	VoteEndsTick   uint64            `json:"vote_ends_tick"`
	Votes          map[string]string `json:"votes"`
}

type OrgV1 struct {
	OrgID       string            `json:"org_id"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	CreatedTick uint64            `json:"created_tick"`
	Members     map[string]string `json:"members"`
	Treasury    map[string]int    `json:"treasury"`
}

type StructureV1 struct {
	StructureID string `json:"structure_id"`
	BlueprintID string `json:"blueprint_id"`
	BuilderID   string `json:"builder_id"`
	Anchor      [3]int `json:"anchor"`
	Min         [3]int `json:"min"`
	Max         [3]int `json:"max"`

	CompletedTick uint64 `json:"completed_tick"`
	AwardDueTick  uint64 `json:"award_due_tick"`
	Awarded       bool   `json:"awarded"`

	UsedBy           map[string]uint64 `json:"used_by,omitempty"`
	LastInfluenceDay int               `json:"last_influence_day,omitempty"`
}

type StatsV1 struct {
	BucketTicks uint64          `json:"bucket_ticks"`
	WindowTicks uint64          `json:"window_ticks"`
	CurIdx      int             `json:"cur_idx"`
	CurBase     uint64          `json:"cur_base"`
	Buckets     []StatsBucketV1 `json:"buckets"`
	SeenChunks  []ChunkKeyV1    `json:"seen_chunks,omitempty"`
}

type StatsBucketV1 struct {
	Trades             int `json:"trades"`
	Denied             int `json:"denied"`
	ChunksDiscovered   int `json:"chunks_discovered"`
	BlueprintsComplete int `json:"blueprints_complete"`
}

type ChunkKeyV1 struct {
	CX int `json:"cx"`
	CZ int `json:"cz"`
}

func WriteSnapshot(path string, snap SnapshotV1) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return err
	}
	defer enc.Close()

	bw := bufio.NewWriterSize(enc, 256*1024)
	defer bw.Flush()

	hb, _ := json.Marshal(snap.Header)
	if _, err := bw.Write(hb); err != nil {
		return err
	}
	if err := bw.WriteByte('\n'); err != nil {
		return err
	}

	if err := gob.NewEncoder(bw).Encode(&snap); err != nil {
		return fmt.Errorf("gob encode: %w", err)
	}
	return nil
}

func ReadSnapshot(path string) (SnapshotV1, error) {
	var snap SnapshotV1
	f, err := os.Open(path)
	if err != nil {
		return snap, err
	}
	defer f.Close()

	dec, err := zstd.NewReader(f)
	if err != nil {
		return snap, err
	}
	defer dec.Close()

	br := bufio.NewReaderSize(dec, 256*1024)

	// Read header line (ignore it for now, gob also contains header).
	_, _ = br.ReadBytes('\n')

	if err := gob.NewDecoder(br).Decode(&snap); err != nil {
		return snap, fmt.Errorf("gob decode: %w", err)
	}
	return snap, nil
}
