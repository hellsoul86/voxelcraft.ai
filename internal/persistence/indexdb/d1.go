package indexdb

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tuning"
	"voxelcraft.ai/internal/sim/world"
)

type D1Config struct {
	Endpoint      string
	Token         string
	WorldID       string
	BatchSize     int
	FlushInterval time.Duration
	HTTPTimeout   time.Duration
	Logger        *log.Logger
}

type D1Index struct {
	cfg        D1Config
	httpClient *http.Client

	ch   chan d1Event
	wg   sync.WaitGroup
	once sync.Once

	closed atomic.Bool

	auditMu       sync.Mutex
	lastAuditTick uint64
	auditSeq      int
}

type d1Event struct {
	Kind    string `json:"kind"`
	WorldID string `json:"world_id"`
	Payload any    `json:"payload"`
}

type d1TickPayload struct {
	Tick    uint64               `json:"tick"`
	Digest  string               `json:"digest"`
	Joins   []world.RecordedJoin `json:"joins,omitempty"`
	Leaves  []string             `json:"leaves,omitempty"`
	Actions []world.RecordedAction `json:"actions,omitempty"`
}

type d1AuditPayload struct {
	Tick   uint64         `json:"tick"`
	Seq    int            `json:"seq"`
	Actor  string         `json:"actor"`
	Action string         `json:"action"`
	Pos    [3]int         `json:"pos"`
	From   uint16         `json:"from"`
	To     uint16         `json:"to"`
	Reason string         `json:"reason,omitempty"`
	Raw    world.AuditEntry `json:"raw"`
}

type d1SnapshotPayload struct {
	Tick       uint64 `json:"tick"`
	Path       string `json:"path"`
	Seed       int64  `json:"seed"`
	Height     int    `json:"height"`
	Chunks     int    `json:"chunks"`
	Agents     int    `json:"agents"`
	Claims     int    `json:"claims"`
	Containers int    `json:"containers"`
	Contracts  int    `json:"contracts"`
	Laws       int    `json:"laws"`
	Orgs       int    `json:"orgs"`
}

type d1SnapshotStatePayload struct {
	Tick              uint64             `json:"tick"`
	WorldID           string             `json:"world_id,omitempty"`
	Weather           string             `json:"weather"`
	WeatherUntilTick  uint64             `json:"weather_until_tick"`
	ActiveEventID     string             `json:"active_event_id"`
	ActiveEventStart  uint64             `json:"active_event_start_tick"`
	ActiveEventEnds   uint64             `json:"active_event_ends_tick"`
	ActiveEventCenter [3]int             `json:"active_event_center"`
	ActiveEventRadius int                `json:"active_event_radius"`
	Trades            []snapshot.TradeV1 `json:"trades,omitempty"`
	Boards            []snapshot.BoardV1 `json:"boards,omitempty"`
	Agents            []snapshot.AgentV1 `json:"agents,omitempty"`
}

type d1SeasonPayload struct {
	Season     int    `json:"season"`
	EndTick    uint64 `json:"end_tick"`
	Path       string `json:"path"`
	Seed       int64  `json:"seed"`
	RecordedAt string `json:"recorded_at"`
}

type d1CatalogPayload struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	JSON      string `json:"json"`
	UpdatedAt string `json:"updated_at"`
}

func OpenD1(cfg D1Config) (*D1Index, error) {
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.WorldID = strings.TrimSpace(cfg.WorldID)
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("empty d1 ingest endpoint")
	}
	if cfg.WorldID == "" {
		return nil, fmt.Errorf("empty world id")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 128
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 500 * time.Millisecond
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 10 * time.Second
	}

	d := &D1Index{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
		ch: make(chan d1Event, 32768),
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.loop()
	}()

	return d, nil
}

func (d *D1Index) Close() error {
	if d == nil {
		return nil
	}
	d.once.Do(func() {
		d.closed.Store(true)
		close(d.ch)
		d.wg.Wait()
	})
	return nil
}

func (d *D1Index) WriteTick(entry world.TickLogEntry) error {
	if d == nil || d.closed.Load() {
		return nil
	}
	p := d1TickPayload{
		Tick:    entry.Tick,
		Digest:  entry.Digest,
		Joins:   entry.Joins,
		Leaves:  entry.Leaves,
		Actions: entry.Actions,
	}
	d.enqueue(d1Event{Kind: "tick", WorldID: d.cfg.WorldID, Payload: p})
	return nil
}

func (d *D1Index) WriteAudit(entry world.AuditEntry) error {
	if d == nil || d.closed.Load() {
		return nil
	}
	seq := d.nextAuditSeq(entry.Tick)
	p := d1AuditPayload{
		Tick:   entry.Tick,
		Seq:    seq,
		Actor:  entry.Actor,
		Action: entry.Action,
		Pos:    entry.Pos,
		From:   entry.From,
		To:     entry.To,
		Reason: entry.Reason,
		Raw:    entry,
	}
	d.enqueue(d1Event{Kind: "audit", WorldID: d.cfg.WorldID, Payload: p})
	return nil
}

