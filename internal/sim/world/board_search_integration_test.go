package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestBoardSearch_FindsMatchingPosts(t *testing.T) {
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
	poster1 := join("p1")
	poster2 := join("p2")
	searcher := join("s")
	if poster1 == nil || poster2 == nil || searcher == nil {
		t.Fatalf("missing agents")
	}

	// Place a physical bulletin board in the world.
	boardPos := poster1.Pos
	bid := w.catalogs.Blocks.Index["BULLETIN_BOARD"]
	w.chunks.SetBlock(boardPos, bid)
	w.ensureBoard(boardPos)
	boardID := boardIDAt(boardPos)

	poster1.Pos = boardPos
	poster2.Pos = boardPos
	searcher.Pos = boardPos

	w.applyInstant(poster1, protocol.InstantReq{
		ID:       "I1",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "Market Week",
		Body:     "market tax reduced",
	}, 0)
	w.applyInstant(poster2, protocol.InstantReq{
		ID:       "I2",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "Need Iron",
		Body:     "offering plank 1:1",
	}, 1)

	searcher.Events = nil
	w.applyInstant(searcher, protocol.InstantReq{
		ID:       "I_search",
		Type:     "SEARCH_BOARD",
		TargetID: boardID,
		Text:     "market",
		Limit:    10,
	}, 2)

	found := false
	for _, ev := range searcher.Events {
		if ev["type"] != "BOARD_SEARCH" {
			continue
		}
		if ev["board_id"] != boardID || ev["query"] != "market" {
			t.Fatalf("unexpected search event: %+v", ev)
		}
		res, ok := ev["results"].([]map[string]any)
		if !ok {
			t.Fatalf("results has unexpected type: %T", ev["results"])
		}
		if len(res) != 1 {
			t.Fatalf("results len=%d want 1", len(res))
		}
		if res[0]["title"] != "Market Week" {
			t.Fatalf("unexpected result[0]: %+v", res[0])
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("expected BOARD_SEARCH event")
	}
}
