package indexdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func TestSQLiteIndex_RecordSnapshotState_WritesStateTables(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "world.sqlite")

	idx, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	snap := snapshot.SnapshotV1{
		Header:           snapshot.Header{Version: 1, WorldID: "w1", Tick: 123},
		Weather:          "CLEAR",
		WeatherUntilTick: 200,
		ActiveEventID:    "MARKET_WEEK",
		ActiveEventStart: 100,
		ActiveEventEnds:  300,
		ActiveEventCenter: [3]int{
			10, 20, 30,
		},
		ActiveEventRadius: 55,
		Agents: []snapshot.AgentV1{
			{
				ID:           "A1",
				Name:         "Alice",
				OrgID:        "ORG1",
				Pos:          [3]int{1, 22, 3},
				Yaw:          90,
				HP:           20,
				Hunger:       10,
				StaminaMilli: 900,
				RepTrade:     500,
				RepBuild:     500,
				RepSocial:    500,
				RepLaw:       500,
				FunNovelty:   10,
				FunSocial:    2,
				Inventory:    map[string]int{"PLANK": 3},
			},
		},
		Trades: []snapshot.TradeV1{
			{
				TradeID:     "T1",
				From:        "A1",
				To:          "A2",
				Offer:       map[string]int{"PLANK": 10},
				Request:     map[string]int{"IRON_INGOT": 10},
				CreatedTick: 120,
			},
		},
		Boards: []snapshot.BoardV1{
			{
				BoardID: "BULLETIN_BOARD@1,22,3",
				Posts: []snapshot.BoardPostV1{
					{PostID: "P1", Author: "A1", Title: "Hello", Body: "world", Tick: 121},
				},
			},
			{
				BoardID: "BOARD_MARKET",
				Posts: []snapshot.BoardPostV1{
					{PostID: "P2", Author: "A1", Title: "WTS", Body: "10 planks for 10 iron", Tick: 122},
					{PostID: "P3", Author: "A2", Title: "WTB", Body: "need coal", Tick: 123},
				},
			},
		},
		Claims: []snapshot.ClaimV1{
			{
				LandID:             "LAND1",
				Owner:              "A1",
				Anchor:             [3]int{1, 22, 3},
				Radius:             32,
				Flags:              snapshot.ClaimFlagsV1{AllowBuild: true, AllowBreak: false, AllowDamage: false, AllowTrade: true},
				Members:            []string{"A2"},
				MarketTax:          0.05,
				MaintenanceDueTick: 999,
				MaintenanceStage:   0,
			},
		},
		Orgs: []snapshot.OrgV1{
			{
				OrgID:       "ORG1",
				Kind:        "CITY",
				Name:        "TestCity",
				CreatedTick: 1,
				Members:     map[string]string{"A1": "OWNER"},
				Treasury:    map[string]int{"IRON_INGOT": 5},
			},
		},
		Contracts: []snapshot.ContractV1{
			{
				ContractID:   "C1",
				TerminalPos:  [3]int{1, 22, 3},
				Poster:       "A1",
				Acceptor:     "",
				Kind:         "GATHER",
				State:        "OPEN",
				Requirements: map[string]int{"STONE": 1},
				Reward:       map[string]int{"PLANK": 2},
				Deposit:      map[string]int{},
				BlueprintID:  "",
				Anchor:       [3]int{0, 0, 0},
				Rotation:     0,
				CreatedTick:  10,
				DeadlineTick: 20,
			},
		},
		Laws: []snapshot.LawV1{
			{
				LawID:          "LAW1",
				LandID:         "LAND1",
				TemplateID:     "MARKET_TAX",
				Title:          "Tax",
				Params:         map[string]string{"market_tax": "0.05"},
				Status:         "ACTIVE",
				ProposedBy:     "A1",
				ProposedTick:   1,
				NoticeEndsTick: 2,
				VoteEndsTick:   3,
				Votes:          map[string]string{"A1": "YES"},
			},
		},
	}

	idx.RecordSnapshotState(snap)
	if err := idx.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sql: %v", err)
	}
	defer db.Close()

	type check struct {
		table string
		want  int
	}
	checks := []check{
		{table: "snapshot_world", want: 1},
		{table: "snapshot_agents", want: 1},
		{table: "snapshot_trades", want: 1},
		{table: "snapshot_boards", want: 2},
		{table: "snapshot_board_posts", want: 3},
		{table: "snapshot_claims", want: 1},
		{table: "snapshot_orgs", want: 1},
		{table: "snapshot_contracts", want: 1},
		{table: "snapshot_laws", want: 1},
	}
	for _, c := range checks {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM `+c.table+` WHERE tick = ?`, 123).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", c.table, err)
		}
		if n != c.want {
			t.Fatalf("table %s count=%d want %d", c.table, n, c.want)
		}
	}

	{
		var (
			weather    string
			until      int64
			activeID   string
			startTick  int64
			endsTick   int64
			cx, cy, cz int
			radius     int
		)
		row := db.QueryRow(`SELECT weather,weather_until_tick,active_event_id,active_event_start_tick,active_event_ends_tick,active_event_center_x,active_event_center_y,active_event_center_z,active_event_radius FROM snapshot_world WHERE tick = ?`, 123)
		if err := row.Scan(&weather, &until, &activeID, &startTick, &endsTick, &cx, &cy, &cz, &radius); err != nil {
			t.Fatalf("scan snapshot_world: %v", err)
		}
		if weather != "CLEAR" || until != 200 || activeID != "MARKET_WEEK" || startTick != 100 || endsTick != 300 || cx != 10 || cy != 20 || cz != 30 || radius != 55 {
			t.Fatalf("snapshot_world mismatch: weather=%q until=%d event=%q start=%d ends=%d center=%d,%d,%d radius=%d", weather, until, activeID, startTick, endsTick, cx, cy, cz, radius)
		}
	}
	{
		var (
			kind string
			x    sql.NullInt64
			y    sql.NullInt64
			z    sql.NullInt64
		)
		row := db.QueryRow(`SELECT kind,x,y,z FROM snapshot_boards WHERE tick = ? AND board_id = ?`, 123, "BULLETIN_BOARD@1,22,3")
		if err := row.Scan(&kind, &x, &y, &z); err != nil {
			t.Fatalf("scan snapshot_boards: %v", err)
		}
		if kind != "PHYSICAL" || !x.Valid || !y.Valid || !z.Valid || x.Int64 != 1 || y.Int64 != 22 || z.Int64 != 3 {
			t.Fatalf("physical board mismatch: kind=%q x=%v y=%v z=%v", kind, x, y, z)
		}
	}
}
