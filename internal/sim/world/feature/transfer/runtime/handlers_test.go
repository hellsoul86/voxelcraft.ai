package runtime

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	eventspkg "voxelcraft.ai/internal/sim/world/feature/transfer/events"
	orgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
)

func TestHandleEventsReqAgentMissing(t *testing.T) {
	req := eventspkg.Req{AgentID: "A1", SinceCursor: 7, Limit: 10}
	resp := HandleEventsReq(req, func(agentID string, sinceCursor uint64, limit int) ([]eventspkg.CursorItem, uint64, bool) {
		return nil, sinceCursor, false
	})
	if resp.Err == "" {
		t.Fatalf("expected error for missing agent")
	}
	if resp.NextCursor != 7 {
		t.Fatalf("unexpected next cursor: got=%d want=7", resp.NextCursor)
	}
}

func TestHandleEventsReqSuccess(t *testing.T) {
	req := eventspkg.Req{AgentID: "A1", SinceCursor: 3, Limit: 2}
	resp := HandleEventsReq(req, func(agentID string, sinceCursor uint64, limit int) ([]eventspkg.CursorItem, uint64, bool) {
		return []eventspkg.CursorItem{
			{Cursor: 4, Event: protocol.Event{"type": "X"}},
			{Cursor: 5, Event: protocol.Event{"type": "Y"}},
		}, 5, true
	})
	if resp.Err != "" {
		t.Fatalf("unexpected err: %s", resp.Err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("unexpected item count: %d", len(resp.Items))
	}
	if resp.NextCursor != 5 {
		t.Fatalf("unexpected next cursor: %d", resp.NextCursor)
	}
}

func TestHandleAgentPosReq(t *testing.T) {
	req := AgentPosReq{AgentID: "A1"}
	resp := HandleAgentPosReq(req, func(agentID string) ([3]int, bool) {
		return [3]int{1, 0, -2}, true
	})
	if resp.Err != "" {
		t.Fatalf("unexpected err: %s", resp.Err)
	}
	if resp.Pos != [3]int{1, 0, -2} {
		t.Fatalf("unexpected pos: %+v", resp.Pos)
	}
}

func TestBuildOrgMetaRespAndMerge(t *testing.T) {
	resp := BuildOrgMetaResp([]orgpkg.State{
		{OrgID: "ORG_B", Name: "B", Kind: "GUILD", MetaVersion: 2, Members: map[string]string{"A2": "MEMBER"}},
		{OrgID: "ORG_A", Name: "A", Kind: "CITY", MetaVersion: 1, Members: map[string]string{"A1": "OWNER"}},
	})
	if len(resp.Orgs) != 2 {
		t.Fatalf("unexpected org count: %d", len(resp.Orgs))
	}
	if resp.Orgs[0].OrgID != "ORG_A" {
		t.Fatalf("expected normalized sort by OrgID, got first=%s", resp.Orgs[0].OrgID)
	}

	merged, ownerByAgent := BuildOrgMetaMerge(
		[]orgpkg.State{{OrgID: "ORG_A", Name: "A", Kind: "CITY", MetaVersion: 1, Members: map[string]string{"A1": "OWNER"}}},
		[]orgpkg.State{{OrgID: "ORG_A", Name: "A2", Kind: "CITY", MetaVersion: 2, Members: map[string]string{"A1": "OWNER", "A3": "MEMBER"}}},
	)
	if len(merged) != 1 || merged[0].Name != "A2" || merged[0].MetaVersion != 2 {
		t.Fatalf("unexpected merged state: %+v", merged)
	}
	if ownerByAgent["A1"] != "ORG_A" || ownerByAgent["A3"] != "ORG_A" {
		t.Fatalf("unexpected owner map: %+v", ownerByAgent)
	}
}
