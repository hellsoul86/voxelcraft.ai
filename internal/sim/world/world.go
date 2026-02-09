package world

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	simenc "voxelcraft.ai/internal/sim/encoding"
	"voxelcraft.ai/internal/sim/tasks"
)

type WorldConfig struct {
	ID         string
	TickRateHz int
	DayTicks   int
	ObsRadius  int
	Height     int
	Seed       int64
	BoundaryR  int
}

type JoinRequest struct {
	Name        string
	DeltaVoxels bool
	Out         chan []byte
	Resp        chan JoinResponse
}

type AttachRequest struct {
	ResumeToken string
	DeltaVoxels bool
	Out         chan []byte
	Resp        chan JoinResponse
}

type JoinResponse struct {
	Welcome  protocol.WelcomeMsg
	Catalogs []protocol.CatalogMsg
}

type ActionEnvelope struct {
	AgentID string
	Act     protocol.ActMsg
}

type RecordedJoin struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
}

// World is a single-threaded authoritative simulation.
// All state must be accessed only from the world loop goroutine.
type World struct {
	cfg      WorldConfig
	catalogs *catalogs.Catalogs

	tick atomic.Uint64

	chunks *ChunkStore

	agents  map[string]*Agent
	clients map[string]*clientState

	claims     map[string]*LandClaim
	containers map[Vec3i]*Container
	trades     map[string]*Trade
	boards     map[string]*Board
	contracts  map[string]*Contract
	laws       map[string]*Law
	orgs       map[string]*Organization

	inbox  chan ActionEnvelope
	join   chan JoinRequest
	attach chan AttachRequest
	leave  chan string
	stop   chan struct{}

	nextAgentNum    atomic.Uint64
	nextTaskNum     atomic.Uint64
	nextLandNum     atomic.Uint64
	nextTradeNum    atomic.Uint64
	nextPostNum     atomic.Uint64
	nextContractNum atomic.Uint64
	nextLawNum      atomic.Uint64
	nextOrgNum      atomic.Uint64

	// Optional loggers (may be nil). Implemented in internal/persistence/*.
	tickLogger  TickLogger
	auditLogger AuditLogger

	// Optional snapshot sink (may be nil). Snapshot writing should be off-thread.
	snapshotSink chan<- snapshot.SnapshotV1

	// World director (MVP): a single active event + simple weather override.
	weather          string
	weatherUntilTick uint64
	activeEventID    string
	activeEventEnds  uint64

	stats *WorldStats

	// Fun-score: track blueprinted structures for delayed awards + usage/influence.
	structures map[string]*Structure
}

type TickLogger interface {
	WriteTick(entry TickLogEntry) error
}

type AuditLogger interface {
	WriteAudit(entry AuditEntry) error
}

type TickLogEntry struct {
	Tick    uint64           `json:"tick"`
	Joins   []RecordedJoin   `json:"joins,omitempty"`
	Leaves  []string         `json:"leaves,omitempty"`
	Actions []RecordedAction `json:"actions,omitempty"`
	Digest  string           `json:"digest"`
}

type RecordedAction struct {
	AgentID string          `json:"agent_id"`
	Act     protocol.ActMsg `json:"act"`
}

type AuditEntry struct {
	Tick   uint64 `json:"tick"`
	Actor  string `json:"actor"`
	Action string `json:"action"` // e.g. "SET_BLOCK"
	Pos    [3]int `json:"pos"`
	From   uint16 `json:"from"`
	To     uint16 `json:"to"`
	Reason string `json:"reason,omitempty"`
}

type clientState struct {
	Out         chan []byte
	DeltaVoxels bool
	LastVoxels  []uint16
}

func New(cfg WorldConfig, cats *catalogs.Catalogs) (*World, error) {
	// Resolve required block ids.
	b := func(id string) (uint16, error) {
		v, ok := cats.Blocks.Index[id]
		if !ok {
			return 0, fmt.Errorf("missing block id in palette: %s", id)
		}
		return v, nil
	}
	air, _ := b("AIR")
	dirt, _ := b("DIRT")
	grass, _ := b("GRASS")
	sand, _ := b("SAND")
	stone, _ := b("STONE")
	water, _ := b("WATER")
	coal, _ := b("COAL_ORE")
	iron, _ := b("IRON_ORE")
	copper, _ := b("COPPER_ORE")
	crystal, _ := b("CRYSTAL_ORE")

	gen := WorldGen{
		Seed:       cfg.Seed,
		Height:     cfg.Height,
		SeaLevel:   20,
		BoundaryR:  cfg.BoundaryR,
		Air:        air,
		Dirt:       dirt,
		Grass:      grass,
		Sand:       sand,
		Stone:      stone,
		Water:      water,
		CoalOre:    coal,
		IronOre:    iron,
		CopperOre:  copper,
		CrystalOre: crystal,
	}

	w := &World{
		cfg:        cfg,
		catalogs:   cats,
		chunks:     NewChunkStore(gen),
		agents:     map[string]*Agent{},
		clients:    map[string]*clientState{},
		claims:     map[string]*LandClaim{},
		containers: map[Vec3i]*Container{},
		trades:     map[string]*Trade{},
		boards:     map[string]*Board{},
		contracts:  map[string]*Contract{},
		laws:       map[string]*Law{},
		orgs:       map[string]*Organization{},
		inbox:      make(chan ActionEnvelope, 1024),
		join:       make(chan JoinRequest, 64),
		attach:     make(chan AttachRequest, 64),
		leave:      make(chan string, 64),
		stop:       make(chan struct{}),
		weather:    "CLEAR",
		stats:      NewWorldStats(300, 72000),
		structures: map[string]*Structure{},
	}
	return w, nil
}

func (w *World) SetTickLogger(l TickLogger)                    { w.tickLogger = l }
func (w *World) SetAuditLogger(l AuditLogger)                  { w.auditLogger = l }
func (w *World) SetSnapshotSink(ch chan<- snapshot.SnapshotV1) { w.snapshotSink = ch }

func (w *World) Inbox() chan<- ActionEnvelope { return w.inbox }
func (w *World) Join() chan<- JoinRequest     { return w.join }
func (w *World) Attach() chan<- AttachRequest { return w.attach }
func (w *World) Leave() chan<- string         { return w.leave }

func (w *World) CurrentTick() uint64 { return w.tick.Load() }

func (w *World) Run(ctx context.Context) error {
	interval := time.Second / time.Duration(w.cfg.TickRateHz)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var pendingActions []ActionEnvelope
	var pendingJoins []JoinRequest
	var pendingLeaves []string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stop:
			return nil
		case req := <-w.join:
			pendingJoins = append(pendingJoins, req)
		case req := <-w.attach:
			w.handleAttach(req)
		case id := <-w.leave:
			pendingLeaves = append(pendingLeaves, id)
		case env := <-w.inbox:
			pendingActions = append(pendingActions, env)
		case <-ticker.C:
			w.step(pendingJoins, pendingLeaves, pendingActions)
			pendingJoins = pendingJoins[:0]
			pendingLeaves = pendingLeaves[:0]
			pendingActions = pendingActions[:0]
		}
	}
}

func (w *World) Stop() { close(w.stop) }

func (w *World) joinAgent(name string, delta bool, out chan []byte) JoinResponse {
	if name == "" {
		name = "agent"
	}

	idNum := w.nextAgentNum.Add(1)
	agentID := fmt.Sprintf("A%d", idNum)

	// Spawn near origin on surface.
	spawnXZ := int(idNum) * 2
	spawnX := spawnXZ
	spawnZ := -spawnXZ
	y := w.surfaceY(spawnX, spawnZ)

	a := &Agent{
		ID:   agentID,
		Name: name,
		Pos:  Vec3i{X: spawnX, Y: y, Z: spawnZ},
		Yaw:  0,
	}
	a.initDefaults()
	// Starter items to make early testing easier.
	a.Inventory["PLANK"] = 20
	a.Inventory["COAL"] = 10
	a.Inventory["STONE"] = 20

	// Fun/novelty: first biome arrival.
	w.funOnBiome(a, w.tick.Load())

	w.agents[agentID] = a
	if out != nil {
		w.clients[agentID] = &clientState{Out: out, DeltaVoxels: delta}
	}

	token := fmt.Sprintf("resume_%s_%d", w.cfg.ID, time.Now().UnixNano())
	a.ResumeToken = token

	welcome := protocol.WelcomeMsg{
		Type:            protocol.TypeWelcome,
		ProtocolVersion: protocol.Version,
		AgentID:         agentID,
		ResumeToken:     token,
		WorldParams: protocol.WorldParams{
			TickRateHz: w.cfg.TickRateHz,
			ChunkSize:  [3]int{16, 16, w.cfg.Height},
			Height:     w.cfg.Height,
			ObsRadius:  w.cfg.ObsRadius,
			DayTicks:   w.cfg.DayTicks,
			Seed:       w.cfg.Seed,
		},
		Catalogs: protocol.CatalogDigests{
			BlockPalette:       protocol.DigestRef{Digest: w.catalogs.Blocks.PaletteDigest, Count: len(w.catalogs.Blocks.Palette)},
			ItemPalette:        protocol.DigestRef{Digest: w.catalogs.Items.PaletteDigest, Count: len(w.catalogs.Items.Palette)},
			RecipesDigest:      w.catalogs.Recipes.Digest,
			BlueprintsDigest:   w.catalogs.Blueprints.Digest,
			LawTemplatesDigest: w.catalogs.Laws.Digest,
			EventsDigest:       w.catalogs.Events.Digest,
		},
	}

	catalogMsgs := []protocol.CatalogMsg{
		{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "block_palette",
			Digest:          w.catalogs.Blocks.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Blocks.Palette,
		},
		{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "item_palette",
			Digest:          w.catalogs.Items.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Items.Palette,
		},
	}

	return JoinResponse{Welcome: welcome, Catalogs: catalogMsgs}
}

func (w *World) handleJoin(req JoinRequest) {
	resp := w.joinAgent(req.Name, req.DeltaVoxels, req.Out)
	if req.Resp != nil {
		req.Resp <- resp
	}
}

