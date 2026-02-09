package indexdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	// audit sequencing (assigned in the writer goroutine)
}

type reqKind int

const (
	reqTick reqKind = iota + 1
	reqAudit
	reqSnapshot
	reqSeason
)

type req struct {
	kind reqKind

	tick     world.TickLogEntry
	audit    world.AuditEntry
	snapshot snapshotRow
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

func (s *SQLiteIndex) WriteTick(entry world.TickLogEntry) error {
	if s == nil || s.closed.Load() {
		return nil
	}
	select {
	case s.ch <- req{kind: reqTick, tick: entry}:
	default:
		// Drop if the indexer falls behind; JSONL logs remain the source of truth.
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

	if _, err := tx.Exec(`INSERT OR REPLACE INTO meta(key,value) VALUES('schema_version','1')`); err != nil {
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

func (s *SQLiteIndex) loop() {
	ctx := context.Background()

	// Prepared statements (on db; executed within tx).
	insertTick, _ := s.db.Prepare(`INSERT OR REPLACE INTO ticks(tick,digest,joins,leaves,actions,raw_json) VALUES(?,?,?,?,?,?)`)
	insertJoin, _ := s.db.Prepare(`INSERT OR REPLACE INTO joins(tick,agent_id,name) VALUES(?,?,?)`)
	insertLeave, _ := s.db.Prepare(`INSERT OR REPLACE INTO leaves(tick,agent_id) VALUES(?,?)`)
	insertAction, _ := s.db.Prepare(`INSERT OR REPLACE INTO actions(tick,seq,agent_id,act_json) VALUES(?,?,?,?)`)
	insertAudit, _ := s.db.Prepare(`INSERT OR REPLACE INTO audits(tick,seq,actor,action,x,y,z,from_block,to_block,reason,raw_json) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	insertSnapshot, _ := s.db.Prepare(`INSERT OR REPLACE INTO snapshots(tick,path,seed,height,chunks,agents,claims,containers,contracts,laws,orgs) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
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
					continue
				}
				opCount++
			}
			for _, j := range r.tick.Joins {
				if insertJoin == nil {
					break
				}
				if _, err := tx.Stmt(insertJoin).Exec(int64(r.tick.Tick), j.AgentID, j.Name); err != nil {
					rollback()
					break
				}
				opCount++
			}
			for _, id := range r.tick.Leaves {
				if insertLeave == nil {
					break
				}
				if _, err := tx.Stmt(insertLeave).Exec(int64(r.tick.Tick), id); err != nil {
					rollback()
					break
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
					break
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