func (d *D1Index) RecordSnapshot(path string, snap snapshot.SnapshotV1) {
	if d == nil || d.closed.Load() {
		return
	}
	p := d1SnapshotPayload{
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
	d.enqueue(d1Event{Kind: "snapshot", WorldID: d.cfg.WorldID, Payload: p})
}

func (d *D1Index) RecordSnapshotState(snap snapshot.SnapshotV1) {
	if d == nil || d.closed.Load() {
		return
	}
	p := d1SnapshotStatePayload{
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
	}
	d.enqueue(d1Event{Kind: "snapshot_state", WorldID: d.cfg.WorldID, Payload: p})
}

func (d *D1Index) RecordSeason(season int, endTick uint64, archivedSnapshotPath string, seed int64) {
	if d == nil || d.closed.Load() {
		return
	}
	if season <= 0 || strings.TrimSpace(archivedSnapshotPath) == "" {
		return
	}
	p := d1SeasonPayload{
		Season:     season,
		EndTick:    endTick,
		Path:       archivedSnapshotPath,
		Seed:       seed,
		RecordedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	d.enqueue(d1Event{Kind: "season", WorldID: d.cfg.WorldID, Payload: p})
}

func (d *D1Index) UpsertCatalogs(configDir string, cats *catalogs.Catalogs, tune tuning.Tuning) error {
	if d == nil || d.closed.Load() || cats == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

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

	type row struct {
		name   string
		digest string
		data   []byte
	}
	rows := make([]row, 0, 12)
	if b := raw["blocks_defs"]; len(b) > 0 {
		rows = append(rows, row{name: "blocks_defs", digest: cats.Blocks.DefsDigest, data: b})
	}
	if b, err := json.Marshal(cats.Blocks.Palette); err == nil && len(b) > 0 {
		rows = append(rows, row{name: "blocks_palette", digest: cats.Blocks.PaletteDigest, data: b})
	}
	if b := raw["items_defs"]; len(b) > 0 {
		rows = append(rows, row{name: "items_defs", digest: cats.Items.DefsDigest, data: b})
	}
	if b, err := json.Marshal(cats.Items.Palette); err == nil && len(b) > 0 {
		rows = append(rows, row{name: "items_palette", digest: cats.Items.PaletteDigest, data: b})
	}
	if b := raw["recipes"]; len(b) > 0 {
		rows = append(rows, row{name: "recipes", digest: cats.Recipes.Digest, data: b})
	}
	{
		bps := make([]catalogs.BlueprintDef, 0, len(cats.Blueprints.ByID))
		for _, bp := range cats.Blueprints.ByID {
			bps = append(bps, bp)
		}
		sort.Slice(bps, func(i, j int) bool { return bps[i].ID < bps[j].ID })
		if b, err := json.Marshal(bps); err == nil && len(b) > 0 {
			rows = append(rows, row{name: "blueprints", digest: cats.Blueprints.Digest, data: b})
		}
	}
	if b, err := json.Marshal(cats.Laws); err == nil && len(b) > 0 {
		rows = append(rows, row{name: "law_catalog", digest: cats.Laws.Digest, data: b})
	}
	{
		evs := make([]catalogs.EventTemplate, 0, len(cats.Events.ByID))
		for _, ev := range cats.Events.ByID {
			evs = append(evs, ev)
		}
		sort.Slice(evs, func(i, j int) bool { return evs[i].ID < evs[j].ID })
		if b, err := json.Marshal(evs); err == nil && len(b) > 0 {
			rows = append(rows, row{name: "events", digest: cats.Events.Digest, data: b})
		}
	}
	if b := raw["law_templates"]; len(b) > 0 {
		rows = append(rows, row{name: "law_templates", digest: cats.Laws.Digest, data: b})
	}
	if b, err := json.Marshal(tune); err == nil && len(b) > 0 {
		sum := sha256.Sum256(b)
		rows = append(rows, row{name: "tuning", digest: hex.EncodeToString(sum[:]), data: b})
	}

	for _, r := range rows {
		if r.name == "" || r.digest == "" || len(r.data) == 0 {
			continue
		}
		d.enqueue(d1Event{Kind: "catalog", WorldID: d.cfg.WorldID, Payload: d1CatalogPayload{
			Name:      r.name,
			Digest:    r.digest,
			JSON:      string(r.data),
			UpdatedAt: now,
		}})
	}
	return nil
}

func (d *D1Index) nextAuditSeq(tick uint64) int {
	d.auditMu.Lock()
	defer d.auditMu.Unlock()
	if tick != d.lastAuditTick {
		d.lastAuditTick = tick
		d.auditSeq = 0
	}
	d.auditSeq++
	return d.auditSeq
}

func (d *D1Index) enqueue(ev d1Event) {
	if d == nil || d.closed.Load() {
		return
	}
	select {
	case d.ch <- ev:
	default:
		d.printf("d1 index queue full; drop kind=%s world=%s", ev.Kind, ev.WorldID)
	}
}

func (d *D1Index) loop() {
	ticker := time.NewTicker(d.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]d1Event, 0, d.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := d.sendBatch(batch); err != nil {
			d.printf("d1 index flush failed batch=%d err=%v", len(batch), err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case ev, ok := <-d.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, ev)
			if len(batch) >= d.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (d *D1Index) sendBatch(events []d1Event) error {
	if len(events) == 0 {
		return nil
	}

	body := struct {
		Events []d1Event `json:"events"`
	}{Events: events}
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, d.cfg.Endpoint, bytes.NewReader(buf))
		if err != nil {
			return err
		}
		req.Header.Set("content-type", "application/json")
		if d.cfg.Token != "" {
			req.Header.Set("x-vc-index-token", d.cfg.Token)
		}

		resp, err := d.httpClient.Do(req)
		if err == nil {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			err = fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
		lastErr = err
		time.Sleep(time.Duration(100*(1<<attempt)) * time.Millisecond)
	}
	return lastErr
}

func (d *D1Index) printf(format string, args ...any) {
	if d != nil && d.cfg.Logger != nil {
		d.cfg.Logger.Printf(format, args...)
	}
}