func (w *World) handleAttach(req AttachRequest) {
	token := strings.TrimSpace(req.ResumeToken)
	if token == "" || req.Out == nil {
		if req.Resp != nil {
			req.Resp <- JoinResponse{}
		}
		return
	}

	// Find agent deterministically by iterating sorted ids.
	agentIDs := make([]string, 0, len(w.agents))
	for id := range w.agents {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)
	var a *Agent
	for _, id := range agentIDs {
		aa := w.agents[id]
		if aa != nil && aa.ResumeToken == token {
			a = aa
			break
		}
	}
	if a == nil {
		if req.Resp != nil {
			req.Resp <- JoinResponse{}
		}
		return
	}

	// Attach client (does not affect simulation determinism).
	w.clients[a.ID] = &clientState{Out: req.Out, DeltaVoxels: req.DeltaVoxels}

	// Rotate token on successful resume.
	newToken := fmt.Sprintf("resume_%s_%d", w.cfg.ID, time.Now().UnixNano())
	a.ResumeToken = newToken

	welcome := protocol.WelcomeMsg{
		Type:            protocol.TypeWelcome,
		ProtocolVersion: protocol.Version,
		AgentID:         a.ID,
		ResumeToken:     newToken,
		WorldParams: protocol.WorldParams{
			TickRateHz: w.cfg.TickRateHz,
			ChunkSize:  [3]int{16, 16, w.cfg.Height},
			Height:     w.cfg.Height,
			ObsRadius:  w.cfg.ObsRadius,
			DayTicks:   w.cfg.DayTicks,
			Seed:       w.cfg.Seed,
		},
		Catalogs: protocol.CatalogDigests{
			BlockPalette:       protocol.DigestRef{Digest: w.catalogs.Blocks.PaletteDigest, Count: len(w.catalogs.Blocks.Palette)},
			ItemPalette:        protocol.DigestRef{Digest: w.catalogs.Items.PaletteDigest, Count: len(w.catalogs.Items.Palette)},
			RecipesDigest:      w.catalogs.Recipes.Digest,
			BlueprintsDigest:   w.catalogs.Blueprints.Digest,
			LawTemplatesDigest: w.catalogs.Laws.Digest,
			EventsDigest:       w.catalogs.Events.Digest,
		},
	}

	catalogMsgs := []protocol.CatalogMsg{
		{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "block_palette",
			Digest:          w.catalogs.Blocks.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Blocks.Palette,
		},
		{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "item_palette",
			Digest:          w.catalogs.Items.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Items.Palette,
		},
	}

	if req.Resp != nil {
		req.Resp <- JoinResponse{Welcome: welcome, Catalogs: catalogMsgs}
	}
}

func (w *World) handleLeave(agentID string) {
	delete(w.clients, agentID)
}

func (w *World) step(joins []JoinRequest, leaves []string, actions []ActionEnvelope) {
	nowTick := w.tick.Load()

	// Apply leaves and joins deterministically at tick boundary.
	recordedLeaves := make([]string, 0, len(leaves))
	for _, id := range leaves {
		if _, ok := w.agents[id]; ok {
			w.handleLeave(id)
			recordedLeaves = append(recordedLeaves, id)
		}
	}
	recordedJoins := make([]RecordedJoin, 0, len(joins))
	for _, req := range joins {
		resp := w.joinAgent(req.Name, req.DeltaVoxels, req.Out)
		if req.Resp != nil {
			req.Resp <- resp
		}
		recordedJoins = append(recordedJoins, RecordedJoin{AgentID: resp.Welcome.AgentID, Name: req.Name})
	}

	// Maintenance runs at tick boundary before any actions so permissions reflect the current stage.
	w.tickClaimsMaintenance(nowTick)

	// Apply actions in server_receive_order (the inbox order).
	recorded := make([]RecordedAction, 0, len(actions))
	for _, env := range actions {
		a := w.agents[env.AgentID]
		if a == nil {
			continue
		}
		env.Act.AgentID = env.AgentID // trust session identity
		recorded = append(recorded, RecordedAction{AgentID: env.AgentID, Act: env.Act})
		w.applyAct(a, env.Act, nowTick)
	}

	// Systems: movement -> work -> environment (minimal) -> others (stub)
	w.systemMovement(nowTick)
	w.systemWork(nowTick)
	w.systemEnvironment(nowTick)
	w.tickLaws(nowTick)
	w.systemDirector(nowTick)
	w.tickContracts(nowTick)
	w.systemFun(nowTick)
	if w.stats != nil {
		w.stats.ObserveAgents(nowTick, w.agents)
	}

	// Build + send OBS for each agent.
	for id, a := range w.agents {
		cl := w.clients[id]
		if cl == nil {
			continue
		}
		obs := w.buildObs(a, cl, nowTick)
		b, err := json.Marshal(obs)
		if err != nil {
			continue
		}
		sendLatest(cl.Out, b)
	}

	digest := w.stateDigest(nowTick)
	if w.tickLogger != nil {
		_ = w.tickLogger.WriteTick(TickLogEntry{Tick: nowTick, Joins: recordedJoins, Leaves: recordedLeaves, Actions: recorded, Digest: digest})
	}

	// Snapshot every 3000 ticks (~10 minutes at 5Hz), starting after tick 0.
	if w.snapshotSink != nil && nowTick != 0 && nowTick%3000 == 0 {
		snap := w.ExportSnapshot(nowTick)
		select {
		case w.snapshotSink <- snap:
		default:
			// Drop snapshot if sink is backed up.
		}
	}

	w.tick.Add(1)
}

// StepOnce advances the world by a single tick using the same ordering semantics as the server.
// It is primarily intended for deterministic replays/tests.
func (w *World) StepOnce(joins []JoinRequest, leaves []string, actions []ActionEnvelope) (tick uint64, digest string) {
	tick = w.tick.Load()
	w.step(joins, leaves, actions)
	return tick, w.stateDigest(tick)
}

func (w *World) applyAct(a *Agent, act protocol.ActMsg, nowTick uint64) {
	// Staleness check: accept only [now-2, now].
	if act.Tick+2 < nowTick || act.Tick > nowTick {
		a.AddEvent(actionResult(nowTick, "ACT", false, "E_STALE", "act tick out of range"))
		return
	}

	// Cancel first.
	for _, cid := range act.Cancel {
		if a.MoveTask != nil && a.MoveTask.TaskID == cid {
			a.MoveTask = nil
			a.AddEvent(actionResult(nowTick, cid, true, "", "canceled"))
			continue
		}
		if a.WorkTask != nil && a.WorkTask.TaskID == cid {
			a.WorkTask = nil
			a.AddEvent(actionResult(nowTick, cid, true, "", "canceled"))
			continue
		}
		a.AddEvent(actionResult(nowTick, cid, false, "E_INVALID_TARGET", "task not found"))
	}

	// Instants.
	for _, inst := range act.Instants {
		w.applyInstant(a, inst, nowTick)
	}

	// Tasks.
	for _, tr := range act.Tasks {
		w.applyTaskReq(a, tr, nowTick)
	}
}

