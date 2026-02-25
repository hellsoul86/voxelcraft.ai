package indexdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tuning"
	"voxelcraft.ai/internal/sim/world"
)

type SQLiteIndex struct {
	db *sql.DB

	ch   chan req
	wg   sync.WaitGroup
	once sync.Once

	closed atomic.Bool

	dropTickTotal          atomic.Uint64
	dropAuditTotal         atomic.Uint64
	dropSnapshotTotal      atomic.Uint64
	dropSnapshotStateTotal atomic.Uint64
	dropSeasonTotal        atomic.Uint64

	// audit sequencing (assigned in the writer goroutine)
}

type SQLiteStats struct {
	QueueDepth             int
	QueueCapacity          int
	DropTickTotal          uint64
	DropAuditTotal         uint64
	DropSnapshotTotal      uint64
	DropSnapshotStateTotal uint64
	DropSeasonTotal        uint64
}

type reqKind int

const (
	reqTick reqKind = iota + 1
	reqAudit
	reqSnapshot
	reqSnapshotState
	reqSeason
)

type req struct {
	kind reqKind

	tick     world.TickLogEntry
	audit    world.AuditEntry
	snapshot snapshotRow
	state    snapshotState
	season   seasonRow
}

type snapshotRow struct {
	Tick       uint64
	Path       string
	Seed       int64
	Height     int
	Chunks     int
	Agents     int
	Claims     int
	Containers int
	Contracts  int
	Laws       int
	Orgs       int
}

type snapshotState struct {
	Tick    uint64
	WorldID string

	Weather           string
	WeatherUntilTick  uint64
	ActiveEventID     string
	ActiveEventStart  uint64
	ActiveEventEnds   uint64
	ActiveEventCenter [3]int
	ActiveEventRadius int

	Trades []snapshot.TradeV1
	Boards []snapshot.BoardV1

	Agents    []snapshot.AgentV1
	Claims    []snapshot.ClaimV1
	Contracts []snapshot.ContractV1
	Laws      []snapshot.LawV1
	Orgs      []snapshot.OrgV1
}

type seasonRow struct {
	Season     int
	EndTick    uint64
	Path       string
	Seed       int64
	RecordedAt string
}

func OpenSQLite(path string) (*SQLiteIndex, error) {
	if path == "" {
		return nil, fmt.Errorf("empty db path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := initPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &SQLiteIndex{
		db: db,
		// High buffer: allow bursty audit writes (e.g. many agents building) without stalling the sim.
		ch: make(chan req, 262144),
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.loop()
	}()
	return s, nil
}

func initPragmas(db *sql.DB) error {
	// WAL is much faster for append-style workloads.
	// NORMAL is a decent durability/perf tradeoff for a secondary index.
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA temp_store=MEMORY;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return err
		}
	}
	return nil
}

func initSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS catalogs (
			name TEXT PRIMARY KEY,
			digest TEXT NOT NULL,
			json TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS ticks (
			tick INTEGER PRIMARY KEY,
			digest TEXT NOT NULL,
			joins INTEGER NOT NULL,
			leaves INTEGER NOT NULL,
			actions INTEGER NOT NULL,
			raw_json TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS joins (
			tick INTEGER NOT NULL,
			agent_id TEXT NOT NULL,
			name TEXT NOT NULL,
			PRIMARY KEY (tick, agent_id)
		);`,
		`CREATE TABLE IF NOT EXISTS leaves (
			tick INTEGER NOT NULL,
			agent_id TEXT NOT NULL,
			PRIMARY KEY (tick, agent_id)
		);`,
		`CREATE TABLE IF NOT EXISTS actions (
			tick INTEGER NOT NULL,
			seq INTEGER NOT NULL,
			agent_id TEXT NOT NULL,
			act_json TEXT NOT NULL,
			PRIMARY KEY (tick, seq)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_actions_agent_tick ON actions(agent_id, tick);`,
		`CREATE TABLE IF NOT EXISTS audits (
			tick INTEGER NOT NULL,
			seq INTEGER NOT NULL,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			z INTEGER NOT NULL,
			from_block INTEGER NOT NULL,
			to_block INTEGER NOT NULL,
			reason TEXT,
			raw_json TEXT NOT NULL,
			PRIMARY KEY (tick, seq)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audits_actor_tick ON audits(actor, tick);`,
		`CREATE INDEX IF NOT EXISTS idx_audits_pos_tick ON audits(x, z, y, tick);`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			tick INTEGER PRIMARY KEY,
			path TEXT NOT NULL,
			seed INTEGER NOT NULL,
			height INTEGER NOT NULL,
			chunks INTEGER NOT NULL,
			agents INTEGER NOT NULL,
			claims INTEGER NOT NULL,
			containers INTEGER NOT NULL,
			contracts INTEGER NOT NULL,
			laws INTEGER NOT NULL,
			orgs INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS snapshot_agents (
			tick INTEGER NOT NULL,
			agent_id TEXT NOT NULL,
			name TEXT NOT NULL,
			org_id TEXT,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			z INTEGER NOT NULL,
			yaw INTEGER NOT NULL,
			hp INTEGER NOT NULL,
			hunger INTEGER NOT NULL,
			stamina_milli INTEGER NOT NULL,
			rep_trade INTEGER NOT NULL,
			rep_build INTEGER NOT NULL,
			rep_social INTEGER NOT NULL,
			rep_law INTEGER NOT NULL,
			fun_novelty INTEGER NOT NULL,
			fun_creation INTEGER NOT NULL,
			fun_social INTEGER NOT NULL,
			fun_influence INTEGER NOT NULL,
			fun_narrative INTEGER NOT NULL,
			fun_risk_rescue INTEGER NOT NULL,
			inventory_json TEXT NOT NULL,
			PRIMARY KEY (tick, agent_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_agents_agent_tick ON snapshot_agents(agent_id, tick);`,
		`CREATE TABLE IF NOT EXISTS snapshot_claims (
			tick INTEGER NOT NULL,
			land_id TEXT NOT NULL,
			owner TEXT NOT NULL,
			ax INTEGER NOT NULL,
			ay INTEGER NOT NULL,
			az INTEGER NOT NULL,
			radius INTEGER NOT NULL,
			allow_build INTEGER NOT NULL,
			allow_break INTEGER NOT NULL,
			allow_damage INTEGER NOT NULL,
			allow_trade INTEGER NOT NULL,
			market_tax REAL NOT NULL,
			curfew_enabled INTEGER NOT NULL,
			curfew_start REAL NOT NULL,
			curfew_end REAL NOT NULL,
			fine_break_enabled INTEGER NOT NULL,
			fine_break_item TEXT NOT NULL,
			fine_break_per_block INTEGER NOT NULL,
			access_pass_enabled INTEGER NOT NULL,
			access_ticket_item TEXT NOT NULL,
			access_ticket_cost INTEGER NOT NULL,
			maintenance_due_tick INTEGER NOT NULL,
			maintenance_stage INTEGER NOT NULL,
			members_json TEXT NOT NULL,
			PRIMARY KEY (tick, land_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_claims_owner_tick ON snapshot_claims(owner, tick);`,
		`CREATE TABLE IF NOT EXISTS snapshot_orgs (
			tick INTEGER NOT NULL,
			org_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			created_tick INTEGER NOT NULL,
			members_json TEXT NOT NULL,
			treasury_json TEXT NOT NULL,
			PRIMARY KEY (tick, org_id)
		);`,
		`CREATE TABLE IF NOT EXISTS snapshot_contracts (
			tick INTEGER NOT NULL,
			contract_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			state TEXT NOT NULL,
			poster TEXT NOT NULL,
			acceptor TEXT NOT NULL,
			terminal_x INTEGER NOT NULL,
			terminal_y INTEGER NOT NULL,
			terminal_z INTEGER NOT NULL,
			created_tick INTEGER NOT NULL,
			deadline_tick INTEGER NOT NULL,
			requirements_json TEXT NOT NULL,
			reward_json TEXT NOT NULL,
			deposit_json TEXT NOT NULL,
			blueprint_id TEXT NOT NULL,
			anchor_x INTEGER NOT NULL,
			anchor_y INTEGER NOT NULL,
			anchor_z INTEGER NOT NULL,
			rotation INTEGER NOT NULL,
			PRIMARY KEY (tick, contract_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_contracts_state_tick ON snapshot_contracts(state, tick);`,
		`CREATE TABLE IF NOT EXISTS snapshot_laws (
			tick INTEGER NOT NULL,
			law_id TEXT NOT NULL,
			land_id TEXT NOT NULL,
			template_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			proposed_by TEXT NOT NULL,
			proposed_tick INTEGER NOT NULL,
			notice_ends_tick INTEGER NOT NULL,
			vote_ends_tick INTEGER NOT NULL,
			params_json TEXT NOT NULL,
			votes_json TEXT NOT NULL,
			PRIMARY KEY (tick, law_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_laws_land_tick ON snapshot_laws(land_id, tick);`,
		`CREATE TABLE IF NOT EXISTS snapshot_world (
			tick INTEGER PRIMARY KEY,
			weather TEXT NOT NULL,
			weather_until_tick INTEGER NOT NULL,
			active_event_id TEXT NOT NULL,
			active_event_start_tick INTEGER NOT NULL,
			active_event_ends_tick INTEGER NOT NULL,
			active_event_center_x INTEGER NOT NULL,
			active_event_center_y INTEGER NOT NULL,
			active_event_center_z INTEGER NOT NULL,
			active_event_radius INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS snapshot_trades (
			tick INTEGER NOT NULL,
			trade_id TEXT NOT NULL,
			from_agent TEXT NOT NULL,
			to_agent TEXT NOT NULL,
			created_tick INTEGER NOT NULL,
			offer_json TEXT NOT NULL,
			request_json TEXT NOT NULL,
			PRIMARY KEY (tick, trade_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_trades_from_tick ON snapshot_trades(from_agent, tick);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_trades_to_tick ON snapshot_trades(to_agent, tick);`,
		`CREATE TABLE IF NOT EXISTS snapshot_boards (
			tick INTEGER NOT NULL,
			board_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			x INTEGER,
			y INTEGER,
			z INTEGER,
			posts_count INTEGER NOT NULL,
			PRIMARY KEY (tick, board_id)
		);`,
		`CREATE TABLE IF NOT EXISTS snapshot_board_posts (
			tick INTEGER NOT NULL,
			board_id TEXT NOT NULL,
			post_id TEXT NOT NULL,
			author TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			post_tick INTEGER NOT NULL,
			PRIMARY KEY (tick, board_id, post_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_board_posts_author_tick ON snapshot_board_posts(author, tick);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_board_posts_board_posttick ON snapshot_board_posts(board_id, post_tick);`,
		`CREATE TABLE IF NOT EXISTS seasons (
			season INTEGER PRIMARY KEY,
			end_tick INTEGER NOT NULL,
			seed INTEGER NOT NULL,
			snapshot_path TEXT NOT NULL,
			recorded_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_seasons_end_tick ON seasons(end_tick);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteIndex) Close() error {
	var err error
	s.once.Do(func() {
		s.closed.Store(true)
		close(s.ch)
		s.wg.Wait()
		err = s.db.Close()
	})
	return err
}

func (s *SQLiteIndex) Stats() SQLiteStats {
	if s == nil {
		return SQLiteStats{}
	}
	return SQLiteStats{
		QueueDepth:             len(s.ch),
		QueueCapacity:          cap(s.ch),
		DropTickTotal:          s.dropTickTotal.Load(),
		DropAuditTotal:         s.dropAuditTotal.Load(),
		DropSnapshotTotal:      s.dropSnapshotTotal.Load(),
		DropSnapshotStateTotal: s.dropSnapshotStateTotal.Load(),
		DropSeasonTotal:        s.dropSeasonTotal.Load(),
	}
}

func (s *SQLiteIndex) logDrop(kind string, total uint64) {
	if s == nil {
		return
	}
	if total == 1 || total%100 == 0 {
		log.Printf("[indexdb sqlite] queue full; drop kind=%s dropped_total=%d queue_depth=%d queue_capacity=%d", kind, total, len(s.ch), cap(s.ch))
	}
}

func (s *SQLiteIndex) WriteTick(entry world.TickLogEntry) error {
	if s == nil || s.closed.Load() {
		return nil
	}
	select {
	case s.ch <- req{kind: reqTick, tick: entry}:
	default:
		// Drop if the indexer falls behind; JSONL logs remain the source of truth.
		drops := s.dropTickTotal.Add(1)
		s.logDrop("tick", drops)
	}
	return nil
}

func (s *SQLiteIndex) WriteAudit(entry world.AuditEntry) error {
	if s == nil || s.closed.Load() {
		return nil
	}
	select {
	case s.ch <- req{kind: reqAudit, audit: entry}:
	default:
		drops := s.dropAuditTotal.Add(1)
		s.logDrop("audit", drops)
	}
	return nil
}

func (s *SQLiteIndex) RecordSnapshot(path string, snap snapshot.SnapshotV1) {
	if s == nil || s.closed.Load() {
		return
	}
	r := snapshotRow{
		Tick:       snap.Header.Tick,
		Path:       path,
		Seed:       snap.Seed,
		Height:     snap.Height,
		Chunks:     len(snap.Chunks),
		Agents:     len(snap.Agents),
		Claims:     len(snap.Claims),
		Containers: len(snap.Containers),
		Contracts:  len(snap.Contracts),
		Laws:       len(snap.Laws),
		Orgs:       len(snap.Orgs),
	}
	select {
	case s.ch <- req{kind: reqSnapshot, snapshot: r}:
	default:
		drops := s.dropSnapshotTotal.Add(1)
		s.logDrop("snapshot", drops)
	}
}

func (s *SQLiteIndex) RecordSnapshotState(snap snapshot.SnapshotV1) {
	if s == nil || s.closed.Load() {
		return
	}
	st := snapshotState{
		Tick:              snap.Header.Tick,
		WorldID:           snap.Header.WorldID,
		Weather:           snap.Weather,
		WeatherUntilTick:  snap.WeatherUntilTick,
		ActiveEventID:     snap.ActiveEventID,
		ActiveEventStart:  snap.ActiveEventStart,
		ActiveEventEnds:   snap.ActiveEventEnds,
		ActiveEventCenter: snap.ActiveEventCenter,
		ActiveEventRadius: snap.ActiveEventRadius,
		Trades:            snap.Trades,
		Boards:            snap.Boards,
		Agents:            snap.Agents,
		Claims:            snap.Claims,
		Contracts:         snap.Contracts,
		Laws:              snap.Laws,
		Orgs:              snap.Orgs,
	}
	select {
	case s.ch <- req{kind: reqSnapshotState, state: st}:
	default:
		drops := s.dropSnapshotStateTotal.Add(1)
		s.logDrop("snapshot_state", drops)
	}
}

func (s *SQLiteIndex) RecordSeason(season int, endTick uint64, archivedSnapshotPath string, seed int64) {
	if s == nil || s.closed.Load() {
		return
	}
	if season <= 0 || archivedSnapshotPath == "" {
		return
	}
	r := seasonRow{
		Season:     season,
		EndTick:    endTick,
		Path:       archivedSnapshotPath,
		Seed:       seed,
		RecordedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	select {
	case s.ch <- req{kind: reqSeason, season: r}:
	default:
		drops := s.dropSeasonTotal.Add(1)
		s.logDrop("season", drops)
	}
}

func (s *SQLiteIndex) UpsertCatalogs(configDir string, cats *catalogs.Catalogs, tune tuning.Tuning) error {
	if s == nil {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Raw json for base catalogs.
	raw := map[string][]byte{}
	read := func(name, path string) {
		b, err := os.ReadFile(path)
		if err != nil {
			return
		}
		raw[name] = b
	}
	if configDir != "" {
		read("blocks_defs", filepath.Join(configDir, "blocks.json"))
		read("items_defs", filepath.Join(configDir, "items.json"))
		read("recipes", filepath.Join(configDir, "recipes.json"))
		read("law_templates", filepath.Join(configDir, "law_templates.json"))
	}

	type kv struct {
		name   string
		digest string
		json   []byte
	}
	var rows []kv
	if b := raw["blocks_defs"]; len(b) > 0 {
		rows = append(rows, kv{name: "blocks_defs", digest: cats.Blocks.DefsDigest, json: b})
	}
	if b, _ := json.Marshal(cats.Blocks.Palette); len(b) > 0 {
		rows = append(rows, kv{name: "blocks_palette", digest: cats.Blocks.PaletteDigest, json: b})
	}
	if b := raw["items_defs"]; len(b) > 0 {
		rows = append(rows, kv{name: "items_defs", digest: cats.Items.DefsDigest, json: b})
	}
	if b, _ := json.Marshal(cats.Items.Palette); len(b) > 0 {
		rows = append(rows, kv{name: "items_palette", digest: cats.Items.PaletteDigest, json: b})
	}
	if b := raw["recipes"]; len(b) > 0 {
		rows = append(rows, kv{name: "recipes", digest: cats.Recipes.Digest, json: b})
	}
	{
		// Canonicalize blueprints/events to stable JSON for easier querying.
		bps := make([]catalogs.BlueprintDef, 0, len(cats.Blueprints.ByID))
		for _, bp := range cats.Blueprints.ByID {
			bps = append(bps, bp)
		}
		sort.Slice(bps, func(i, j int) bool { return bps[i].ID < bps[j].ID })
		if b, _ := json.Marshal(bps); len(b) > 0 {
			rows = append(rows, kv{name: "blueprints", digest: cats.Blueprints.Digest, json: b})
		}
	}
	{
		laws := cats.Laws
		if b, _ := json.Marshal(laws); len(b) > 0 {
			rows = append(rows, kv{name: "law_catalog", digest: cats.Laws.Digest, json: b})
		}
	}
	{
		evs := make([]catalogs.EventTemplate, 0, len(cats.Events.ByID))
		for _, ev := range cats.Events.ByID {
			evs = append(evs, ev)
		}
		sort.Slice(evs, func(i, j int) bool { return evs[i].ID < evs[j].ID })
		if b, _ := json.Marshal(evs); len(b) > 0 {
			rows = append(rows, kv{name: "events", digest: cats.Events.Digest, json: b})
		}
	}
	if b := raw["law_templates"]; len(b) > 0 {
		rows = append(rows, kv{name: "law_templates", digest: cats.Laws.Digest, json: b})
	}

	// Tuning: store the values we actually apply (canonical JSON).
	{
		b, _ := json.Marshal(tune)
		sum := sha256.Sum256(b)
		digest := hex.EncodeToString(sum[:])
		rows = append(rows, kv{name: "tuning", digest: digest, json: b})
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT OR REPLACE INTO meta(key,value) VALUES('schema_version','3')`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO catalogs(name,digest,json,updated_at) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if r.name == "" || r.digest == "" || len(r.json) == 0 {
			continue
		}
		if _, err := stmt.Exec(r.name, r.digest, string(r.json), now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func parseContainerXYZ(id string) (typ string, pos [3]int, ok bool) {
	parts := strings.SplitN(id, "@", 2)
	if len(parts) != 2 {
		return "", [3]int{}, false
	}
	typ = parts[0]
	coord := strings.Split(parts[1], ",")
	if len(coord) != 3 {
		return "", [3]int{}, false
	}
	x, err1 := strconv.Atoi(coord[0])
	y, err2 := strconv.Atoi(coord[1])
	z, err3 := strconv.Atoi(coord[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", [3]int{}, false
	}
	return typ, [3]int{x, y, z}, true
}

func (s *SQLiteIndex) loop() {
	ctx := context.Background()

	// Prepared statements (on db; executed within tx).
	insertTick, _ := s.db.Prepare(`INSERT OR REPLACE INTO ticks(tick,digest,joins,leaves,actions,raw_json) VALUES(?,?,?,?,?,?)`)
	insertJoin, _ := s.db.Prepare(`INSERT OR REPLACE INTO joins(tick,agent_id,name) VALUES(?,?,?)`)
	insertLeave, _ := s.db.Prepare(`INSERT OR REPLACE INTO leaves(tick,agent_id) VALUES(?,?)`)
	insertAction, _ := s.db.Prepare(`INSERT OR REPLACE INTO actions(tick,seq,agent_id,act_json) VALUES(?,?,?,?)`)
	insertAudit, _ := s.db.Prepare(`INSERT OR REPLACE INTO audits(tick,seq,actor,action,x,y,z,from_block,to_block,reason,raw_json) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	insertSnapshot, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshots(tick,path,seed,height,chunks,agents,claims,containers,contracts,laws,orgs) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	insertSnapAgent, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_agents(
		tick,agent_id,name,org_id,x,y,z,yaw,hp,hunger,stamina_milli,
		rep_trade,rep_build,rep_social,rep_law,
		fun_novelty,fun_creation,fun_social,fun_influence,fun_narrative,fun_risk_rescue,
		inventory_json
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	insertSnapClaim, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_claims(
		tick,land_id,owner,ax,ay,az,radius,
		allow_build,allow_break,allow_damage,allow_trade,
		market_tax,curfew_enabled,curfew_start,curfew_end,
		fine_break_enabled,fine_break_item,fine_break_per_block,
		access_pass_enabled,access_ticket_item,access_ticket_cost,
		maintenance_due_tick,maintenance_stage,members_json
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	insertSnapOrg, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_orgs(
		tick,org_id,kind,name,created_tick,members_json,treasury_json
	) VALUES(?,?,?,?,?,?,?)`)
	insertSnapContract, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_contracts(
		tick,contract_id,kind,state,poster,acceptor,
		terminal_x,terminal_y,terminal_z,
		created_tick,deadline_tick,
		requirements_json,reward_json,deposit_json,
		blueprint_id,anchor_x,anchor_y,anchor_z,rotation
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	insertSnapLaw, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_laws(
		tick,law_id,land_id,template_id,title,status,
		proposed_by,proposed_tick,notice_ends_tick,vote_ends_tick,
		params_json,votes_json
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`)
	insertSnapWorld, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_world(
		tick,weather,weather_until_tick,
		active_event_id,active_event_start_tick,active_event_ends_tick,
		active_event_center_x,active_event_center_y,active_event_center_z,
		active_event_radius
	) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	insertSnapTrade, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_trades(
		tick,trade_id,from_agent,to_agent,created_tick,offer_json,request_json
	) VALUES(?,?,?,?,?,?,?)`)
	insertSnapBoard, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_boards(
		tick,board_id,kind,x,y,z,posts_count
	) VALUES(?,?,?,?,?,?,?)`)
	insertSnapBoardPost, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshot_board_posts(
		tick,board_id,post_id,author,title,body,post_tick
	) VALUES(?,?,?,?,?,?,?)`)
	insertSeason, _ := s.db.Prepare(`INSERT OR REPLACE INTO seasons(season,end_tick,seed,snapshot_path,recorded_at) VALUES(?,?,?,?,?)`)
	defer func() {
		if insertTick != nil {
			_ = insertTick.Close()
		}
		if insertJoin != nil {
			_ = insertJoin.Close()
		}
		if insertLeave != nil {
			_ = insertLeave.Close()
		}
		if insertAction != nil {
			_ = insertAction.Close()
		}
		if insertAudit != nil {
			_ = insertAudit.Close()
		}
		if insertSnapshot != nil {
			_ = insertSnapshot.Close()
		}
		if insertSnapAgent != nil {
			_ = insertSnapAgent.Close()
		}
		if insertSnapClaim != nil {
			_ = insertSnapClaim.Close()
		}
		if insertSnapOrg != nil {
			_ = insertSnapOrg.Close()
		}
		if insertSnapContract != nil {
			_ = insertSnapContract.Close()
		}
		if insertSnapLaw != nil {
			_ = insertSnapLaw.Close()
		}
		if insertSnapWorld != nil {
			_ = insertSnapWorld.Close()
		}
		if insertSnapTrade != nil {
			_ = insertSnapTrade.Close()
		}
		if insertSnapBoard != nil {
			_ = insertSnapBoard.Close()
		}
		if insertSnapBoardPost != nil {
			_ = insertSnapBoardPost.Close()
		}
		if insertSeason != nil {
			_ = insertSeason.Close()
		}
	}()

	var (
		tx            *sql.Tx
		opCount       int
		lastCommit    = time.Now()
		commitEvery   = 2000
		commitMaxWait = 2 * time.Second

		lastAuditTick uint64
		auditSeq      int
	)

	begin := func() {
		if tx != nil {
			return
		}
		txx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			// If we can't start a tx, we can't do much; sleep a bit.
			time.Sleep(50 * time.Millisecond)
			return
		}
		tx = txx
		opCount = 0
		lastCommit = time.Now()
	}
	commit := func() {
		if tx == nil {
			return
		}
		_ = tx.Commit()
		tx = nil
		opCount = 0
		lastCommit = time.Now()
	}
	rollback := func() {
		if tx == nil {
			return
		}
		_ = tx.Rollback()
		tx = nil
		opCount = 0
		lastCommit = time.Now()
	}

	flushIfNeeded := func() {
		if tx == nil {
			return
		}
		if opCount >= commitEvery || time.Since(lastCommit) >= commitMaxWait {
			commit()
		}
	}

nextReq:
	for r := range s.ch {
		begin()
		if tx == nil {
			continue
		}
		switch r.kind {
		case reqTick:
			b, _ := json.Marshal(r.tick)
			if insertTick != nil {
				if _, err := tx.Stmt(insertTick).Exec(
					int64(r.tick.Tick),
					r.tick.Digest,
					len(r.tick.Joins),
					len(r.tick.Leaves),
					len(r.tick.Actions),
					string(b),
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, j := range r.tick.Joins {
				if insertJoin == nil {
					break
				}
				if _, err := tx.Stmt(insertJoin).Exec(int64(r.tick.Tick), j.AgentID, j.Name); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, id := range r.tick.Leaves {
				if insertLeave == nil {
					break
				}
				if _, err := tx.Stmt(insertLeave).Exec(int64(r.tick.Tick), id); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for i, a := range r.tick.Actions {
				if insertAction == nil {
					break
				}
				actJSON, _ := json.Marshal(a.Act)
				if _, err := tx.Stmt(insertAction).Exec(int64(r.tick.Tick), i, a.AgentID, string(actJSON)); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}

		case reqAudit:
			a := r.audit
			if a.Tick != lastAuditTick {
				lastAuditTick = a.Tick
				auditSeq = 0
			}
			seq := auditSeq
			auditSeq++
			raw, _ := json.Marshal(a)
			if insertAudit != nil {
				if _, err := tx.Stmt(insertAudit).Exec(
					int64(a.Tick),
					seq,
					a.Actor,
					a.Action,
					a.Pos[0], a.Pos[1], a.Pos[2],
					int64(a.From),
					int64(a.To),
					a.Reason,
					string(raw),
				); err != nil {
					rollback()
					continue
				}
				opCount++
			}

		case reqSnapshot:
			sn := r.snapshot
			if insertSnapshot != nil {
				if _, err := tx.Stmt(insertSnapshot).Exec(
					int64(sn.Tick),
					sn.Path,
					sn.Seed,
					sn.Height,
					sn.Chunks,
					sn.Agents,
					sn.Claims,
					sn.Containers,
					sn.Contracts,
					sn.Laws,
					sn.Orgs,
				); err != nil {
					rollback()
					continue
				}
				opCount++
			}

		case reqSnapshotState:
			st := r.state
			tick := int64(st.Tick)
			btoi := func(b bool) int {
				if b {
					return 1
				}
				return 0
			}
			if insertSnapWorld != nil {
				if _, err := tx.Stmt(insertSnapWorld).Exec(
					tick,
					st.Weather,
					int64(st.WeatherUntilTick),
					st.ActiveEventID,
					int64(st.ActiveEventStart),
					int64(st.ActiveEventEnds),
					st.ActiveEventCenter[0], st.ActiveEventCenter[1], st.ActiveEventCenter[2],
					st.ActiveEventRadius,
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, a := range st.Agents {
				if insertSnapAgent == nil {
					break
				}
				invJSON, _ := json.Marshal(a.Inventory)
				if _, err := tx.Stmt(insertSnapAgent).Exec(
					tick,
					a.ID,
					a.Name,
					a.OrgID,
					a.Pos[0], a.Pos[1], a.Pos[2],
					a.Yaw,
					a.HP,
					a.Hunger,
					a.StaminaMilli,
					a.RepTrade, a.RepBuild, a.RepSocial, a.RepLaw,
					a.FunNovelty, a.FunCreation, a.FunSocial, a.FunInfluence, a.FunNarrative, a.FunRiskRescue,
					string(invJSON),
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, c := range st.Claims {
				if insertSnapClaim == nil {
					break
				}
				membersJSON, _ := json.Marshal(c.Members)
				if _, err := tx.Stmt(insertSnapClaim).Exec(
					tick,
					c.LandID,
					c.Owner,
					c.Anchor[0], c.Anchor[1], c.Anchor[2],
					c.Radius,
					btoi(c.Flags.AllowBuild), btoi(c.Flags.AllowBreak), btoi(c.Flags.AllowDamage), btoi(c.Flags.AllowTrade),
					c.MarketTax,
					btoi(c.CurfewEnabled), c.CurfewStart, c.CurfewEnd,
					btoi(c.FineBreakEnabled), c.FineBreakItem, c.FineBreakPerBlock,
					btoi(c.AccessPassEnabled), c.AccessTicketItem, c.AccessTicketCost,
					int64(c.MaintenanceDueTick), c.MaintenanceStage,
					string(membersJSON),
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, o := range st.Orgs {
				if insertSnapOrg == nil {
					break
				}
				membersJSON, _ := json.Marshal(o.Members)
				treasury := o.Treasury
				if len(treasury) == 0 && len(o.TreasuryByWorld) > 0 {
					if tw := o.TreasuryByWorld[st.WorldID]; len(tw) > 0 {
						treasury = tw
					}
				}
				treasuryJSON, _ := json.Marshal(treasury)
				if _, err := tx.Stmt(insertSnapOrg).Exec(
					tick,
					o.OrgID,
					o.Kind,
					o.Name,
					int64(o.CreatedTick),
					string(membersJSON),
					string(treasuryJSON),
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, c := range st.Contracts {
				if insertSnapContract == nil {
					break
				}
				reqJSON, _ := json.Marshal(c.Requirements)
				rewJSON, _ := json.Marshal(c.Reward)
				depJSON, _ := json.Marshal(c.Deposit)
				if _, err := tx.Stmt(insertSnapContract).Exec(
					tick,
					c.ContractID,
					c.Kind,
					c.State,
					c.Poster,
					c.Acceptor,
					c.TerminalPos[0], c.TerminalPos[1], c.TerminalPos[2],
					int64(c.CreatedTick),
					int64(c.DeadlineTick),
					string(reqJSON),
					string(rewJSON),
					string(depJSON),
					c.BlueprintID,
					c.Anchor[0], c.Anchor[1], c.Anchor[2],
					c.Rotation,
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, l := range st.Laws {
				if insertSnapLaw == nil {
					break
				}
				paramsJSON, _ := json.Marshal(l.Params)
				votesJSON, _ := json.Marshal(l.Votes)
				if _, err := tx.Stmt(insertSnapLaw).Exec(
					tick,
					l.LawID,
					l.LandID,
					l.TemplateID,
					l.Title,
					l.Status,
					l.ProposedBy,
					int64(l.ProposedTick),
					int64(l.NoticeEndsTick),
					int64(l.VoteEndsTick),
					string(paramsJSON),
					string(votesJSON),
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, tr := range st.Trades {
				if insertSnapTrade == nil {
					break
				}
				offerJSON, _ := json.Marshal(tr.Offer)
				reqJSON, _ := json.Marshal(tr.Request)
				if _, err := tx.Stmt(insertSnapTrade).Exec(
					tick,
					tr.TradeID,
					tr.From,
					tr.To,
					int64(tr.CreatedTick),
					string(offerJSON),
					string(reqJSON),
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++
			}
			for _, b := range st.Boards {
				if insertSnapBoard == nil {
					break
				}
				kind := "GLOBAL"
				var bx, by, bz any
				if typ, pos, ok := parseContainerXYZ(b.BoardID); ok && typ == "BULLETIN_BOARD" {
					kind = "PHYSICAL"
					bx, by, bz = pos[0], pos[1], pos[2]
				}
				if _, err := tx.Stmt(insertSnapBoard).Exec(
					tick,
					b.BoardID,
					kind,
					bx, by, bz,
					len(b.Posts),
				); err != nil {
					rollback()
					continue nextReq
				}
				opCount++

				for _, p := range b.Posts {
					if insertSnapBoardPost == nil {
						break
					}
					if _, err := tx.Stmt(insertSnapBoardPost).Exec(
						tick,
						b.BoardID,
						p.PostID,
						p.Author,
						p.Title,
						p.Body,
						int64(p.Tick),
					); err != nil {
						rollback()
						continue nextReq
					}
					opCount++
				}
			}

		case reqSeason:
			se := r.season
			if insertSeason != nil {
				if _, err := tx.Stmt(insertSeason).Exec(
					se.Season,
					int64(se.EndTick),
					se.Seed,
					se.Path,
					se.RecordedAt,
				); err != nil {
					rollback()
					continue
				}
				opCount++
			}
		}
		flushIfNeeded()
	}

	commit()
}
