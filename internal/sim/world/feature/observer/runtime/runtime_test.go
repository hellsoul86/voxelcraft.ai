package runtime

import "testing"

func TestBuildStatus(t *testing.T) {
	got := BuildStatus(0, 100, "COLD")
	if len(got) == 0 {
		t.Fatalf("empty status")
	}
}

func TestBuildPublicBoards(t *testing.T) {
	boards := BuildPublicBoards([]BoardInput{
		{
			BoardID: "B1",
			Posts: []BoardPostInput{
				{PostID: "P1", Author: "A", Title: "T1", Body: "hello"},
				{PostID: "P2", Author: "B", Title: "T2", Body: "world"},
			},
		},
	}, 1, 10)
	if len(boards) != 1 || len(boards[0].TopPosts) != 1 || boards[0].TopPosts[0].PostID != "P2" {
		t.Fatalf("unexpected boards output: %+v", boards)
	}
}

func TestBuildLocalRules(t *testing.T) {
	out := BuildLocalRules(LocalRulesInput{
		Permissions: map[string]bool{"can_build": true},
		HasLand:     false,
	})
	if out.Role != "WILD" || out.Tax["market"] != 0 {
		t.Fatalf("unexpected wild rules: %+v", out)
	}

	out = BuildLocalRules(LocalRulesInput{
		Permissions:        map[string]bool{"can_build": false},
		HasLand:            true,
		LandID:             "L1",
		Owner:              "A1",
		IsOwner:            true,
		MarketTax:          0.05,
		MaintenanceDueTick: 123,
		MaintenanceStage:   2,
	})
	if out.Role != "OWNER" || out.LandID != "L1" || out.Tax["market"] != 0.05 {
		t.Fatalf("unexpected claimed rules: %+v", out)
	}
}