func (w *World) applyInstant(a *Agent, inst protocol.InstantReq, nowTick uint64) {
	switch inst.Type {
	case "SAY":
		if !a.RateLimitAllow("SAY", nowTick, 50, 5) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many SAY"))
			return
		}
		if inst.Text == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing text"))
			return
		}
		ch := inst.Channel
		if ch == "" {
			ch = "LOCAL"
		}
		w.broadcastChat(nowTick, a, ch, inst.Text)
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "WHISPER":
		if !a.RateLimitAllow("WHISPER", nowTick, 50, 5) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many WHISPER"))
			return
		}
		if inst.To == "" || inst.Text == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing to/text"))
			return
		}
		to := w.agents[inst.To]
		if to == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
			return
		}
		to.AddEvent(protocol.Event{
			"t":       nowTick,
			"type":    "CHAT",
			"from":    a.ID,
			"channel": "WHISPER",
			"text":    inst.Text,
		})
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "SAVE_MEMORY":
		if inst.Key == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing key"))
			return
		}
		// Enforce a very small budget (64KB total).
		if overMemoryBudget(a.Memory, inst.Key, inst.Value, 64*1024) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "memory budget exceeded"))
			return
		}
		a.MemorySave(inst.Key, inst.Value, inst.TTLTicks, nowTick)
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "LOAD_MEMORY":
		kvs := a.MemoryLoad(inst.Prefix, inst.Limit, nowTick)
		a.PendingMemory = kvs
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", fmt.Sprintf("loaded %d keys", len(kvs))))

	case "OFFER_TRADE":
		if !a.RateLimitAllow("OFFER_TRADE", nowTick, 50, 3) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many OFFER_TRADE"))
			return
		}
		if _, perms := w.permissionsFor(a.ID, a.Pos); !perms["can_trade"] {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade not allowed here"))
			return
		}
		if inst.To == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing to"))
			return
		}
		to := w.agents[inst.To]
		if to == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
			return
		}
		offer, err := parseItemPairs(inst.Offer)
		if err != nil || len(offer) == 0 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad offer"))
			return
		}
		req, err := parseItemPairs(inst.Request)
		if err != nil || len(req) == 0 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad request"))
			return
		}

		tradeID := w.newTradeID()
		w.trades[tradeID] = &Trade{
			TradeID:     tradeID,
			From:        a.ID,
			To:          to.ID,
			Offer:       offer,
			Request:     req,
			CreatedTick: nowTick,
		}
		to.AddEvent(protocol.Event{
			"t":        nowTick,
			"type":     "TRADE_OFFER",
			"trade_id": tradeID,
			"from":     a.ID,
			"offer":    encodeItemPairs(offer),
			"request":  encodeItemPairs(req),
		})
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "trade_id": tradeID})

	case "ACCEPT_TRADE":
		if inst.TradeID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing trade_id"))
			return
		}
		tr := w.trades[inst.TradeID]
		if tr == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "trade not found"))
			return
		}
		if tr.To != a.ID {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not your trade"))
			return
		}
		from := w.agents[tr.From]
		if from == nil {
			delete(w.trades, inst.TradeID)
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "trader offline"))
			return
		}
		landFrom, permsFrom := w.permissionsFor(from.ID, from.Pos)
		landTo, permsTo := w.permissionsFor(a.ID, a.Pos)
		if !permsFrom["can_trade"] || !permsTo["can_trade"] {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade not allowed here"))
			return
		}
		if !hasItems(from.Inventory, tr.Offer) || !hasItems(a.Inventory, tr.Request) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
			return
		}
		taxRate := 0.0
		var taxSink map[string]int
		if landFrom != nil && landTo != nil && landFrom.LandID == landTo.LandID && landFrom.MarketTax > 0 {
			taxRate = landFrom.MarketTax
			if landFrom.Owner != "" {
				if owner := w.agents[landFrom.Owner]; owner != nil {
					taxSink = owner.Inventory
				} else if org := w.orgByID(landFrom.Owner); org != nil {
					if org.Treasury == nil {
						org.Treasury = map[string]int{}
					}
					taxSink = org.Treasury
				}
			}
		}
		applyTransferWithTax(from.Inventory, a.Inventory, tr.Offer, taxSink, taxRate)
		applyTransferWithTax(a.Inventory, from.Inventory, tr.Request, taxSink, taxRate)
		delete(w.trades, inst.TradeID)

		// Reputation: successful trade increases trade/social credit.
		w.bumpRepTrade(from.ID, 2)
		w.bumpRepTrade(a.ID, 2)
		w.bumpRepSocial(from.ID, 1)
		w.bumpRepSocial(a.ID, 1)
		if w.stats != nil {
			w.stats.RecordTrade(nowTick)
		}
		w.funOnTrade(from, nowTick)
		w.funOnTrade(a, nowTick)

		from.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": a.ID})
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": from.ID})
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "DECLINE_TRADE":
		if inst.TradeID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing trade_id"))
			return
		}
		tr := w.trades[inst.TradeID]
		if tr == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "trade not found"))
			return
		}
		if tr.To != a.ID {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not your trade"))
			return
		}
		from := w.agents[tr.From]
		delete(w.trades, inst.TradeID)
		if from != nil {
			from.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DECLINED", "trade_id": tr.TradeID, "by": a.ID})
		}
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "declined"))

	case "POST_BOARD":
		if !a.RateLimitAllow("POST_BOARD", nowTick, 600, 1) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many POST_BOARD"))
			return
		}
		if inst.BoardID == "" || inst.Title == "" || inst.Body == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing board_id/title/body"))
			return
		}
		if len(inst.Title) > 80 || len(inst.Body) > 2000 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "post too large"))
			return
		}
		b := w.boards[inst.BoardID]
		if b == nil {
			b = &Board{BoardID: inst.BoardID}
			w.boards[inst.BoardID] = b
		}
		postID := w.newPostID()
		b.Posts = append(b.Posts, BoardPost{
			PostID: postID,
			Author: a.ID,
			Title:  inst.Title,
			Body:   inst.Body,
			Tick:   nowTick,
		})
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "post_id": postID})

	case "CLAIM_OWED":
		// Claim owed items from a terminal container to self.
		if inst.TerminalID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing terminal_id"))
			return
		}
		c := w.getContainerByID(inst.TerminalID)
		if c == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal not found"))
			return
		}
		if Manhattan(a.Pos, c.Pos) > 3 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
			return
		}
		owed := c.claimOwed(a.ID)
		if len(owed) == 0 {
			a.AddEvent(actionResult(nowTick, inst.ID, true, "", "nothing owed"))
			return
		}
		for item, n := range owed {
			if n > 0 {
				a.Inventory[item] += n
			}
		}
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "claimed"))

	case "POST_CONTRACT":
		if inst.TerminalID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing terminal_id"))
			return
		}
		term := w.getContainerByID(inst.TerminalID)
		if term == nil || term.Type != "CONTRACT_TERMINAL" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "contract terminal not found"))
			return
		}
		if Manhattan(a.Pos, term.Pos) > 3 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
			return
		}
		kind := normalizeContractKind(inst.ContractKind)
		if kind == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad contract_kind"))
			return
		}
		req := stacksToMap(inst.Requirements)
		reward := stacksToMap(inst.Reward)
		deposit := stacksToMap(inst.Deposit)
		if len(reward) == 0 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing reward"))
			return
		}
		if kind != "BUILD" && len(req) == 0 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing requirements"))
			return
		}
		var deadline uint64
		if inst.DeadlineTick != 0 {
			deadline = inst.DeadlineTick
		} else {
			dur := inst.DurationTicks
			if dur <= 0 {
				dur = w.cfg.DayTicks
			}
			deadline = nowTick + uint64(dur)
		}

		// Move reward into terminal inventory and reserve it.
		for item, n := range reward {
			if a.Inventory[item] < n {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "insufficient reward items"))
				return
			}
		}
		for item, n := range reward {
			a.Inventory[item] -= n
			term.Inventory[item] += n
			term.reserve(item, n)
		}

		cid := w.newContractID()
		c := &Contract{
			ContractID:   cid,
			TerminalPos:  term.Pos,
			Poster:       a.ID,
			Kind:         kind,
			Requirements: req,
			Reward:       reward,
			Deposit:      deposit,
			CreatedTick:  nowTick,
			DeadlineTick: deadline,
			State:        ContractOpen,
		}
		if kind == "BUILD" {
			if inst.BlueprintID == "" {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing blueprint_id"))
				return
			}
			c.BlueprintID = inst.BlueprintID
			c.Anchor = Vec3i{X: inst.Anchor[0], Y: inst.Anchor[1], Z: inst.Anchor[2]}
			c.Rotation = inst.Rotation
		}
		w.contracts[cid] = c
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "contract_id": cid})

	case "ACCEPT_CONTRACT":
		if inst.ContractID == "" || inst.TerminalID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing contract_id/terminal_id"))
			return
		}
		c := w.contracts[inst.ContractID]
		if c == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "contract not found"))
			return
		}
		if c.State != ContractOpen {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "contract not open"))
			return
		}
		term := w.getContainerByID(inst.TerminalID)
		if term == nil || term.Type != "CONTRACT_TERMINAL" || term.Pos != c.TerminalPos {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal mismatch"))
			return
		}
		if Manhattan(a.Pos, term.Pos) > 3 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
			return
		}
		if nowTick > c.DeadlineTick {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "contract expired"))
			return
		}
		// Take deposit from acceptor into terminal and reserve.
		// MVP: low trade rep requires higher deposit multiplier.
		reqDep := c.Deposit
		if len(c.Deposit) > 0 {
			m := w.repDepositMultiplier(a)
			if m > 1 {
				reqDep = map[string]int{}
				for item, n := range c.Deposit {
					reqDep[item] = n * m
				}
			}
		}
		for item, n := range reqDep {
			if a.Inventory[item] < n {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "insufficient deposit"))
				return
			}
		}
		for item, n := range reqDep {
			a.Inventory[item] -= n
			term.Inventory[item] += n
			term.reserve(item, n)
		}
		c.Deposit = reqDep
		c.Acceptor = a.ID
		c.State = ContractAccepted
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "accepted"))

	case "SUBMIT_CONTRACT":
		if inst.ContractID == "" || inst.TerminalID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing contract_id/terminal_id"))
			return
		}
		c := w.contracts[inst.ContractID]
		if c == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "contract not found"))
			return
		}
		if c.State != ContractAccepted || c.Acceptor != a.ID {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not acceptor"))
			return
		}
		term := w.getContainerByID(inst.TerminalID)
		if term == nil || term.Type != "CONTRACT_TERMINAL" || term.Pos != c.TerminalPos {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal mismatch"))
			return
		}
		if Manhattan(a.Pos, term.Pos) > 3 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
			return
		}
		if nowTick > c.DeadlineTick {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "contract expired"))
			return
		}

		ok := false
		switch c.Kind {
		case "GATHER", "DELIVER":
			ok = hasAvailable(term, c.Requirements)
		case "BUILD":
			ok = w.checkBlueprintPlaced(c.BlueprintID, c.Anchor, c.Rotation)
		}
		if !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "requirements not met"))
			return
		}

		// Settle immediately (consume requirements if applicable, then pay out).
		if c.Kind == "GATHER" || c.Kind == "DELIVER" {
			for item, n := range c.Requirements {
				term.Inventory[item] -= n
				if term.Inventory[item] <= 0 {
					delete(term.Inventory, item)
				}
				term.addOwed(c.Poster, item, n)
			}
		}
		for item, n := range c.Reward {
			term.unreserve(item, n)
			term.Inventory[item] -= n
			if term.Inventory[item] <= 0 {
				delete(term.Inventory, item)
			}
			a.Inventory[item] += n
		}
		for item, n := range c.Deposit {
			term.unreserve(item, n)
			term.Inventory[item] -= n
			if term.Inventory[item] <= 0 {
				delete(term.Inventory, item)
			}
			a.Inventory[item] += n
		}
		c.State = ContractCompleted
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "completed"))

	case "SET_PERMISSIONS":
		if inst.LandID == "" || inst.Policy == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/policy"))
			return
		}
		land := w.claims[inst.LandID]
		if land == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
			return
		}
		if !w.isLandAdmin(a.ID, land) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
			return
		}
		if v, ok := inst.Policy["allow_build"]; ok {
			land.Flags.AllowBuild = v
		}
		if v, ok := inst.Policy["allow_break"]; ok {
			land.Flags.AllowBreak = v
		}
		if v, ok := inst.Policy["allow_damage"]; ok {
			land.Flags.AllowDamage = v
		}
		if v, ok := inst.Policy["allow_trade"]; ok {
			land.Flags.AllowTrade = v
		}
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "ADD_MEMBER":
		if inst.LandID == "" || inst.MemberID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/member_id"))
			return
		}
		land := w.claims[inst.LandID]
		if land == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
			return
		}
		if !w.isLandAdmin(a.ID, land) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
			return
		}
		if land.Members == nil {
			land.Members = map[string]bool{}
		}
		land.Members[inst.MemberID] = true
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "REMOVE_MEMBER":
		if inst.LandID == "" || inst.MemberID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/member_id"))
			return
		}
		land := w.claims[inst.LandID]
		if land == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
			return
		}
		if !w.isLandAdmin(a.ID, land) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
			return
		}
		if land.Members != nil {
			delete(land.Members, inst.MemberID)
		}
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "CREATE_ORG":
		kind := strings.ToUpper(strings.TrimSpace(inst.OrgKind))
		var k OrgKind
		switch kind {
		case string(OrgGuild):
			k = OrgGuild
		case string(OrgCity):
			k = OrgCity
		default:
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_kind"))
			return
		}
		name := strings.TrimSpace(inst.OrgName)
		if name == "" || len(name) > 40 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_name"))
			return
		}
		if a.OrgID != "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
			return
		}
		orgID := w.newOrgID()
		w.orgs[orgID] = &Organization{
			OrgID:       orgID,
			Kind:        k,
			Name:        name,
			CreatedTick: nowTick,
			Members:     map[string]OrgRole{a.ID: OrgLeader},
			Treasury:    map[string]int{},
		}
		a.OrgID = orgID
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "org_id": orgID})

	case "JOIN_ORG":
		if inst.OrgID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id"))
			return
		}
		org := w.orgByID(inst.OrgID)
		if org == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
			return
		}
		if a.OrgID != "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
			return
		}
		if org.Members == nil {
			org.Members = map[string]OrgRole{}
		}
		org.Members[a.ID] = OrgMember
		a.OrgID = org.OrgID
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "ORG_DEPOSIT":
		if inst.OrgID == "" || inst.ItemID == "" || inst.Count <= 0 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id/item_id/count"))
			return
		}
		org := w.orgByID(inst.OrgID)
		if org == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
			return
		}
		if !w.isOrgMember(a.ID, org.OrgID) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org member"))
			return
		}
		if a.Inventory[inst.ItemID] < inst.Count {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
			return
		}
		a.Inventory[inst.ItemID] -= inst.Count
		if a.Inventory[inst.ItemID] <= 0 {
			delete(a.Inventory, inst.ItemID)
		}
		if org.Treasury == nil {
			org.Treasury = map[string]int{}
		}
		org.Treasury[inst.ItemID] += inst.Count
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "ORG_WITHDRAW":
		if inst.OrgID == "" || inst.ItemID == "" || inst.Count <= 0 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id/item_id/count"))
			return
		}
		org := w.orgByID(inst.OrgID)
		if org == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
			return
		}
		if !w.isOrgAdmin(a.ID, org.OrgID) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org admin"))
			return
		}
		if org.Treasury[inst.ItemID] < inst.Count {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "treasury lacks items"))
			return
		}
		org.Treasury[inst.ItemID] -= inst.Count
		if org.Treasury[inst.ItemID] <= 0 {
			delete(org.Treasury, inst.ItemID)
		}
		a.Inventory[inst.ItemID] += inst.Count
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "LEAVE_ORG":
		if a.OrgID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "not in org"))
			return
		}
		org := w.orgByID(a.OrgID)
		orgID := a.OrgID
		a.OrgID = ""
		if org == nil || org.Members == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
			return
		}
		role := org.Members[a.ID]
		delete(org.Members, a.ID)
		if len(org.Members) == 0 {
			delete(w.orgs, orgID)
			a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
			return
		}
		// If leader left, promote lexicographically smallest remaining member.
		if role == OrgLeader {
			best := ""
			for aid := range org.Members {
				if best == "" || aid < best {
					best = aid
				}
			}
			if best != "" {
				org.Members[best] = OrgLeader
			}
		}
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "DEED_LAND":
		if inst.LandID == "" || inst.NewOwner == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/new_owner"))
			return
		}
		land := w.claims[inst.LandID]
		if land == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
			return
		}
		if !w.isLandAdmin(a.ID, land) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
			return
		}
		newOwner := strings.TrimSpace(inst.NewOwner)
		if newOwner == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad new_owner"))
			return
		}
		if w.agents[newOwner] == nil && w.orgByID(newOwner) == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "new owner not found"))
			return
		}
		land.Owner = newOwner
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	case "PROPOSE_LAW":
		if inst.LandID == "" || inst.TemplateID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/template_id"))
			return
		}
		land := w.claims[inst.LandID]
		if land == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
			return
		}
		// MVP: land admin/member can propose laws.
		if !w.isLandMember(a.ID, land) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible"))
			return
		}
		if _, ok := w.catalogs.Laws.ByID[inst.TemplateID]; !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown law template"))
			return
		}
		if inst.Params == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing params"))
			return
		}

		params := map[string]string{}
		switch inst.TemplateID {
		case "MARKET_TAX":
			f, err := paramFloat(inst.Params, "market_tax")
			if err != nil {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
				return
			}
			if f < 0 {
				f = 0
			}
			if f > 0.25 {
				f = 0.25
			}
			params["market_tax"] = floatToCanonString(f)
		case "CURFEW_NO_BUILD":
			s, err := paramFloat(inst.Params, "start_time")
			if err != nil {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
				return
			}
			en, err := paramFloat(inst.Params, "end_time")
			if err != nil {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
				return
			}
			if s < 0 {
				s = 0
			}
			if s > 1 {
				s = 1
			}
			if en < 0 {
				en = 0
			}
			if en > 1 {
				en = 1
			}
			params["start_time"] = floatToCanonString(s)
			params["end_time"] = floatToCanonString(en)
		case "FINE_BREAK_PER_BLOCK":
			item, err := paramString(inst.Params, "fine_item")
			if err != nil {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
				return
			}
			if _, ok := w.catalogs.Items.Defs[item]; !ok {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown fine_item"))
				return
			}
			n, err := paramInt(inst.Params, "fine_per_block")
			if err != nil {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
				return
			}
			if n < 0 {
				n = 0
			}
			if n > 100 {
				n = 100
			}
			params["fine_item"] = item
			params["fine_per_block"] = fmt.Sprintf("%d", n)
		case "ACCESS_PASS_CORE":
			item, err := paramString(inst.Params, "ticket_item")
			if err != nil {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
				return
			}
			if _, ok := w.catalogs.Items.Defs[item]; !ok {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown ticket_item"))
				return
			}
			n, err := paramInt(inst.Params, "ticket_cost")
			if err != nil {
				a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
				return
			}
			if n < 0 {
				n = 0
			}
			if n > 64 {
				n = 64
			}
			params["ticket_item"] = item
			params["ticket_cost"] = fmt.Sprintf("%d", n)
		default:
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unsupported template"))
			return
		}

		tmpl := w.catalogs.Laws.ByID[inst.TemplateID]
		title := strings.TrimSpace(inst.Title)
		if title == "" {
			title = tmpl.Title
		}
		lawID := w.newLawID()
		law := &Law{
			LawID:          lawID,
			LandID:         land.LandID,
			TemplateID:     inst.TemplateID,
			Title:          title,
			Params:         params,
			ProposedBy:     a.ID,
			ProposedTick:   nowTick,
			NoticeEndsTick: nowTick + 3000,
			VoteEndsTick:   nowTick + 6000,
			Status:         LawNotice,
			Votes:          map[string]string{},
		}
		w.laws[lawID] = law
		w.broadcastLawEvent(nowTick, "PROPOSED", law, "")
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "law_id": lawID})

	case "VOTE":
		if inst.LawID == "" || inst.Choice == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing law_id/choice"))
			return
		}
		law := w.laws[inst.LawID]
		if law == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "law not found"))
			return
		}
		if law.Status != LawVoting {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "law not in voting"))
			return
		}
		land := w.claims[law.LandID]
		if land == nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
			return
		}
		// MVP: land owner or member can vote.
		if !w.isLandMember(a.ID, land) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible to vote"))
			return
		}
		choice := strings.ToUpper(strings.TrimSpace(inst.Choice))
		switch choice {
		case "YES", "Y", "1", "TRUE":
			choice = "YES"
		case "NO", "N", "0", "FALSE":
			choice = "NO"
		case "ABSTAIN":
			choice = "ABSTAIN"
		default:
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad choice"))
			return
		}
		if law.Votes == nil {
			law.Votes = map[string]string{}
		}
		law.Votes[a.ID] = choice
		w.funOnVote(a, nowTick)
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))

	default:
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown instant type"))
	}
}

