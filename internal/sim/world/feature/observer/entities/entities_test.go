package entities

import "testing"

func TestBuildAgentEntities(t *testing.T) {
	got := BuildAgentEntities("A1", Pos{X: 0, Y: 0, Z: 0}, []AgentInput{
		{ID: "A1", Pos: Pos{X: 0, Y: 0, Z: 0}},
		{ID: "A2", Pos: Pos{X: 2, Y: 0, Z: 0}, OrgID: "ORG1", RepTrade: 500, RepLaw: 100},
		{ID: "A3", Pos: Pos{X: 30, Y: 0, Z: 0}},
	}, 16)
	if len(got) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(got))
	}
	if got[0].ID != "A2" || got[0].Tags[0] != "org:ORG1" {
		t.Fatalf("unexpected entity: %#v", got[0])
	}
}

func TestBuildItemEntitiesSorted(t *testing.T) {
	got := BuildItemEntities([]ItemInput{
		{ID: "IT2", Pos: Pos{X: 0, Y: 0, Z: 0}, Item: "STONE", Count: 1},
		{ID: "IT1", Pos: Pos{X: 1, Y: 0, Z: 0}, Item: "DIRT", Count: 2},
	})
	if len(got) != 2 || got[0].ID != "IT1" || got[1].ID != "IT2" {
		t.Fatalf("expected sorted item ids, got %#v", got)
	}
}
