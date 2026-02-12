package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestBoardSearch_FindsMatchingPosts(t *testing.T) {
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
		Seed:       42,
		BoundaryR:  4000,
		StarterItems: map[string]int{
			"BULLETIN_BOARD": 1,
		},
		RateLimits: world.RateLimitConfig{
			PostBoardWindowTicks: 1,
			PostBoardMax:         10,
		},
	}, cats, "p1")
	p1 := h.DefaultAgentID
	p2 := h.Join("p2")
	searcher := h.Join("s")

	boardPosArr := h.LastObsFor(p1).Self.Pos
	boardPos := world.Vec3i{X: boardPosArr[0], Y: 0, Z: boardPosArr[2]}
	h.SetBlock(boardPos, "AIR")
	h.SetAgentPosFor(p2, boardPos)
	h.SetAgentPosFor(searcher, boardPos)

	// Place a physical bulletin board.
	h.ClearAgentEventsFor(p1)
	h.StepFor(p1, nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "BULLETIN_BOARD",
		BlockPos: boardPos.ToArray(),
	}}, nil)
	if got := actionResultCode(h.LastObsFor(p1), "K_place"); got != "" {
		t.Fatalf("place expected ok, got code=%q", got)
	}
	boardID := findEntityIDAt(h.LastObsFor(p1), "BULLETIN_BOARD", boardPos.ToArray())
	if boardID == "" {
		t.Fatalf("missing BULLETIN_BOARD entity at %v; entities=%v", boardPos, h.LastObsFor(p1).Entities)
	}

	h.ClearAgentEventsFor(p1)
	h.StepFor(p1, []protocol.InstantReq{{
		ID:       "I1",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "Market Week",
		Body:     "market tax reduced",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(p1), "I1"); got != "" {
		t.Fatalf("post expected ok, got code=%q", got)
	}

	h.ClearAgentEventsFor(p2)
	h.StepFor(p2, []protocol.InstantReq{{
		ID:       "I2",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "Need Iron",
		Body:     "offering plank 1:1",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(p2), "I2"); got != "" {
		t.Fatalf("post expected ok, got code=%q", got)
	}

	h.ClearAgentEventsFor(searcher)
	h.StepFor(searcher, []protocol.InstantReq{{
		ID:       "I_search",
		Type:     "SEARCH_BOARD",
		TargetID: boardID,
		Text:     "market",
		Limit:    10,
	}}, nil, nil)

	obs := h.LastObsFor(searcher)
	found := false
	for _, ev := range obs.Events {
		if ev["type"] != "BOARD_SEARCH" {
			continue
		}
		if ev["board_id"] != boardID || ev["query"] != "market" {
			t.Fatalf("unexpected search event: %+v", ev)
		}
		res, ok := ev["results"].([]interface{})
		if !ok {
			t.Fatalf("results has unexpected type: %T", ev["results"])
		}
		if len(res) != 1 {
			t.Fatalf("results len=%d want 1", len(res))
		}
		m0, ok := res[0].(map[string]interface{})
		if !ok {
			t.Fatalf("results[0] has unexpected type: %T", res[0])
		}
		if title, _ := m0["title"].(string); title != "Market Week" {
			t.Fatalf("unexpected result[0]: %+v", m0)
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("expected BOARD_SEARCH event; events=%v", obs.Events)
	}
}