func (w *World) applyTaskReq(a *Agent, tr protocol.TaskReq, nowTick uint64) {
	switch tr.Type {
	case "STOP":
		a.MoveTask = nil
		a.AddEvent(actionResult(nowTick, tr.ID, true, "", "stopped"))
		return

	case "MOVE_TO":
		if a.MoveTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
			return
		}
		taskID := w.newTaskID()
		a.MoveTask = &tasks.MovementTask{
			TaskID:      taskID,
			Kind:        tasks.KindMoveTo,
			Target:      tasks.Vec3i{X: tr.Target[0], Y: tr.Target[1], Z: tr.Target[2]},
			Tolerance:   tr.Tolerance,
			StartPos:    tasks.Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			StartedTick: nowTick,
		}
		a.AddEvent(protocol.Event{
			"t":       nowTick,
			"type":    "ACTION_RESULT",
			"ref":     tr.ID,
			"ok":      true,
			"task_id": taskID,
		})

	case "MINE":
		if a.WorkTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
			return
		}
		taskID := w.newTaskID()
		a.WorkTask = &tasks.WorkTask{
			TaskID:      taskID,
			Kind:        tasks.KindMine,
			BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: tr.BlockPos[1], Z: tr.BlockPos[2]},
			StartedTick: nowTick,
			WorkTicks:   0,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})

	case "PLACE":
		if a.WorkTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
			return
		}
		if tr.ItemID == "" {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id"))
			return
		}
		taskID := w.newTaskID()
		a.WorkTask = &tasks.WorkTask{
			TaskID:      taskID,
			Kind:        tasks.KindPlace,
			ItemID:      tr.ItemID,
			BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: tr.BlockPos[1], Z: tr.BlockPos[2]},
			StartedTick: nowTick,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})

	case "OPEN":
		if a.WorkTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
			return
		}
		if tr.TargetID == "" {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
			return
		}
		taskID := w.newTaskID()
		a.WorkTask = &tasks.WorkTask{
			TaskID:      taskID,
			Kind:        tasks.KindOpen,
			TargetID:    tr.TargetID,
			StartedTick: nowTick,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})

	case "TRANSFER":
		if a.WorkTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
			return
		}
		if tr.Src == "" || tr.Dst == "" || tr.ItemID == "" || tr.Count <= 0 {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing src/dst/item_id/count"))
			return
		}
		taskID := w.newTaskID()
		a.WorkTask = &tasks.WorkTask{
			TaskID:       taskID,
			Kind:         tasks.KindTransfer,
			SrcContainer: tr.Src,
			DstContainer: tr.Dst,
			ItemID:       tr.ItemID,
			Count:        tr.Count,
			StartedTick:  nowTick,
			WorkTicks:    0,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})

	case "CRAFT":
		if a.WorkTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
			return
		}
		if tr.RecipeID == "" || tr.Count <= 0 {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing recipe_id/count"))
			return
		}
		if _, ok := w.catalogs.Recipes.ByID[tr.RecipeID]; !ok {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown recipe"))
			return
		}
		taskID := w.newTaskID()
		a.WorkTask = &tasks.WorkTask{
			TaskID:      taskID,
			Kind:        tasks.KindCraft,
			RecipeID:    tr.RecipeID,
			Count:       tr.Count,
			StartedTick: nowTick,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})

	case "SMELT":
		if a.WorkTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
			return
		}
		if tr.ItemID == "" || tr.Count <= 0 {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id/count"))
			return
		}
		taskID := w.newTaskID()
		a.WorkTask = &tasks.WorkTask{
			TaskID:      taskID,
			Kind:        tasks.KindSmelt,
			ItemID:      tr.ItemID,
			Count:       tr.Count,
			StartedTick: nowTick,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})

	case "CLAIM_LAND":
		r := tr.Radius
		if r <= 0 {
			r = 32
		}
		if r > 128 {
			r = 128
		}
		anchor := Vec3i{X: tr.Anchor[0], Y: tr.Anchor[1], Z: tr.Anchor[2]}
		// Must be allowed to build at anchor (unclaimed or owned land with build permission).
		if !w.canBuildAt(a.ID, anchor, nowTick) {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "cannot claim here"))
			return
		}
		// Must have resources: 1 battery + 1 crystal shard (MVP).
		if a.Inventory["BATTERY"] < 1 || a.Inventory["CRYSTAL_SHARD"] < 1 {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_RESOURCE", "need BATTERY + CRYSTAL_SHARD"))
			return
		}
		// Must not overlap existing claims.
		for _, c := range w.claims {
			// Conservative overlap check: if anchors are close enough, treat as overlap.
			if abs(anchor.X-c.Anchor.X) <= r+c.Radius && abs(anchor.Z-c.Anchor.Z) <= r+c.Radius {
				a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "claim overlaps existing land"))
				return
			}
		}
		// Place Claim Totem block at anchor.
		if w.chunks.GetBlock(anchor) != w.chunks.gen.Air {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BLOCKED", "anchor occupied"))
			return
		}
		totemID, ok := w.catalogs.Blocks.Index["CLAIM_TOTEM"]
		if !ok {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INTERNAL", "missing CLAIM_TOTEM block"))
			return
		}

		// Consume cost.
		a.Inventory["BATTERY"]--
		a.Inventory["CRYSTAL_SHARD"]--

		w.chunks.SetBlock(anchor, totemID)
		w.auditSetBlock(nowTick, a.ID, anchor, w.chunks.gen.Air, totemID, "CLAIM_LAND")

		landID := w.newLandID(a.ID)
		due := uint64(0)
		if w.cfg.DayTicks > 0 {
			due = nowTick + uint64(w.cfg.DayTicks)
		}
		w.claims[landID] = &LandClaim{
			LandID:             landID,
			Owner:              a.ID,
			Anchor:             anchor,
			Radius:             r,
			Flags:              ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
			MaintenanceDueTick: due,
			MaintenanceStage:   0,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "land_id": landID})

	case "BUILD_BLUEPRINT":
		if a.WorkTask != nil {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
			return
		}
		if tr.BlueprintID == "" {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing blueprint_id"))
			return
		}
		if _, ok := w.catalogs.Blueprints.ByID[tr.BlueprintID]; !ok {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown blueprint"))
			return
		}
		taskID := w.newTaskID()
		a.WorkTask = &tasks.WorkTask{
			TaskID:      taskID,
			Kind:        tasks.KindBuildBlueprint,
			BlueprintID: tr.BlueprintID,
			Anchor:      tasks.Vec3i{X: tr.Anchor[0], Y: tr.Anchor[1], Z: tr.Anchor[2]},
			Rotation:    tr.Rotation,
			BuildIndex:  0,
			StartedTick: nowTick,
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})

	default:
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "unknown task type"))
	}
}

