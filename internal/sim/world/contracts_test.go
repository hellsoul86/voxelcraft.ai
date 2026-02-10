package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
)

func TestContract_RequirementsDoNotCountEscrow(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		out := make(chan []byte, 1)
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: true, Out: out, Resp: resp})
		r := <-resp
		return w.agents[r.Welcome.AgentID]
	}

	poster := join("poster")
	acceptor := join("acceptor")

	// Create a terminal near poster.
	termPos := poster.Pos
	term := w.ensureContainer(termPos, "CONTRACT_TERMINAL")
	termID := containerID("CONTRACT_TERMINAL", termPos)

	poster.Inventory["IRON_INGOT"] = 10

	// Post contract: requires 10 iron ingots, reward is also 10 iron ingots.
	w.applyInstant(poster, protocol.InstantReq{
		ID:            "I_post",
		Type:          "POST_CONTRACT",
		TerminalID:    termID,
		ContractKind:  "GATHER",
		Requirements:  []protocol.ItemStack{{Item: "IRON_INGOT", Count: 10}},
		Reward:        []protocol.ItemStack{{Item: "IRON_INGOT", Count: 10}},
		DurationTicks: 1000,
	}, 0)

	if got := term.Inventory["IRON_INGOT"]; got != 10 {
		t.Fatalf("terminal inv after post: got %d want 10", got)
	}
	if got := term.Reserved["IRON_INGOT"]; got != 10 {
		t.Fatalf("terminal reserved after post: got %d want 10", got)
	}

	var cid string
	for id := range w.contracts {
		cid = id
		break
	}
	if cid == "" {
		t.Fatalf("expected contract id")
	}

	// Move acceptor in range of the terminal for accept/submit.
	acceptor.Pos = poster.Pos

	// Accept contract.
	w.applyInstant(acceptor, protocol.InstantReq{
		ID:         "I_accept",
		Type:       "ACCEPT_CONTRACT",
		TerminalID: termID,
		ContractID: cid,
	}, 1)

	// Submit without delivering: should fail because escrow doesn't count toward requirements.
	w.applyInstant(acceptor, protocol.InstantReq{
		ID:         "I_submit_1",
		Type:       "SUBMIT_CONTRACT",
		TerminalID: termID,
		ContractID: cid,
	}, 2)

	if w.contracts[cid].State != ContractAccepted {
		t.Fatalf("expected accepted state, got %s", w.contracts[cid].State)
	}

	// Deliver 10 iron ingots to terminal.
	acceptor.Pos = poster.Pos // ensure in range for transfer
	acceptor.Inventory["IRON_INGOT"] = 10
	acceptor.WorkTask = &tasks.WorkTask{
		TaskID:       "T_transfer",
		Kind:         tasks.KindTransfer,
		SrcContainer: "SELF",
		DstContainer: termID,
		ItemID:       "IRON_INGOT",
		Count:        10,
		StartedTick:  3,
	}
	w.tickTransfer(acceptor, acceptor.WorkTask, 3)

	if got := term.Inventory["IRON_INGOT"]; got != 20 {
		t.Fatalf("terminal inv after deliver: got %d want 20", got)
	}

	// Submit again: should complete.
	w.applyInstant(acceptor, protocol.InstantReq{
		ID:         "I_submit_2",
		Type:       "SUBMIT_CONTRACT",
		TerminalID: termID,
		ContractID: cid,
	}, 4)

	if w.contracts[cid].State != ContractCompleted {
		t.Fatalf("expected completed state, got %s", w.contracts[cid].State)
	}
	if got := term.Inventory["IRON_INGOT"]; got != 0 {
		t.Fatalf("terminal inv after settle: got %d want 0", got)
	}
	if got := term.Reserved["IRON_INGOT"]; got != 0 {
		t.Fatalf("terminal reserved after settle: got %d want 0", got)
	}
	if got := acceptor.Inventory["IRON_INGOT"]; got != 10 {
		t.Fatalf("acceptor inv after settle: got %d want 10", got)
	}
	if term.Owed == nil || term.Owed[poster.ID]["IRON_INGOT"] != 10 {
		t.Fatalf("poster owed mismatch: %+v", term.Owed)
	}

	// Reputation/fun should be granted on successful manual submit.
	if got := acceptor.RepTrade; got != 506 {
		t.Fatalf("acceptor RepTrade=%d want 506", got)
	}
	if got := acceptor.RepSocial; got != 502 {
		t.Fatalf("acceptor RepSocial=%d want 502", got)
	}
	if got := acceptor.Fun.Social; got != 5 {
		t.Fatalf("acceptor Fun.Social=%d want 5", got)
	}
}
