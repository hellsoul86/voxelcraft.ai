package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestMarketWeek_ReducesMarketTaxOnTrades(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		r := <-resp
		return w.agents[r.Welcome.AgentID]
	}
	owner := join("owner")
	seller := join("seller")
	buyer := join("buyer")
	if owner == nil || seller == nil || buyer == nil {
		t.Fatalf("missing agents")
	}

	// Reset inventories for predictable assertions.
	owner.Inventory = map[string]int{}
	seller.Inventory = map[string]int{}
	buyer.Inventory = map[string]int{}

	// Put everyone inside the same claim with a non-zero market tax.
	seller.Pos = owner.Pos
	buyer.Pos = owner.Pos
	land := &LandClaim{
		LandID:    "LAND_MARKET",
		Owner:     owner.ID,
		Anchor:    owner.Pos,
		Radius:    32,
		Flags:     ClaimFlags{AllowBuild: true, AllowBreak: true, AllowDamage: false, AllowTrade: true},
		MarketTax: 0.10,
	}
	w.claims[land.LandID] = land

	// Start Market Week.
	w.startEvent(0, "MARKET_WEEK")

	seller.Inventory["PLANK"] = 20
	buyer.Inventory["IRON_INGOT"] = 20

	// Seller offers 20 planks for 20 iron ingots.
	w.applyInstant(seller, protocol.InstantReq{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      buyer.ID,
		Offer:   [][]interface{}{{"PLANK", 20}},
		Request: [][]interface{}{{"IRON_INGOT", 20}},
	}, 0)
	var tradeID string
	for id := range w.trades {
		tradeID = id
		break
	}
	if tradeID == "" {
		t.Fatalf("expected trade id")
	}

	w.applyInstant(buyer, protocol.InstantReq{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}, 1)

	// TaxRate 0.10 * 0.5 => 0.05 => floor(20*0.05)=1 per transfer.
	if got := owner.Inventory["PLANK"]; got != 1 {
		t.Fatalf("owner tax PLANK=%d want 1", got)
	}
	if got := owner.Inventory["IRON_INGOT"]; got != 1 {
		t.Fatalf("owner tax IRON_INGOT=%d want 1", got)
	}
}

func TestBanditCamp_OpenChestAwardsGoal(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "raider", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	w.startEvent(0, "BANDIT_CAMP")

	chestPos := w.activeEventCenter
	chestID := containerID("CHEST", chestPos)
	a.Pos = chestPos

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K_open", Type: "OPEN", TargetID: chestID},
		},
	}
	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	found := false
	for _, ev := range a.Events {
		if ev["type"] == "EVENT_GOAL" && ev["event_id"] == "BANDIT_CAMP" && ev["kind"] == "LOOT_BANDITS" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EVENT_GOAL LOOT_BANDITS")
	}
}

func TestBlightZone_ReducesStaminaRecoveryInZone(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "walker", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	w.startEvent(0, "BLIGHT_ZONE")

	a.Pos = w.activeEventCenter
	a.Hunger = 20
	a.StaminaMilli = 0
	w.weather = "CLEAR"

	w.systemEnvironment(1)
	if got := a.StaminaMilli; got != 0 {
		t.Fatalf("stamina in blight zone: got %d want 0", got)
	}

	// Outside the zone stamina should recover.
	a.Pos = Vec3i{X: w.activeEventCenter.X + w.activeEventRadius + 10, Y: a.Pos.Y, Z: w.activeEventCenter.Z}
	w.systemEnvironment(2)
	if got := a.StaminaMilli; got == 0 {
		t.Fatalf("expected stamina to recover outside zone")
	}
}

func TestFloodWarning_SlowsMovementInZone(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "mover", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	w.startEvent(0, "FLOOD_WARNING")

	start := w.activeEventCenter
	a.Pos = start
	target := Vec3i{X: start.X + 10, Y: start.Y, Z: start.Z}

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "MOVE_TO", Target: target.ToArray(), Tolerance: 1.2},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}}) // tick 0: should move
	w.step(nil, nil, nil)                                         // tick 1: should be slowed
	w.step(nil, nil, nil)                                         // tick 2: should move

	if got := a.Pos.X; got != start.X+2 {
		t.Fatalf("pos.X=%d want %d", got, start.X+2)
	}
}

func TestBuilderExpo_SpawnsNoticeBoardAndSign(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	w.startEvent(0, "BUILDER_EXPO")
	center := w.activeEventCenter
	// surfaceY() returns the air block above the topmost non-air block (including water); after
	// spawning the board/sign, the topmost non-air becomes the spawned block itself, so surfaceY() moves up
	// by 1. We want the placed block position, hence -1.
	y := w.surfaceY(center.X, center.Z) - 1
	if y < 1 {
		y = 1
	}
	boardPos := Vec3i{X: center.X, Y: y, Z: center.Z}
	signPos := Vec3i{X: center.X + 1, Y: y, Z: center.Z}

	bBoard := w.blockName(w.chunks.GetBlock(boardPos))
	if bBoard != "BULLETIN_BOARD" {
		t.Fatalf("board block=%q want BULLETIN_BOARD", bBoard)
	}
	bSign := w.blockName(w.chunks.GetBlock(signPos))
	if bSign != "SIGN" {
		t.Fatalf("sign block=%q want SIGN", bSign)
	}
	if s := w.signs[signPos]; s == nil || s.Text == "" {
		t.Fatalf("expected sign text to be set")
	}
	if b := w.boards[boardIDAt(boardPos)]; b == nil || len(b.Posts) == 0 {
		t.Fatalf("expected board post to be created")
	}
}
