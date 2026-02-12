package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world/logic/ids"
	world "voxelcraft.ai/internal/sim/world"
)

func findContainerAt(s snapshot.SnapshotV1, typ string, pos [3]int) *snapshot.ContainerV1 {
	for i := range s.Containers {
		c := &s.Containers[i]
		if c.Type == typ && c.Pos == pos {
			return c
		}
	}
	return nil
}

func findContract(s snapshot.SnapshotV1, contractID string) *snapshot.ContractV1 {
	for i := range s.Contracts {
		c := &s.Contracts[i]
		if c.ContractID == contractID {
			return c
		}
	}
	return nil
}

func findAgent(s snapshot.SnapshotV1, agentID string) *snapshot.AgentV1 {
	for i := range s.Agents {
		a := &s.Agents[i]
		if a.ID == agentID {
			return a
		}
	}
	return nil
}

func TestContract_RequirementsDoNotCountEscrow(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
	}, cats, "poster")
	poster := h.DefaultAgentID
	acceptor := h.Join("acceptor")

	// Place a contract terminal adjacent to poster.
	anchorArr := h.LastObsFor(poster).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	termPos := world.Vec3i{X: anchor.X + 1, Y: 0, Z: anchor.Z}
	h.SetBlock(termPos, "AIR")

	h.AddInventoryFor(poster, "CONTRACT_TERMINAL", 1)
	h.ClearAgentEventsFor(poster)
	obs := h.StepFor(poster, nil, []protocol.TaskReq{{
		ID:       "K_place_term",
		Type:     "PLACE",
		ItemID:   "CONTRACT_TERMINAL",
		BlockPos: termPos.ToArray(),
	}}, nil)
	if hasTaskFail(obs, "") {
		t.Fatalf("place terminal failed: events=%v", obs.Events)
	}

	termID := ids.ContainerID("CONTRACT_TERMINAL", termPos.X, termPos.Y, termPos.Z)

	// Record baseline reputation/fun before completion.
	_, snap0 := h.Snapshot()
	a0 := findAgent(snap0, acceptor)
	if a0 == nil {
		t.Fatalf("missing acceptor in snapshot")
	}
	baseRepTrade := a0.RepTrade
	baseRepSocial := a0.RepSocial
	baseFunSocial := a0.FunSocial

	// Poster posts contract: requires 10 iron ingots; reward is also 10 iron ingots (escrowed).
	h.AddInventoryFor(poster, "IRON_INGOT", 10)
	h.ClearAgentEventsFor(poster)
	obs = h.StepFor(poster, []protocol.InstantReq{{
		ID:            "I_post",
		Type:          "POST_CONTRACT",
		TerminalID:    termID,
		ContractKind:  "GATHER",
		Requirements:  []protocol.ItemStack{{Item: "IRON_INGOT", Count: 10}},
		Reward:        []protocol.ItemStack{{Item: "IRON_INGOT", Count: 10}},
		DurationTicks: 1000,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_post"); got != "" {
		t.Fatalf("POST_CONTRACT expected ok, got code=%q events=%v", got, obs.Events)
	}
	contractID := actionResultFieldString(obs, "I_post", "contract_id")
	if contractID == "" {
		t.Fatalf("missing contract_id; events=%v", obs.Events)
	}

	// Terminal should hold the escrowed reward and reserve it.
	_, snap1 := h.Snapshot()
	term := findContainerAt(snap1, "CONTRACT_TERMINAL", termPos.ToArray())
	if term == nil {
		t.Fatalf("missing terminal in snapshot")
	}
	if got := term.Inventory["IRON_INGOT"]; got != 10 {
		t.Fatalf("terminal inv after post: got %d want 10", got)
	}
	if got := term.Reserved["IRON_INGOT"]; got != 10 {
		t.Fatalf("terminal reserved after post: got %d want 10", got)
	}

	// Move acceptor in range of terminal.
	h.SetAgentPosFor(acceptor, anchor)
	h.StepNoop()

	// Accept contract.
	h.ClearAgentEventsFor(acceptor)
	obs = h.StepFor(acceptor, []protocol.InstantReq{{
		ID:         "I_accept",
		Type:       "ACCEPT_CONTRACT",
		TerminalID: termID,
		ContractID: contractID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_accept"); got != "" {
		t.Fatalf("ACCEPT_CONTRACT expected ok, got code=%q events=%v", got, obs.Events)
	}

	// Submit without delivering: should fail because escrow doesn't count toward requirements.
	h.ClearAgentEventsFor(acceptor)
	obs = h.StepFor(acceptor, []protocol.InstantReq{{
		ID:         "I_submit_1",
		Type:       "SUBMIT_CONTRACT",
		TerminalID: termID,
		ContractID: contractID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_submit_1"); got != "E_BLOCKED" {
		t.Fatalf("SUBMIT_CONTRACT expected E_BLOCKED, got code=%q events=%v", got, obs.Events)
	}
	_, snapAfterFail := h.Snapshot()
	if c := findContract(snapAfterFail, contractID); c == nil || c.State != "ACCEPTED" {
		state := "<missing>"
		if c != nil {
			state = c.State
		}
		t.Fatalf("expected contract to remain ACCEPTED after failed submit, got %q", state)
	}

	// Deliver 10 iron ingots to terminal.
	h.AddInventoryFor(acceptor, "IRON_INGOT", 10)
	h.ClearAgentEventsFor(acceptor)
	obs = h.StepFor(acceptor, nil, []protocol.TaskReq{{
		ID:           "K_transfer",
		Type:         "TRANSFER",
		Src:          "SELF",
		Dst:          termID,
		ItemID:       "IRON_INGOT",
		Count:        10,
	}}, nil)
	if got := actionResultCode(obs, "K_transfer"); got != "" {
		t.Fatalf("TRANSFER expected ok, got code=%q events=%v", got, obs.Events)
	}
	if hasTaskFail(obs, "") {
		t.Fatalf("TRANSFER failed: events=%v", obs.Events)
	}

	// Delivering requirements should auto-settle the contract on the same tick (tickContracts runs after work).
	_, snap3 := h.Snapshot()
	c3 := findContract(snap3, contractID)
	if c3 == nil {
		t.Fatalf("missing contract in snapshot after settle")
	}
	if c3.State != "COMPLETED" {
		t.Fatalf("expected COMPLETED state, got %q", c3.State)
	}
	term = findContainerAt(snap3, "CONTRACT_TERMINAL", termPos.ToArray())
	if term == nil {
		t.Fatalf("missing terminal in snapshot after settle")
	}
	if got := term.Inventory["IRON_INGOT"]; got != 0 {
		t.Fatalf("terminal inv after settle: got %d want 0", got)
	}
	if got := term.Reserved["IRON_INGOT"]; got != 0 {
		t.Fatalf("terminal reserved after settle: got %d want 0", got)
	}
	if got := invCount(h.LastObsFor(poster).Inventory, "IRON_INGOT"); got != 10 {
		t.Fatalf("poster inv after settle: got %d want 10", got)
	}
	if got := invCount(h.LastObsFor(acceptor).Inventory, "IRON_INGOT"); got != 10 {
		t.Fatalf("acceptor inv after settle: got %d want 10", got)
	}

	a3 := findAgent(snap3, acceptor)
	if a3 == nil {
		t.Fatalf("missing acceptor in snapshot after settle")
	}
	if a3.RepTrade <= baseRepTrade || a3.RepSocial <= baseRepSocial {
		t.Fatalf("expected reputation increase on successful submit, got trade=%d->%d social=%d->%d", baseRepTrade, a3.RepTrade, baseRepSocial, a3.RepSocial)
	}
	if a3.FunSocial <= baseFunSocial {
		t.Fatalf("expected Fun.Social increase on successful submit, got %d->%d", baseFunSocial, a3.FunSocial)
	}
}