func (w *World) systemMovement(nowTick uint64) {
	for _, a := range w.sortedAgents() {
		mt := a.MoveTask
		if mt == nil {
			continue
		}
		if mt.Kind != tasks.KindMoveTo {
			continue
		}
		target := v3FromTask(mt.Target)
		if Manhattan(a.Pos, target) <= 1 {
			a.Pos = Vec3i{X: target.X, Y: w.surfaceY(target.X, target.Z), Z: target.Z}
			w.recordStructureUsage(a.ID, a.Pos, nowTick)
			w.funOnBiome(a, nowTick)
			a.MoveTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": mt.TaskID, "kind": string(mt.Kind)})
			continue
		}

		next := a.Pos
		// Prefer X then Z steps (deterministic).
		if target.X > a.Pos.X {
			next.X++
		} else if target.X < a.Pos.X {
			next.X--
		} else if target.Z > a.Pos.Z {
			next.Z++
		} else if target.Z < a.Pos.Z {
			next.Z--
		}
		next.Y = w.surfaceY(next.X, next.Z)

		// Land core access pass (law): charge ticket on core entry for non-members.
		if toLand := w.landAt(next); toLand != nil && toLand.AccessPassEnabled && toLand.CoreContains(next) && !w.isLandMember(a.ID, toLand) {
			fromLand := w.landAt(a.Pos)
			entering := fromLand == nil || fromLand.LandID != toLand.LandID || !toLand.CoreContains(a.Pos)
			if entering {
				item := strings.TrimSpace(toLand.AccessTicketItem)
				cost := toLand.AccessTicketCost
				if item == "" || cost <= 0 {
					// Misconfigured law: treat as blocked.
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_PERMISSION", "message": "access pass required"})
					continue
				}
				if a.Inventory[item] < cost {
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_RESOURCE", "message": "need access ticket"})
					continue
				}
				a.Inventory[item] -= cost
				// Credit to land owner if present (agent or org treasury); else burn.
				if toLand.Owner != "" {
					if owner := w.agents[toLand.Owner]; owner != nil {
						owner.Inventory[item] += cost
					} else if org := w.orgByID(toLand.Owner); org != nil {
						if org.Treasury == nil {
							org.Treasury = map[string]int{}
						}
						org.Treasury[item] += cost
					}
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "ACCESS_PASS", "land_id": toLand.LandID, "item": item, "count": cost})
			}
		}

		// Basic collision: need head/feet air.
		if w.chunks.GetBlock(Vec3i{X: next.X, Y: next.Y, Z: next.Z}) != w.chunks.gen.Air {
			a.MoveTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_BLOCKED", "message": "blocked"})
			continue
		}
		a.Pos = next
		w.recordStructureUsage(a.ID, a.Pos, nowTick)
		w.funOnBiome(a, nowTick)
	}
}

func (w *World) systemWork(nowTick uint64) {
	for _, a := range w.sortedAgents() {
		wt := a.WorkTask
		if wt == nil {
			continue
		}

		switch wt.Kind {
		case tasks.KindMine:
			w.tickMine(a, wt, nowTick)
		case tasks.KindPlace:
			w.tickPlace(a, wt, nowTick)
		case tasks.KindOpen:
			w.tickOpen(a, wt, nowTick)
		case tasks.KindTransfer:
			w.tickTransfer(a, wt, nowTick)
		case tasks.KindCraft:
			w.tickCraft(a, wt, nowTick)
		case tasks.KindSmelt:
			w.tickSmelt(a, wt, nowTick)
		case tasks.KindBuildBlueprint:
			w.tickBuildBlueprint(a, wt, nowTick)
		}
	}
}

func (w *World) tickMine(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	pos := v3FromTask(wt.BlockPos)
	// Require within 2 blocks (Manhattan).
	if Manhattan(a.Pos, pos) > 2 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "too far"})
		return
	}
	if !w.canBreakAt(a.ID, pos, nowTick) {
		// Optional law: fine visitors for illegal break attempts (permission-denied only; curfew does not fine).
		if land, perms := w.permissionsFor(a.ID, pos); land != nil && !w.isLandMember(a.ID, land) && !perms["can_break"] &&
			land.FineBreakEnabled && land.FineBreakPerBlock > 0 && strings.TrimSpace(land.FineBreakItem) != "" {
			item := strings.TrimSpace(land.FineBreakItem)
			fine := land.FineBreakPerBlock
			pay := fine
			if have := a.Inventory[item]; have < pay {
				pay = have
			}
			if pay > 0 {
				a.Inventory[item] -= pay
				if land.Owner != "" {
					if owner := w.agents[land.Owner]; owner != nil {
						owner.Inventory[item] += pay
					} else if org := w.orgByID(land.Owner); org != nil {
						if org.Treasury == nil {
							org.Treasury = map[string]int{}
						}
						org.Treasury[item] += pay
					}
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "FINE", "land_id": land.LandID, "item": item, "count": pay, "reason": "BREAK_DENIED"})
			}
		}
		a.WorkTask = nil
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "break denied"})
		return
	}
	b := w.chunks.GetBlock(pos)
	if b == w.chunks.gen.Air {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "no block"})
		return
	}
	wt.WorkTicks++
	if wt.WorkTicks < 10 {
		return
	}

	// Break block -> AIR, add a very simplified drop.
	// If the block is a container/terminal, handle inventory safely.
	if blockName := w.blockName(b); blockName != "" {
		switch blockName {
		case "CHEST", "FURNACE", "CONTRACT_TERMINAL":
			c := w.containers[pos]
			if c != nil && len(c.Reserved) > 0 {
				// Prevent breaking terminals with escrow-reserved items.
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "container has reserved items"})
				return
			}
			if c != nil {
				for item, n := range c.Inventory {
					if n > 0 {
						a.Inventory[item] += n
					}
				}
				if c.Owed != nil {
					if owed := c.Owed[a.ID]; owed != nil {
						for item, n := range owed {
							if n > 0 {
								a.Inventory[item] += n
							}
						}
						delete(c.Owed, a.ID)
					}
				}
				w.removeContainer(pos)
			}
		}
	}

	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	w.auditSetBlock(nowTick, a.ID, pos, b, w.chunks.gen.Air, "MINE")

	item := w.blockIDToItem(b)
	if item != "" {
		a.Inventory[item]++
	}
	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickPlace(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	pos := v3FromTask(wt.BlockPos)
	if !w.canBuildAt(a.ID, pos, nowTick) {
		a.WorkTask = nil
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "build denied"})
		return
	}
	if w.chunks.GetBlock(pos) != w.chunks.gen.Air {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "space occupied"})
		return
	}
	if wt.ItemID == "" || a.Inventory[wt.ItemID] < 1 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing item"})
		return
	}

	blockName := wt.ItemID
	if def, ok := w.catalogs.Items.Defs[wt.ItemID]; ok && def.PlaceAs != "" {
		blockName = def.PlaceAs
	}
	bid, ok := w.catalogs.Blocks.Index[blockName]
	if !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "item not placeable"})
		return
	}

	a.Inventory[wt.ItemID]--
	w.chunks.SetBlock(pos, bid)
	w.auditSetBlock(nowTick, a.ID, pos, w.chunks.gen.Air, bid, "PLACE")
	w.ensureContainerForPlacedBlock(pos, blockName)

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickOpen(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	c := w.getContainerByID(wt.TargetID)
	if c == nil {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "container not found"})
		return
	}
	if Manhattan(a.Pos, c.Pos) > 3 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "too far"})
		return
	}

	ev := protocol.Event{
		"t":              nowTick,
		"type":           "CONTAINER",
		"container":      c.ID(),
		"container_type": c.Type,
		"pos":            c.Pos.ToArray(),
		"inventory":      c.InventoryList(),
	}
	// Include owed summary for this agent.
	if c.Owed != nil {
		if owed := c.Owed[a.ID]; owed != nil {
			ev["owed"] = encodeItemPairs(owed)
		}
	}
	// Include contract summaries if it's a terminal.
	if c.Type == "CONTRACT_TERMINAL" {
		ev["contracts"] = w.contractSummariesForTerminal(c.Pos)
	}
	a.AddEvent(ev)

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickTransfer(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	srcID := wt.SrcContainer
	dstID := wt.DstContainer
	item := wt.ItemID
	n := wt.Count

	if srcID == "SELF" && dstID == "SELF" {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BAD_REQUEST", "message": "no-op transfer"})
		return
	}

	var srcC *Container
	var dstC *Container
	if srcID != "SELF" {
		srcC = w.getContainerByID(srcID)
		if srcC == nil {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "src container not found"})
			return
		}
		if Manhattan(a.Pos, srcC.Pos) > 3 {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "too far from src"})
			return
		}
	}
	if dstID != "SELF" {
		dstC = w.getContainerByID(dstID)
		if dstC == nil {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "dst container not found"})
			return
		}
		if Manhattan(a.Pos, dstC.Pos) > 3 {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "too far from dst"})
			return
		}
	}

	// Withdraw permission and escrow protection.
	if srcC != nil {
		if !w.canWithdrawFromContainer(a.ID, srcC.Pos) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "withdraw denied"})
			return
		}
		if srcC.availableCount(item) < n {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "insufficient src items"})
			return
		}
	} else {
		if a.Inventory[item] < n {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "insufficient self items"})
			return
		}
	}

	// Execute transfer.
	if srcC != nil {
		srcC.Inventory[item] -= n
		if srcC.Inventory[item] <= 0 {
			delete(srcC.Inventory, item)
		}
	} else {
		a.Inventory[item] -= n
	}
	if dstC != nil {
		if dstC.Inventory == nil {
			dstC.Inventory = map[string]int{}
		}
		dstC.Inventory[item] += n
	} else {
		a.Inventory[item] += n
	}

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickCraft(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	rec := w.catalogs.Recipes.ByID[wt.RecipeID]
	// Station constraint (MVP): must be within 2 blocks of a crafting bench block.
	switch rec.Station {
	case "HAND":
		// no constraint
	case "CRAFTING_BENCH":
		if !w.nearBlock(a.Pos, "CRAFTING_BENCH", 2) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "need crafting bench nearby"})
			return
		}
	default:
		// Unknown station for CRAFT.
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unsupported station"})
		return
	}

	wt.WorkTicks++
	if wt.WorkTicks < rec.TimeTicks {
		return
	}
	wt.WorkTicks = 0

	// Check + consume inputs.
	for _, in := range rec.Inputs {
		if a.Inventory[in.Item] < in.Count {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing inputs"})
			return
		}
	}
	for _, in := range rec.Inputs {
		a.Inventory[in.Item] -= in.Count
	}
	for _, out := range rec.Outputs {
		a.Inventory[out.Item] += out.Count
	}
	w.funOnRecipe(a, wt.RecipeID, rec.Tier, nowTick)

	wt.Count--
	if wt.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}

