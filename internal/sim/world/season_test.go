package world

import (
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
)

func TestSeasonRollover_ForcesSnapshotAndResetsWorld(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		// Keep a realistic day length so the Director doesn't schedule a new day-1 event each tick.
		DayTicks:          6000,
		SeasonLengthTicks: 3,
		ObsRadius:         7,
		Height:            64,
		Seed:              42,
		BoundaryR:         4000,
		// Avoid regular snapshots/director ticks in this unit test.
		SnapshotEveryTicks: 1000,
		DirectorEveryTicks: 1000,
	}

	w, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	snapCh := make(chan snapshot.SnapshotV1, 8)
	w.SetSnapshotSink(snapCh)

	// Join an agent without a connected client so we can inspect its event queue directly.
	join := w.joinAgent("bot", true, nil)
	aid := join.Welcome.AgentID
	a := w.agents[aid]
	if a == nil {
		t.Fatalf("missing agent %s", aid)
	}

	// Step ticks 0,1,2. Season rollover happens at the tick boundary at nowTick=3.
	w.step(nil, nil, nil) // tick 0
	w.step(nil, nil, nil) // tick 1
	w.step(nil, nil, nil) // tick 2
	if got := w.CurrentTick(); got != 3 {
		t.Fatalf("tick after 3 steps: got %d want 3", got)
	}

	// Add end-of-season state that should be captured by the forced snapshot and then cleared.
	org := &Organization{
		OrgID:       "ORGTEST",
		Kind:        OrgGuild,
		Name:        "TestOrg",
		CreatedTick: 0,
		Members:     map[string]OrgRole{aid: OrgLeader},
		Treasury:    map[string]int{"IRON_INGOT": 5},
	}
	w.orgs[org.OrgID] = org
	a.OrgID = org.OrgID

	// Mutate agent state so we can verify what persists across seasons.
	a.RepTrade = 777
	a.MemorySave("note", "persist", 0, 0)
	a.Inventory["IRON_INGOT"] = 99
	a.MoveTask = &tasks.MovementTask{
		TaskID:    "T1",
		Kind:      tasks.KindMoveTo,
		Target:    tasks.Vec3i{X: 10, Y: 0, Z: -10},
		Tolerance: 1.2,
	}
	a.WorkTask = &tasks.WorkTask{
		TaskID:   "T2",
		Kind:     tasks.KindMine,
		BlockPos: tasks.Vec3i{X: 0, Y: 10, Z: 0},
	}

	w.claims["LAND1"] = &LandClaim{
		LandID: "LAND1",
		Owner:  aid,
		Anchor: Vec3i{X: 0, Y: 0, Z: 0},
		Radius: 4,
		Flags:  ClaimFlags{AllowBuild: true, AllowBreak: true},
	}
	termPos := Vec3i{X: 2, Y: 20, Z: 2}
	w.containers[termPos] = &Container{Type: "CONTRACT_TERMINAL", Pos: termPos, Inventory: map[string]int{"PLANK": 1}}

	w.items["ITX"] = &ItemEntity{EntityID: "ITX", Pos: Vec3i{X: 3, Y: 20, Z: 3}, Item: "PLANK", Count: 1}
	w.itemsAt[Vec3i{X: 3, Y: 20, Z: 3}] = []string{"ITX"}

	w.trades["TR1"] = &Trade{TradeID: "TR1", From: aid, To: "A999", Offer: map[string]int{"PLANK": 1}, Request: map[string]int{"COAL": 1}}

	bid := boardIDAt(Vec3i{X: 4, Y: 20, Z: 4})
	w.boards[bid] = &Board{BoardID: bid, Posts: []BoardPost{{PostID: "P1", Author: aid, Title: "t", Body: "b", Tick: 2}}}

	signPos := Vec3i{X: 5, Y: 20, Z: 5}
	w.signs[signPos] = &Sign{Pos: signPos, Text: "hello", UpdatedTick: 2, UpdatedBy: aid}

	convPos := Vec3i{X: 6, Y: 20, Z: 6}
	w.conveyors[convPos] = ConveyorMeta{DX: 1, DZ: 0}
	if b, ok := w.catalogs.Blocks.Index["CONVEYOR"]; ok {
		w.chunks.SetBlock(convPos, b)
	}

	switchPos := Vec3i{X: 7, Y: 20, Z: 7}
	w.switches[switchPos] = true
	if b, ok := w.catalogs.Blocks.Index["SWITCH"]; ok {
		w.chunks.SetBlock(switchPos, b)
	}

	w.contracts["C1"] = &Contract{
		ContractID:   "C1",
		TerminalPos:  termPos,
		Poster:       aid,
		Acceptor:     "A999",
		Kind:         "DELIVER",
		Requirements: map[string]int{"PLANK": 1},
		Reward:       map[string]int{"COAL": 1},
		Deposit:      map[string]int{"STONE": 1},
		CreatedTick:  2,
		DeadlineTick: 100,
		State:        ContractOpen,
	}

	w.laws["L1"] = &Law{LawID: "L1", LandID: "LAND1", TemplateID: "MARKET_TAX", Title: "tax", Params: map[string]string{"rate": "0.05"}, Status: LawActive}

	w.structures["S1"] = &Structure{StructureID: "S1", BlueprintID: "house_small", BuilderID: aid, Anchor: Vec3i{X: 8, Y: 20, Z: 8}, CompletedTick: 2}

	// Tick 3 should rollover (seasonLen=3): it forces a snapshot at tick 2 and resets the world in-place.
	w.step(nil, nil, nil) // tick 3

	// Forced end-of-season snapshot should have been written at archiveTick=2 and reflect the old season seed.
	select {
	case snap := <-snapCh:
		if snap.Header.Tick != 2 {
			t.Fatalf("archive snapshot tick: got %d want 2", snap.Header.Tick)
		}
		if snap.Seed != 42 {
			t.Fatalf("archive snapshot seed: got %d want 42", snap.Seed)
		}
		if snap.SeasonLengthTicks != 3 {
			t.Fatalf("archive snapshot season_length_ticks: got %d want 3", snap.SeasonLengthTicks)
		}
		if len(snap.Claims) != 1 || len(snap.Containers) != 1 || len(snap.Contracts) != 1 || len(snap.Laws) != 1 {
			t.Fatalf("archive snapshot content counts: claims=%d containers=%d contracts=%d laws=%d",
				len(snap.Claims), len(snap.Containers), len(snap.Contracts), len(snap.Laws))
		}
	default:
		t.Fatalf("expected forced season-end snapshot")
	}

	// World should now be in the new season: seed advanced, world-scoped state cleared, orgs kept but treasury reset.
	if w.cfg.Seed != 43 {
		t.Fatalf("seed after rollover: got %d want 43", w.cfg.Seed)
	}
	if len(w.claims) != 0 || len(w.containers) != 0 || len(w.items) != 0 || len(w.trades) != 0 || len(w.boards) != 0 || len(w.signs) != 0 || len(w.contracts) != 0 || len(w.laws) != 0 || len(w.structures) != 0 || len(w.conveyors) != 0 || len(w.switches) != 0 {
		t.Fatalf("expected world-scoped state to be cleared after rollover")
	}
	if gotOrg := w.orgs[org.OrgID]; gotOrg == nil {
		t.Fatalf("expected org to persist across seasons")
	} else if len(gotOrg.Treasury) != 0 {
		t.Fatalf("expected org treasury reset; got %v", gotOrg.Treasury)
	}

	// Agent should have been reset physically, but keep reputation and memory.
	if a.RepTrade != 777 {
		t.Fatalf("rep trade should persist; got %d want 777", a.RepTrade)
	}
	if got := a.Memory["note"].Value; got != "persist" {
		t.Fatalf("memory should persist; got %q", got)
	}
	if a.Inventory["IRON_INGOT"] != 0 {
		t.Fatalf("inventory should reset; IRON_INGOT=%d", a.Inventory["IRON_INGOT"])
	}
	if a.Inventory["PLANK"] != 20 || a.Inventory["COAL"] != 10 || a.Inventory["STONE"] != 20 || a.Inventory["BERRIES"] != 10 {
		t.Fatalf("starter kit mismatch after rollover: inv=%v", a.Inventory)
	}
	if a.MoveTask != nil || a.WorkTask != nil {
		t.Fatalf("tasks should be canceled at rollover: move=%v work=%v", a.MoveTask, a.WorkTask)
	}

	// Agent should receive a season rollover event (and likely a FUN novelty event too).
	found := false
	for _, ev := range a.Events {
		typ, _ := ev["type"].(string)
		if typ != "SEASON_ROLLOVER" {
			continue
		}
		found = true
		if season, ok := ev["season"].(int); !ok || season != 2 {
			t.Fatalf("season event season: %#v", ev["season"])
		}
		if at, ok := ev["archive_tick"].(uint64); !ok || at != 2 {
			t.Fatalf("season event archive_tick: %#v", ev["archive_tick"])
		}
		if seed, ok := ev["seed"].(int64); !ok || seed != 43 {
			t.Fatalf("season event seed: %#v", ev["seed"])
		}
		break
	}
	if !found {
		t.Fatalf("missing SEASON_ROLLOVER event; got %v", a.Events)
	}
}
