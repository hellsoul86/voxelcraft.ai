package world

import (
	"encoding/json"
	"time"
)

func (w *World) stepInternal(joins []JoinRequest, leaves []string, actions []ActionEnvelope, transferOutReqs []transferOutReq, transferInReqs []transferInReq, injectEvents []injectEventReq) {
	stepStart := time.Now()
	nowTick := w.tick.Load()

	// Reset per-tick observer audit buffer (filled by auditSetBlock).
	w.obsAuditsThisTick = w.obsAuditsThisTick[:0]

	// Season rollover happens at tick boundaries before processing joins/leaves/actions for this tick.
	w.maybeWorldResetNotice(nowTick)
	w.maybeSeasonRollover(nowTick)

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

	// Cross-world migration (atomic at tick boundary).
	for _, req := range transferOutReqs {
		w.handleTransferOut(req)
	}
	for _, req := range transferInReqs {
		w.handleTransferIn(req)
	}
	for _, req := range injectEvents {
		if req.AgentID == "" || req.Event == nil {
			continue
		}
		if a := w.agents[req.AgentID]; a != nil {
			a.AddEvent(req.Event)
		}
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
	w.systemConveyors(nowTick)
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

	// Observer stream (admin-only, read-only).
	w.stepObservers(nowTick, recordedJoins, recordedLeaves, recorded)

	digest := w.stateDigest(nowTick)
	if w.tickLogger != nil {
		_ = w.tickLogger.WriteTick(TickLogEntry{Tick: nowTick, Joins: recordedJoins, Leaves: recordedLeaves, Actions: recorded, Digest: digest})
	}

	// Snapshot every N ticks (default 3000), starting after tick 0.
	if w.snapshotSink != nil && nowTick != 0 && w.cfg.SnapshotEveryTicks > 0 {
		every := uint64(w.cfg.SnapshotEveryTicks)
		if every > 0 && nowTick%every == 0 {
			snap := w.ExportSnapshot(nowTick)
			select {
			case w.snapshotSink <- snap:
			default:
				// Drop snapshot if sink is backed up.
			}
		}
	}

	stepMS := float64(time.Since(stepStart).Microseconds()) / 1000.0
	nextTick := w.tick.Add(1)

	sum := StatsBucket{}
	windowTicks := uint64(0)
	if w.stats != nil {
		sum = w.stats.Summarize(nowTick)
		windowTicks = w.stats.WindowTicks()
	}

	if nowTick >= w.nextDensitySampleAt {
		w.resourceDensity = w.computeResourceDensity()
		w.nextDensitySampleAt = nowTick + 300
	}
	resourceDensity := map[string]float64{}
	for k, v := range w.resourceDensity {
		resourceDensity[k] = v
	}

	dm := w.computeDirectorMetrics(nowTick)
	w.metrics.Store(WorldMetrics{
		Tick:         nextTick,
		Agents:       len(w.agents),
		Clients:      len(w.clients),
		LoadedChunks: len(w.chunks.chunks),
		ResetTotal:   w.resetTotal,
		QueueDepths: QueueDepths{
			Inbox:  len(w.inbox),
			Join:   len(w.join),
			Leave:  len(w.leave),
			Attach: len(w.attach),
		},
		StepMS:           stepMS,
		StatsWindowTicks: windowTicks,
		StatsWindow:      sum,
		Director: DirectorMetrics{
			Trade:       dm.Trade,
			Conflict:    dm.Conflict,
			Exploration: dm.Exploration,
			Inequality:  dm.Inequality,
			PublicInfra: dm.PublicInfra,
		},
		ResourceDensity:  resourceDensity,
		Weather:          w.weather,
		WeatherUntilTick: w.weatherUntilTick,
		ActiveEventID:    w.activeEventID,
		ActiveEventStart: w.activeEventStart,
		ActiveEventEnds:  w.activeEventEnds,
		ActiveEventCenter: [3]int{
			w.activeEventCenter.X,
			w.activeEventCenter.Y,
			w.activeEventCenter.Z,
		},
		ActiveEventRadius: w.activeEventRadius,
	})
}

func (w *World) computeResourceDensity() map[string]float64 {
	targets := []string{"COAL_ORE", "IRON_ORE", "COPPER_ORE", "CRYSTAL_ORE", "STONE", "LOG"}
	out := map[string]float64{}
	for _, name := range targets {
		out[name] = 0
	}
	if w == nil || w.chunks == nil || len(w.chunks.chunks) == 0 {
		return out
	}
	idToName := map[uint16]string{}
	for _, name := range targets {
		if id, ok := w.catalogs.Blocks.Index[name]; ok {
			idToName[id] = name
		}
	}
	if len(idToName) == 0 {
		return out
	}
	counts := map[string]int{}
	total := 0
	for _, ch := range w.chunks.chunks {
		if ch == nil {
			continue
		}
		for _, b := range ch.Blocks {
			total++
			if name, ok := idToName[b]; ok {
				counts[name]++
			}
		}
	}
	if total == 0 {
		return out
	}
	denom := float64(total)
	for _, name := range targets {
		out[name] = float64(counts[name]) / denom
	}
	return out
}