func (w *World) tickSmelt(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	// MVP: require furnace nearby for any smelt.
	if !w.nearBlock(a.Pos, "FURNACE", 2) {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "need furnace nearby"})
		return
	}
	wt.WorkTicks++
	if wt.WorkTicks < 10 {
		return
	}
	wt.WorkTicks = 0

	// Very simplified: iron ore -> iron ingot, copper ore -> copper ingot, raw meat -> cooked meat.
	in := wt.ItemID
	if a.Inventory[in] <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing item"})
		return
	}
	if a.Inventory["COAL"] <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing fuel (COAL)"})
		return
	}
	out := ""
	switch in {
	case "IRON_ORE":
		out = "IRON_INGOT"
	case "COPPER_ORE":
		out = "COPPER_INGOT"
	case "RAW_MEAT":
		out = "COOKED_MEAT"
	default:
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unsupported smelt item"})
		return
	}

	a.Inventory[in]--
	a.Inventory["COAL"]--
	a.Inventory[out]++
	wt.Count--
	if wt.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}

func (w *World) tickBuildBlueprint(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	bp := w.catalogs.Blueprints.ByID[wt.BlueprintID]
	anchor := v3FromTask(wt.Anchor)

	// On first tick, validate cost.
	if wt.BuildIndex == 0 && wt.WorkTicks == 0 {
		// Preflight: space + permission check so we don't consume materials and then fail immediately.
		for _, p := range bp.Blocks {
			pos := Vec3i{
				X: anchor.X + p.Pos[0],
				Y: anchor.Y + p.Pos[1],
				Z: anchor.Z + p.Pos[2],
			}
			if !w.canBuildAt(a.ID, pos, nowTick) {
				a.WorkTask = nil
				w.bumpRepLaw(a.ID, -1)
				if w.stats != nil {
					w.stats.RecordDenied(nowTick)
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "build denied"})
				return
			}
			if w.chunks.GetBlock(pos) != w.chunks.gen.Air {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "space occupied"})
				return
			}
		}
		for _, c := range bp.Cost {
			if a.Inventory[c.Item] < c.Count {
				// Try auto-pull from nearby storage (same land, within range) if possible.
				if ok, msg := w.blueprintEnsureMaterials(a, anchor, bp.Cost, nowTick); !ok {
					a.WorkTask = nil
					if msg == "" {
						msg = "missing materials"
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": msg})
					return
				}
				break
			}
		}
		for _, c := range bp.Cost {
			a.Inventory[c.Item] -= c.Count
		}
	}

	// Place up to 2 blocks per tick.
	placed := 0
	for placed < 2 && wt.BuildIndex < len(bp.Blocks) {
		p := bp.Blocks[wt.BuildIndex]
		pos := Vec3i{
			X: anchor.X + p.Pos[0],
			Y: anchor.Y + p.Pos[1],
			Z: anchor.Z + p.Pos[2],
		}
		bid, ok := w.catalogs.Blocks.Index[p.Block]
		if !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INTERNAL", "message": "unknown block in blueprint"})
			return
		}
		if !w.canBuildAt(a.ID, pos, nowTick) {
			a.WorkTask = nil
			w.bumpRepLaw(a.ID, -1)
			if w.stats != nil {
				w.stats.RecordDenied(nowTick)
			}
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "build denied"})
			return
		}
		if w.chunks.GetBlock(pos) != w.chunks.gen.Air {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "space occupied"})
			return
		}
		w.chunks.SetBlock(pos, bid)
		w.auditSetBlock(nowTick, a.ID, pos, w.chunks.gen.Air, bid, "BUILD_BLUEPRINT")
		w.ensureContainerForPlacedBlock(pos, p.Block)

		wt.BuildIndex++
		placed++
	}

	if wt.BuildIndex >= len(bp.Blocks) {
		if w.stats != nil {
			w.stats.RecordBlueprintComplete(nowTick)
		}
		w.registerStructure(nowTick, a.ID, wt.BlueprintID, anchor)
		w.funOnBlueprintComplete(a, nowTick)
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}

func (w *World) systemEnvironment(nowTick uint64) {
	// Soft survival MVP: hunger decreases slowly; stamina recovers slowly.
	if nowTick%200 == 0 { // ~40s at 5Hz
		for _, a := range w.agents {
			if a.Hunger > 0 {
				a.Hunger--
			}
		}
	}
	for _, a := range w.agents {
		if a.StaminaMilli < 1000 {
			a.StaminaMilli += 2
			if a.StaminaMilli > 1000 {
				a.StaminaMilli = 1000
			}
		}
	}
}

func (w *World) buildObs(a *Agent, cl *clientState, nowTick uint64) protocol.ObsMsg {
	// Voxel cube
	center := a.Pos
	r := w.cfg.ObsRadius
	curr := make([]uint16, 0, (2*r+1)*(2*r+1)*(2*r+1))
	for dy := -r; dy <= r; dy++ {
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				b := w.chunks.GetBlock(Vec3i{X: center.X + dx, Y: center.Y + dy, Z: center.Z + dz})
				curr = append(curr, b)
			}
		}
	}

	vox := protocol.VoxelsObs{
		Center:   center.ToArray(),
		Radius:   r,
		Encoding: "RLE",
	}

	if cl.DeltaVoxels && cl.LastVoxels != nil && len(cl.LastVoxels) == len(curr) {
		ops := make([]protocol.VoxelDeltaOp, 0, 64)
		i := 0
		for dy := -r; dy <= r; dy++ {
			for dz := -r; dz <= r; dz++ {
				for dx := -r; dx <= r; dx++ {
					if curr[i] != cl.LastVoxels[i] {
						ops = append(ops, protocol.VoxelDeltaOp{D: [3]int{dx, dy, dz}, B: curr[i]})
					}
					i++
				}
			}
		}
		if len(ops) > 0 && len(ops) < len(curr)/2 {
			vox.Encoding = "DELTA"
			vox.Ops = ops
		} else {
			vox.Data = simenc.EncodeRLE(curr)
		}
	} else {
		vox.Data = simenc.EncodeRLE(curr)
	}
	cl.LastVoxels = curr

	land, perms := w.permissionsFor(a.ID, a.Pos)
	if land != nil && land.CurfewEnabled {
		t := w.timeOfDay(nowTick)
		if inWindow(t, land.CurfewStart, land.CurfewEnd) {
			perms["can_build"] = false
			perms["can_break"] = false
		}
	}

	tasksObs := make([]protocol.TaskObs, 0, 2)
	if a.MoveTask != nil {
		target := v3FromTask(a.MoveTask.Target)
		start := v3FromTask(a.MoveTask.StartPos)
		eta := Manhattan(a.Pos, target)
		tasksObs = append(tasksObs, protocol.TaskObs{
			TaskID:   a.MoveTask.TaskID,
			Kind:     string(a.MoveTask.Kind),
			Progress: taskProgress(start, a.Pos, target),
			Target:   target.ToArray(),
			EtaTicks: eta,
		})
	}
	if a.WorkTask != nil {
		tasksObs = append(tasksObs, protocol.TaskObs{
			TaskID:   a.WorkTask.TaskID,
			Kind:     string(a.WorkTask.Kind),
			Progress: workProgress(a.WorkTask),
		})
	}

	ents := make([]protocol.EntityObs, 0, 16)
	for _, other := range w.agents {
		if other.ID == a.ID {
			continue
		}
		if Manhattan(other.Pos, a.Pos) <= 16 {
			tags := []string{}
			if other.OrgID != "" {
				tags = append(tags, "org:"+other.OrgID)
			}
			ents = append(ents, protocol.EntityObs{
				ID:             other.ID,
				Type:           "AGENT",
				Pos:            other.Pos.ToArray(),
				Tags:           tags,
				ReputationHint: float64(other.RepTrade) / 1000.0,
			})
		}
	}
	for _, c := range w.containers {
		if Manhattan(c.Pos, a.Pos) <= 16 {
			ents = append(ents, protocol.EntityObs{ID: c.ID(), Type: c.Type, Pos: c.Pos.ToArray()})
		}
	}

	// Public boards (global, MVP).
	publicBoards := make([]protocol.BoardObs, 0, len(w.boards))
	if len(w.boards) > 0 {
		boardIDs := make([]string, 0, len(w.boards))
		for id := range w.boards {
			boardIDs = append(boardIDs, id)
		}
		sort.Strings(boardIDs)
		for _, bid := range boardIDs {
			b := w.boards[bid]
			if b == nil || len(b.Posts) == 0 {
				continue
			}
			posts := make([]protocol.BoardPost, 0, 5)
			// Newest first.
			for i := len(b.Posts) - 1; i >= 0 && len(posts) < 5; i-- {
				p := b.Posts[i]
				summary := p.Body
				if len(summary) > 120 {
					summary = summary[:120]
				}
				posts = append(posts, protocol.BoardPost{
					PostID:  p.PostID,
					Author:  p.Author,
					Title:   p.Title,
					Summary: summary,
				})
			}
			publicBoards = append(publicBoards, protocol.BoardObs{BoardID: bid, TopPosts: posts})
		}
	}

	localRules := protocol.LocalRulesObs{Permissions: perms}
	if land != nil {
		localRules.LandID = land.LandID
		localRules.Owner = land.Owner
		if land.Owner == a.ID {
			localRules.Role = "OWNER"
		} else if w.isLandMember(a.ID, land) {
			localRules.Role = "MEMBER"
		} else {
			localRules.Role = "VISITOR"
		}
		localRules.Tax = map[string]float64{"market": land.MarketTax}
		localRules.MaintenanceDueTick = land.MaintenanceDueTick
		localRules.MaintenanceStage = land.MaintenanceStage
	} else {
		localRules.Role = "WILD"
		localRules.Tax = map[string]float64{"market": 0.0}
	}

	obs := protocol.ObsMsg{
		Type:            protocol.TypeObs,
		ProtocolVersion: protocol.Version,
		Tick:            nowTick,
		AgentID:         a.ID,
		World: protocol.WorldObs{
			TimeOfDay:           float64(int(nowTick)%w.cfg.DayTicks) / float64(w.cfg.DayTicks),
			Weather:             w.weather,
			SeasonDay:           int(nowTick/uint64(w.cfg.DayTicks))%7 + 1,
			Biome:               biomeFrom(hash2(w.cfg.Seed, a.Pos.X, a.Pos.Z)),
			ActiveEvent:         w.activeEventID,
			ActiveEventEndsTick: w.activeEventEnds,
		},
		Self: protocol.SelfObs{
			Pos:     a.Pos.ToArray(),
			Yaw:     a.Yaw,
			HP:      a.HP,
			Hunger:  a.Hunger,
			Stamina: float64(a.StaminaMilli) / 1000.0,
			Status:  []string{"NONE"},
			Reputation: protocol.ReputationObs{
				Trade:  float64(a.RepTrade) / 1000.0,
				Build:  float64(a.RepBuild) / 1000.0,
				Social: float64(a.RepSocial) / 1000.0,
				Law:    float64(a.RepLaw) / 1000.0,
			},
		},
		Inventory: a.InventoryList(),
		Equipment: protocol.EquipmentObs{
			MainHand: a.Equipment.MainHand,
			Armor:    []string{a.Equipment.Armor[0], a.Equipment.Armor[1], a.Equipment.Armor[2], a.Equipment.Armor[3]},
		},
		LocalRules:   localRules,
		Voxels:       vox,
		Entities:     ents,
		Events:       a.TakeEvents(),
		Tasks:        tasksObs,
		PublicBoards: publicBoards,
	}

	if a.Fun.Novelty != 0 || a.Fun.Creation != 0 || a.Fun.Social != 0 || a.Fun.Influence != 0 || a.Fun.Narrative != 0 || a.Fun.RiskRescue != 0 {
		obs.FunScore = &protocol.FunScoreObs{
			Novelty:    a.Fun.Novelty,
			Creation:   a.Fun.Creation,
			Social:     a.Fun.Social,
			Influence:  a.Fun.Influence,
			Narrative:  a.Fun.Narrative,
			RiskRescue: a.Fun.RiskRescue,
		}
	}

	if len(a.PendingMemory) > 0 {
		obs.Memory = a.PendingMemory
		a.PendingMemory = nil
	}

	return obs
}

