package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestLaw_FineBreakPerBlock_FinesDeniedMine(t *testing.T) {
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
	visitor := join("visitor")
	if owner == nil || visitor == nil {
		t.Fatalf("missing agents")
	}

	// Create a land claim owned by owner, with visitors disallowed by default.
	landID := w.newLandID(owner.ID)
	w.claims[landID] = &LandClaim{
		LandID:  landID,
		Owner:   owner.ID,
		Anchor:  owner.Pos,
		Radius:  32,
		Flags:   ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
		Members: map[string]bool{},
	}

	// Activate fine law.
	law := &Law{
		LandID:     landID,
		TemplateID: "FINE_BREAK_PER_BLOCK",
		Params:     map[string]string{"fine_item": "IRON_INGOT", "fine_per_block": "3"},
	}
	if err := w.activateLaw(0, law); err != nil {
		t.Fatalf("activate: %v", err)
	}

	visitor.Inventory["IRON_INGOT"] = 10
	owner.Inventory["IRON_INGOT"] = 0

	// Place a mineable block within reach of visitor.
	pos := Vec3i{X: visitor.Pos.X + 1, Y: visitor.Pos.Y, Z: visitor.Pos.Z}
	stone := w.catalogs.Blocks.Index["STONE"]
	w.chunks.SetBlock(pos, stone)

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         visitor.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "MINE", BlockPos: pos.ToArray()},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: visitor.ID, Act: act}})

	if got := visitor.Inventory["IRON_INGOT"]; got != 7 {
		t.Fatalf("visitor fine: got %d want %d", got, 7)
	}
	if got := owner.Inventory["IRON_INGOT"]; got != 3 {
		t.Fatalf("owner fine credit: got %d want %d", got, 3)
	}
	if got := w.chunks.GetBlock(pos); got != stone {
		t.Fatalf("block should remain (denied mine): got %d want %d", got, stone)
	}
}

func TestLaw_AccessPassCore_ChargesOnCoreEntry(t *testing.T) {
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
	visitor := join("visitor")
	if owner == nil || visitor == nil {
		t.Fatalf("missing agents")
	}

	landID := w.newLandID(owner.ID)
	w.claims[landID] = &LandClaim{
		LandID:  landID,
		Owner:   owner.ID,
		Anchor:  owner.Pos,
		Radius:  32,
		Flags:   ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
		Members: map[string]bool{},
	}
	law := &Law{
		LandID:     landID,
		TemplateID: "ACCESS_PASS_CORE",
		Params:     map[string]string{"ticket_item": "IRON_INGOT", "ticket_cost": "2"},
	}
	if err := w.activateLaw(0, law); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// Place visitor just outside core (dx=17), within claim.
	visitor.Pos = Vec3i{X: owner.Pos.X + 17, Y: w.surfaceY(owner.Pos.X+17, owner.Pos.Z), Z: owner.Pos.Z}
	visitor.Inventory["IRON_INGOT"] = 5
	owner.Inventory["IRON_INGOT"] = 0

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         visitor.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "MOVE_TO", Target: owner.Pos.ToArray(), Tolerance: 1.2},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: visitor.ID, Act: act}})

	if got := visitor.Inventory["IRON_INGOT"]; got != 3 {
		t.Fatalf("ticket charge: got %d want %d", got, 3)
	}
	if got := owner.Inventory["IRON_INGOT"]; got != 2 {
		t.Fatalf("ticket credit: got %d want %d", got, 2)
	}
}

func TestLaw_AccessPassCore_BlocksIfInsufficientTicket(t *testing.T) {
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
	visitor := join("visitor")
	if owner == nil || visitor == nil {
		t.Fatalf("missing agents")
	}

	landID := w.newLandID(owner.ID)
	w.claims[landID] = &LandClaim{
		LandID:  landID,
		Owner:   owner.ID,
		Anchor:  owner.Pos,
		Radius:  32,
		Flags:   ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
		Members: map[string]bool{},
	}
	law := &Law{
		LandID:     landID,
		TemplateID: "ACCESS_PASS_CORE",
		Params:     map[string]string{"ticket_item": "IRON_INGOT", "ticket_cost": "2"},
	}
	if err := w.activateLaw(0, law); err != nil {
		t.Fatalf("activate: %v", err)
	}

	start := Vec3i{X: owner.Pos.X + 17, Y: w.surfaceY(owner.Pos.X+17, owner.Pos.Z), Z: owner.Pos.Z}
	visitor.Pos = start
	visitor.Inventory["IRON_INGOT"] = 1

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         visitor.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "MOVE_TO", Target: owner.Pos.ToArray(), Tolerance: 1.2},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: visitor.ID, Act: act}})

	if visitor.MoveTask != nil {
		t.Fatalf("move task should fail/cancel when ticket missing")
	}
	if got := visitor.Pos; got != start {
		t.Fatalf("position should not change when ticket missing: got %+v want %+v", got, start)
	}
}