func (w *World) sortedAgents() []*Agent {
	out := make([]*Agent, 0, len(w.agents))
	for _, a := range w.agents {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (w *World) newTaskID() string {
	n := w.nextTaskNum.Add(1)
	return fmt.Sprintf("T%06d", n)
}

func sendLatest(ch chan []byte, b []byte) {
	select {
	case ch <- b:
		return
	default:
	}
	// Drop one.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- b:
	default:
	}
}

func actionResult(tick uint64, ref string, ok bool, code string, message string) protocol.Event {
	e := protocol.Event{
		"t":    tick,
		"type": "ACTION_RESULT",
		"ref":  ref,
		"ok":   ok,
	}
	if code != "" {
		e["code"] = code
	}
	if message != "" {
		e["message"] = message
	}
	return e
}

func (w *World) broadcastChat(tick uint64, from *Agent, channel string, text string) {
	for _, a := range w.agents {
		if channel == "LOCAL" && Manhattan(a.Pos, from.Pos) > 32 {
			continue
		}
		a.AddEvent(protocol.Event{
			"t":       tick,
			"type":    "CHAT",
			"from":    from.ID,
			"channel": channel,
			"text":    text,
		})
	}
}

func (w *World) surfaceY(x, z int) int {
	// Find the topmost solid block in a small vertical scan around expected height.
	// For generated terrain this is cheap; for modified terrain it's "good enough".
	for y := w.cfg.Height - 2; y >= 1; y-- {
		b := w.chunks.GetBlock(Vec3i{X: x, Y: y, Z: z})
		if b != w.chunks.gen.Air && b != w.chunks.gen.Water {
			return y + 1
		}
	}
	return 1
}

func (w *World) nearBlock(pos Vec3i, blockID string, dist int) bool {
	bid, ok := w.catalogs.Blocks.Index[blockID]
	if !ok {
		return false
	}
	for dz := -dist; dz <= dist; dz++ {
		for dx := -dist; dx <= dist; dx++ {
			p := Vec3i{X: pos.X + dx, Y: pos.Y, Z: pos.Z + dz}
			if w.chunks.GetBlock(p) == bid {
				return true
			}
		}
	}
	return false
}

func (w *World) blockIDToItem(b uint16) string {
	if int(b) < 0 || int(b) >= len(w.catalogs.Blocks.Palette) {
		return ""
	}
	blockName := w.catalogs.Blocks.Palette[b]
	// If an item with same id exists, drop that.
	if _, ok := w.catalogs.Items.Defs[blockName]; ok {
		return blockName
	}
	// Special: ore blocks drop the ore item id.
	switch blockName {
	case "COAL_ORE":
		return "COAL"
	case "IRON_ORE":
		return "IRON_ORE"
	case "COPPER_ORE":
		return "COPPER_ORE"
	case "CRYSTAL_ORE":
		return "CRYSTAL_SHARD"
	}
	return ""
}

func (w *World) blockName(b uint16) string {
	if int(b) < 0 || int(b) >= len(w.catalogs.Blocks.Palette) {
		return ""
	}
	return w.catalogs.Blocks.Palette[b]
}

func (w *World) auditSetBlock(tick uint64, actor string, pos Vec3i, from, to uint16, reason string) {
	if w.auditLogger == nil {
		return
	}
	_ = w.auditLogger.WriteAudit(AuditEntry{
		Tick:   tick,
		Actor:  actor,
		Action: "SET_BLOCK",
		Pos:    pos.ToArray(),
		From:   from,
		To:     to,
		Reason: reason,
	})
}

func (w *World) stateDigest(nowTick uint64) string {
	h := sha256.New()
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], nowTick)
	h.Write(tmp[:])
	binary.LittleEndian.PutUint64(tmp[:], uint64(w.cfg.Seed))
	h.Write(tmp[:])
	h.Write([]byte(w.weather))
	binary.LittleEndian.PutUint64(tmp[:], w.weatherUntilTick)
	h.Write(tmp[:])
	h.Write([]byte(w.activeEventID))
	binary.LittleEndian.PutUint64(tmp[:], w.activeEventEnds)
	h.Write(tmp[:])

	// Chunks (sorted keys).
	for _, k := range w.chunks.LoadedChunkKeys() {
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(k.CX)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(k.CZ)))
		h.Write(tmp[:])
		ch := w.chunks.chunks[k]
		d := ch.Digest()
		h.Write(d[:])
	}

	// Claims (sorted).
	landIDs := make([]string, 0, len(w.claims))
	for id := range w.claims {
		landIDs = append(landIDs, id)
	}
	sort.Strings(landIDs)
	for _, id := range landIDs {
		c := w.claims[id]
		h.Write([]byte(id))
		h.Write([]byte(c.Owner))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.Radius))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.Flags.AllowBuild), boolByte(c.Flags.AllowBreak), boolByte(c.Flags.AllowDamage), boolByte(c.Flags.AllowTrade)})
		if len(c.Members) > 0 {
			memberIDs := make([]string, 0, len(c.Members))
			for mid, ok := range c.Members {
				if ok {
					memberIDs = append(memberIDs, mid)
				}
			}
			sort.Strings(memberIDs)
			binary.LittleEndian.PutUint64(tmp[:], uint64(len(memberIDs)))
			h.Write(tmp[:])
			for _, mid := range memberIDs {
				h.Write([]byte(mid))
			}
		} else {
			binary.LittleEndian.PutUint64(tmp[:], 0)
			h.Write(tmp[:])
		}
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(c.MarketTax))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.CurfewEnabled)})
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(c.CurfewStart))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(c.CurfewEnd))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.FineBreakEnabled)})
		h.Write([]byte(c.FineBreakItem))
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.FineBreakPerBlock))
		h.Write(tmp[:])
		h.Write([]byte{boolByte(c.AccessPassEnabled)})
		h.Write([]byte(c.AccessTicketItem))
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.AccessTicketCost))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], c.MaintenanceDueTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.MaintenanceStage))
		h.Write(tmp[:])
	}

	// Laws (sorted).
	if len(w.laws) > 0 {
		lawIDs := make([]string, 0, len(w.laws))
		for id := range w.laws {
			lawIDs = append(lawIDs, id)
		}
		sort.Strings(lawIDs)
		for _, id := range lawIDs {
			l := w.laws[id]
			if l == nil {
				continue
			}
			h.Write([]byte(id))
			h.Write([]byte(l.LandID))
			h.Write([]byte(l.TemplateID))
			h.Write([]byte(l.Title))
			h.Write([]byte(l.ProposedBy))
			h.Write([]byte(string(l.Status)))
			binary.LittleEndian.PutUint64(tmp[:], l.ProposedTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], l.NoticeEndsTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], l.VoteEndsTick)
			h.Write(tmp[:])

			if len(l.Params) > 0 {
				keys := make([]string, 0, len(l.Params))
				for k := range l.Params {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					h.Write([]byte(k))
					h.Write([]byte(l.Params[k]))
				}
			}
			if len(l.Votes) > 0 {
				voters := make([]string, 0, len(l.Votes))
				for aid := range l.Votes {
					voters = append(voters, aid)
				}
				sort.Strings(voters)
				for _, aid := range voters {
					h.Write([]byte(aid))
					h.Write([]byte(l.Votes[aid]))
				}
			}
		}
	}

	// Orgs (sorted).
	if len(w.orgs) > 0 {
		orgIDs := make([]string, 0, len(w.orgs))
		for id := range w.orgs {
			orgIDs = append(orgIDs, id)
		}
		sort.Strings(orgIDs)
		for _, id := range orgIDs {
			o := w.orgs[id]
			if o == nil {
				continue
			}
			h.Write([]byte(id))
			h.Write([]byte(string(o.Kind)))
			h.Write([]byte(o.Name))
			binary.LittleEndian.PutUint64(tmp[:], o.CreatedTick)
			h.Write(tmp[:])
			if len(o.Members) > 0 {
				memberIDs := make([]string, 0, len(o.Members))
				for aid := range o.Members {
					memberIDs = append(memberIDs, aid)
				}
				sort.Strings(memberIDs)
				for _, aid := range memberIDs {
					h.Write([]byte(aid))
					h.Write([]byte(string(o.Members[aid])))
				}
			}
			writeItemMap(h, tmp, o.Treasury)
		}
	}

	// Containers (sorted by pos).
	if len(w.containers) > 0 {
		posKeys := make([]Vec3i, 0, len(w.containers))
		for p := range w.containers {
			posKeys = append(posKeys, p)
		}
		sort.Slice(posKeys, func(i, j int) bool {
			if posKeys[i].X != posKeys[j].X {
				return posKeys[i].X < posKeys[j].X
			}
			if posKeys[i].Y != posKeys[j].Y {
				return posKeys[i].Y < posKeys[j].Y
			}
			return posKeys[i].Z < posKeys[j].Z
		})
		for _, p := range posKeys {
			c := w.containers[p]
			h.Write([]byte(c.Type))
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Pos.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Pos.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Pos.Z)))
			h.Write(tmp[:])
			writeItemMap(h, tmp, c.Inventory)
			writeItemMap(h, tmp, c.Reserved)
			if c.Owed != nil {
				owedAgents := make([]string, 0, len(c.Owed))
				for aid := range c.Owed {
					owedAgents = append(owedAgents, aid)
				}
				sort.Strings(owedAgents)
				for _, aid := range owedAgents {
					h.Write([]byte(aid))
					writeItemMap(h, tmp, c.Owed[aid])
				}
			}
		}
	}

	// Contracts (sorted).
	contractIDs := make([]string, 0, len(w.contracts))
	for id := range w.contracts {
		contractIDs = append(contractIDs, id)
	}
	sort.Strings(contractIDs)
	for _, id := range contractIDs {
		c := w.contracts[id]
		h.Write([]byte(id))
		h.Write([]byte(c.Kind))
		h.Write([]byte(string(c.State)))
		h.Write([]byte(c.Poster))
		h.Write([]byte(c.Acceptor))
		binary.LittleEndian.PutUint64(tmp[:], c.CreatedTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], c.DeadlineTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.TerminalPos.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.TerminalPos.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.TerminalPos.Z)))
		h.Write(tmp[:])
		writeItemMap(h, tmp, c.Requirements)
		writeItemMap(h, tmp, c.Reward)
		writeItemMap(h, tmp, c.Deposit)
		h.Write([]byte(c.BlueprintID))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(c.Anchor.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(c.Rotation))
		h.Write(tmp[:])
	}

	// Trades (sorted).
	tradeIDs := make([]string, 0, len(w.trades))
	for id := range w.trades {
		tradeIDs = append(tradeIDs, id)
	}
	sort.Strings(tradeIDs)
	for _, id := range tradeIDs {
		tr := w.trades[id]
		h.Write([]byte(id))
		h.Write([]byte(tr.From))
		h.Write([]byte(tr.To))
		writeItemMap(h, tmp, tr.Offer)
		writeItemMap(h, tmp, tr.Request)
	}

	// Boards (sorted).
	boardIDs := make([]string, 0, len(w.boards))
	for id := range w.boards {
		boardIDs = append(boardIDs, id)
	}
	sort.Strings(boardIDs)
	for _, id := range boardIDs {
		b := w.boards[id]
		if b == nil {
			continue
		}
		h.Write([]byte(id))
		for _, p := range b.Posts {
			h.Write([]byte(p.PostID))
			h.Write([]byte(p.Author))
			h.Write([]byte(p.Title))
			h.Write([]byte(p.Body))
			binary.LittleEndian.PutUint64(tmp[:], p.Tick)
			h.Write(tmp[:])
		}
	}

	// Structures (sorted).
	structIDs := make([]string, 0, len(w.structures))
	for id := range w.structures {
		structIDs = append(structIDs, id)
	}
	sort.Strings(structIDs)
	binary.LittleEndian.PutUint64(tmp[:], uint64(len(structIDs)))
	h.Write(tmp[:])
	for _, id := range structIDs {
		s := w.structures[id]
		if s == nil {
			continue
		}
		h.Write([]byte(s.StructureID))
		h.Write([]byte(s.BlueprintID))
		h.Write([]byte(s.BuilderID))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Anchor.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Anchor.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Anchor.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Min.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Min.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Min.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Max.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Max.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(s.Max.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], s.CompletedTick)
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], s.AwardDueTick)
		h.Write(tmp[:])
		h.Write([]byte{boolByte(s.Awarded)})
		binary.LittleEndian.PutUint64(tmp[:], uint64(s.LastInfluenceDay))
		h.Write(tmp[:])

		if len(s.UsedBy) > 0 {
			usedIDs := make([]string, 0, len(s.UsedBy))
			for aid := range s.UsedBy {
				usedIDs = append(usedIDs, aid)
			}
			sort.Strings(usedIDs)
			binary.LittleEndian.PutUint64(tmp[:], uint64(len(usedIDs)))
			h.Write(tmp[:])
			for _, aid := range usedIDs {
				h.Write([]byte(aid))
				binary.LittleEndian.PutUint64(tmp[:], s.UsedBy[aid])
				h.Write(tmp[:])
			}
		} else {
			binary.LittleEndian.PutUint64(tmp[:], 0)
			h.Write(tmp[:])
		}
	}

	// Agents (sorted).
	agents := w.sortedAgents()
	for _, a := range agents {
		h.Write([]byte(a.ID))
		h.Write([]byte(a.Name))
		h.Write([]byte(a.OrgID))
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Pos.X)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Pos.Y)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Pos.Z)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(int64(a.Yaw)))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.HP))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Hunger))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.StaminaMilli))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepTrade))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepBuild))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepSocial))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.RepLaw))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Novelty))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Creation))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Social))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Influence))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.Narrative))
		h.Write(tmp[:])
		binary.LittleEndian.PutUint64(tmp[:], uint64(a.Fun.RiskRescue))
		h.Write(tmp[:])

		// Fun novelty memory (seen biome/recipes/events) and anti-exploit state.
		biomes := make([]string, 0, len(a.seenBiomes))
		for b, ok := range a.seenBiomes {
			if ok {
				biomes = append(biomes, b)
			}
		}
		sort.Strings(biomes)
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(biomes)))
		h.Write(tmp[:])
		for _, b := range biomes {
			h.Write([]byte(b))
		}
		recipes := make([]string, 0, len(a.seenRecipes))
		for r, ok := range a.seenRecipes {
			if ok {
				recipes = append(recipes, r)
			}
		}
		sort.Strings(recipes)
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(recipes)))
		h.Write(tmp[:])
		for _, r := range recipes {
			h.Write([]byte(r))
		}
		events := make([]string, 0, len(a.seenEvents))
		for e, ok := range a.seenEvents {
			if ok {
				events = append(events, e)
			}
		}
		sort.Strings(events)
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(events)))
		h.Write(tmp[:])
		for _, e := range events {
			h.Write([]byte(e))
		}
		decayKeys := make([]string, 0, len(a.funDecay))
		for k, dw := range a.funDecay {
			if dw != nil {
				decayKeys = append(decayKeys, k)
			}
		}
		sort.Strings(decayKeys)
		binary.LittleEndian.PutUint64(tmp[:], uint64(len(decayKeys)))
		h.Write(tmp[:])
		for _, k := range decayKeys {
			dw := a.funDecay[k]
			h.Write([]byte(k))
			binary.LittleEndian.PutUint64(tmp[:], dw.StartTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(dw.Count))
			h.Write(tmp[:])
		}
		h.Write([]byte(a.Equipment.MainHand))
		for i := 0; i < 4; i++ {
			h.Write([]byte(a.Equipment.Armor[i]))
		}

		// Tasks (affects future simulation state; include in digest).
		h.Write([]byte{boolByte(a.MoveTask != nil)})
		if a.MoveTask != nil {
			mt := a.MoveTask
			h.Write([]byte(mt.TaskID))
			h.Write([]byte(string(mt.Kind)))
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.Target.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.Target.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.Target.Z)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(mt.Tolerance))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.StartPos.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.StartPos.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(mt.StartPos.Z)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], mt.StartedTick)
			h.Write(tmp[:])
		}
		h.Write([]byte{boolByte(a.WorkTask != nil)})
		if a.WorkTask != nil {
			wt := a.WorkTask
			h.Write([]byte(wt.TaskID))
			h.Write([]byte(string(wt.Kind)))
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.BlockPos.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.BlockPos.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.BlockPos.Z)))
			h.Write(tmp[:])
			h.Write([]byte(wt.RecipeID))
			h.Write([]byte(wt.ItemID))
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.Count))
			h.Write(tmp[:])
			h.Write([]byte(wt.BlueprintID))
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.Anchor.X)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.Anchor.Y)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(int64(wt.Anchor.Z)))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.Rotation))
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.BuildIndex))
			h.Write(tmp[:])
			h.Write([]byte(wt.TargetID))
			h.Write([]byte(wt.SrcContainer))
			h.Write([]byte(wt.DstContainer))
			binary.LittleEndian.PutUint64(tmp[:], wt.StartedTick)
			h.Write(tmp[:])
			binary.LittleEndian.PutUint64(tmp[:], uint64(wt.WorkTicks))
			h.Write(tmp[:])
		}

		// Inventory (sorted).
		inv := a.InventoryList()
		for _, it := range inv {
			h.Write([]byte(it.Item))
			binary.LittleEndian.PutUint64(tmp[:], uint64(it.Count))
			h.Write(tmp[:])
		}
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func writeItemMap(h hashWriter, tmp [8]byte, m map[string]int) {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		binary.LittleEndian.PutUint64(tmp[:], uint64(m[k]))
		h.Write(tmp[:])
	}
}

type hashWriter interface {
	Write(p []byte) (n int, err error)
}

func taskProgress(start, cur, target Vec3i) float64 {
	total := Manhattan(start, target)
	if total <= 0 {
		return 1
	}
	rem := Manhattan(cur, target)
	p := float64(total-rem) / float64(total)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func workProgress(wt *tasks.WorkTask) float64 {
	switch wt.Kind {
	case tasks.KindMine:
		return float64(wt.WorkTicks) / 10.0
	case tasks.KindPlace:
		return 0
	case tasks.KindCraft:
		return float64(wt.WorkTicks) / 10.0
	case tasks.KindSmelt:
		return float64(wt.WorkTicks) / 10.0
	case tasks.KindBuildBlueprint:
		// BuildIndex progress depends on blueprint length; we don't know here.
		return 0
	default:
		return 0
	}
}

func overMemoryBudget(mem map[string]memoryEntry, key, val string, budget int) bool {
	total := 0
	for k, e := range mem {
		if k == key {
			continue
		}
		total += len(k) + len(e.Value)
	}
	total += len(key) + len(val)
	return total > budget
}

func v3FromTask(v tasks.Vec3i) Vec3i {
	return Vec3i{X: v.X, Y: v.Y, Z: v.Z}
}

func v3ToTask(v Vec3i) tasks.Vec3i {
	return tasks.Vec3i{X: v.X, Y: v.Y, Z: v.Z}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func parseItemPairs(pairs [][]interface{}) (map[string]int, error) {
	out := map[string]int{}
	for _, p := range pairs {
		if len(p) != 2 {
			return nil, fmt.Errorf("pair must have len=2")
		}
		item, ok := p[0].(string)
		if !ok || item == "" {
			return nil, fmt.Errorf("item id must be string")
		}
		n := 0
		switch v := p[1].(type) {
		case float64:
			n = int(v)
		case int:
			n = v
		case int64:
			n = int(v)
		default:
			return nil, fmt.Errorf("count must be number")
		}
		if n <= 0 {
			return nil, fmt.Errorf("count must be > 0")
		}
		out[item] += n
	}
	return out, nil
}

func encodeItemPairs(m map[string]int) [][]interface{} {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([][]interface{}, 0, len(keys))
	for _, k := range keys {
		out = append(out, []interface{}{k, m[k]})
	}
	return out
}

func hasItems(inv map[string]int, want map[string]int) bool {
	if len(want) == 0 {
		return true
	}
	for item, c := range want {
		if inv[item] < c {
			return false
		}
	}
	return true
}

func applyTransfer(src, dst map[string]int, items map[string]int) {
	for item, c := range items {
		src[item] -= c
		dst[item] += c
	}
}

func applyTransferWithTax(src, dst map[string]int, items map[string]int, taxSink map[string]int, taxRate float64) {
	if taxRate <= 0 {
		applyTransfer(src, dst, items)
		return
	}
	if taxRate > 1 {
		taxRate = 1
	}
	for item, c := range items {
		src[item] -= c
		tax := int(float64(c) * taxRate) // floor
		if tax < 0 {
			tax = 0
		}
		if tax > c {
			tax = c
		}
		dst[item] += c - tax
		if taxSink != nil && tax > 0 {
			taxSink[item] += tax
		}
	}
}

func stacksToMap(stacks []protocol.ItemStack) map[string]int {
	out := map[string]int{}
	for _, s := range stacks {
		if s.Item == "" || s.Count <= 0 {
			continue
		}
		out[s.Item] += s.Count
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
