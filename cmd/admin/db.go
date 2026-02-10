package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func dbCmd(args []string) {
	fs := flag.NewFlagSet("db", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "runtime data directory")
	worldID := fs.String("world", "", "world id (required unless -db)")
	dbPath := fs.String("db", "", "sqlite db path (optional)")
	tick := fs.Uint64("tick", 0, "snapshot tick (optional; defaults to latest)")
	limit := fs.Int("limit", 20, "result limit")
	boardID := fs.String("board", "", "board_id filter (board_posts)")
	_ = fs.Parse(args)

	q := "snapshots"
	if fs.NArg() > 0 {
		q = strings.TrimSpace(fs.Arg(0))
	}

	path := strings.TrimSpace(*dbPath)
	if path == "" {
		if strings.TrimSpace(*worldID) == "" {
			fmt.Fprintln(os.Stderr, "missing -world or -db")
			os.Exit(2)
		}
		path = filepath.Join(*dataDir, "worlds", *worldID, "index", "world.sqlite")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer db.Close()

	if q != "snapshots" {
		if *tick == 0 {
			lt, err := latestSnapshotTick(db)
			if err != nil {
				fmt.Fprintln(os.Stderr, "latest tick:", err)
				os.Exit(1)
			}
			if lt == 0 {
				fmt.Fprintln(os.Stderr, "no snapshots found")
				os.Exit(2)
			}
			*tick = lt
		}
	}

	switch q {
	case "snapshots":
		if *limit <= 0 {
			*limit = 20
		}
		rows, err := db.Query(`SELECT tick,path,seed,height,chunks,agents,claims,containers,contracts,laws,orgs FROM snapshots ORDER BY tick DESC LIMIT ?`, *limit)
		if err != nil {
			fmt.Fprintln(os.Stderr, "query:", err)
			os.Exit(1)
		}
		defer rows.Close()
		for rows.Next() {
			var r struct {
				Tick       int64  `json:"tick"`
				Path       string `json:"path"`
				Seed       int64  `json:"seed"`
				Height     int    `json:"height"`
				Chunks     int    `json:"chunks"`
				Agents     int    `json:"agents"`
				Claims     int    `json:"claims"`
				Containers int    `json:"containers"`
				Contracts  int    `json:"contracts"`
				Laws       int    `json:"laws"`
				Orgs       int    `json:"orgs"`
			}
			if err := rows.Scan(&r.Tick, &r.Path, &r.Seed, &r.Height, &r.Chunks, &r.Agents, &r.Claims, &r.Containers, &r.Contracts, &r.Laws, &r.Orgs); err != nil {
				fmt.Fprintln(os.Stderr, "scan:", err)
				os.Exit(1)
			}
			printJSON(r)
		}
		if err := rows.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "rows:", err)
			os.Exit(1)
		}

	case "world":
		var r struct {
			Tick               uint64 `json:"tick"`
			Weather            string `json:"weather"`
			WeatherUntilTick   int64  `json:"weather_until_tick"`
			ActiveEventID      string `json:"active_event_id"`
			ActiveEventStart   int64  `json:"active_event_start_tick"`
			ActiveEventEnds    int64  `json:"active_event_ends_tick"`
			ActiveEventCenterX int    `json:"active_event_center_x"`
			ActiveEventCenterY int    `json:"active_event_center_y"`
			ActiveEventCenterZ int    `json:"active_event_center_z"`
			ActiveEventRadius  int    `json:"active_event_radius"`
		}
		row := db.QueryRow(`SELECT weather,weather_until_tick,active_event_id,active_event_start_tick,active_event_ends_tick,active_event_center_x,active_event_center_y,active_event_center_z,active_event_radius FROM snapshot_world WHERE tick=?`, *tick)
		if err := row.Scan(&r.Weather, &r.WeatherUntilTick, &r.ActiveEventID, &r.ActiveEventStart, &r.ActiveEventEnds, &r.ActiveEventCenterX, &r.ActiveEventCenterY, &r.ActiveEventCenterZ, &r.ActiveEventRadius); err != nil {
			fmt.Fprintln(os.Stderr, "scan:", err)
			os.Exit(1)
		}
		r.Tick = *tick
		printJSON(r)

	case "agents":
		rows, err := db.Query(`SELECT agent_id,name,org_id,x,y,z,yaw,hp,hunger,stamina_milli,rep_trade,rep_build,rep_social,rep_law,fun_novelty,fun_creation,fun_social,fun_influence,fun_narrative,fun_risk_rescue,inventory_json FROM snapshot_agents WHERE tick=? ORDER BY agent_id`, *tick)
		if err != nil {
			fmt.Fprintln(os.Stderr, "query:", err)
			os.Exit(1)
		}
		defer rows.Close()
		for rows.Next() {
			var r struct {
				Tick          uint64 `json:"tick"`
				AgentID       string `json:"agent_id"`
				Name          string `json:"name"`
				OrgID         string `json:"org_id,omitempty"`
				X             int    `json:"x"`
				Y             int    `json:"y"`
				Z             int    `json:"z"`
				Yaw           int    `json:"yaw"`
				HP            int    `json:"hp"`
				Hunger        int    `json:"hunger"`
				StaminaMilli  int    `json:"stamina_milli"`
				RepTrade      int    `json:"rep_trade"`
				RepBuild      int    `json:"rep_build"`
				RepSocial     int    `json:"rep_social"`
				RepLaw        int    `json:"rep_law"`
				FunNovelty    int    `json:"fun_novelty"`
				FunCreation   int    `json:"fun_creation"`
				FunSocial     int    `json:"fun_social"`
				FunInfluence  int    `json:"fun_influence"`
				FunNarrative  int    `json:"fun_narrative"`
				FunRiskRescue int    `json:"fun_risk_rescue"`
				InventoryJSON string `json:"inventory_json"`
			}
			if err := rows.Scan(
				&r.AgentID, &r.Name, &r.OrgID,
				&r.X, &r.Y, &r.Z,
				&r.Yaw, &r.HP, &r.Hunger, &r.StaminaMilli,
				&r.RepTrade, &r.RepBuild, &r.RepSocial, &r.RepLaw,
				&r.FunNovelty, &r.FunCreation, &r.FunSocial, &r.FunInfluence, &r.FunNarrative, &r.FunRiskRescue,
				&r.InventoryJSON,
			); err != nil {
				fmt.Fprintln(os.Stderr, "scan:", err)
				os.Exit(1)
			}
			r.Tick = *tick
			printJSON(r)
		}
		if err := rows.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "rows:", err)
			os.Exit(1)
		}

	case "boards":
		rows, err := db.Query(`SELECT board_id,kind,x,y,z,posts_count FROM snapshot_boards WHERE tick=? ORDER BY board_id`, *tick)
		if err != nil {
			fmt.Fprintln(os.Stderr, "query:", err)
			os.Exit(1)
		}
		defer rows.Close()
		for rows.Next() {
			var r struct {
				Tick       uint64        `json:"tick"`
				BoardID    string        `json:"board_id"`
				Kind       string        `json:"kind"`
				X          sql.NullInt64 `json:"x"`
				Y          sql.NullInt64 `json:"y"`
				Z          sql.NullInt64 `json:"z"`
				PostsCount int           `json:"posts_count"`
			}
			if err := rows.Scan(&r.BoardID, &r.Kind, &r.X, &r.Y, &r.Z, &r.PostsCount); err != nil {
				fmt.Fprintln(os.Stderr, "scan:", err)
				os.Exit(1)
			}
			r.Tick = *tick
			printJSON(r)
		}
		if err := rows.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "rows:", err)
			os.Exit(1)
		}

	case "board_posts":
		if *limit <= 0 {
			*limit = 50
		}
		q := `SELECT board_id,post_id,author,title,body,post_tick FROM snapshot_board_posts WHERE tick=? ORDER BY post_tick DESC LIMIT ?`
		args := []any{*tick, *limit}
		if strings.TrimSpace(*boardID) != "" {
			q = `SELECT board_id,post_id,author,title,body,post_tick FROM snapshot_board_posts WHERE tick=? AND board_id=? ORDER BY post_tick DESC LIMIT ?`
			args = []any{*tick, strings.TrimSpace(*boardID), *limit}
		}
		rows, err := db.Query(q, args...)
		if err != nil {
			fmt.Fprintln(os.Stderr, "query:", err)
			os.Exit(1)
		}
		defer rows.Close()
		for rows.Next() {
			var r struct {
				Tick     uint64 `json:"tick"`
				BoardID  string `json:"board_id"`
				PostID   string `json:"post_id"`
				Author   string `json:"author"`
				Title    string `json:"title"`
				Body     string `json:"body"`
				PostTick int64  `json:"post_tick"`
			}
			if err := rows.Scan(&r.BoardID, &r.PostID, &r.Author, &r.Title, &r.Body, &r.PostTick); err != nil {
				fmt.Fprintln(os.Stderr, "scan:", err)
				os.Exit(1)
			}
			r.Tick = *tick
			printJSON(r)
		}
		if err := rows.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "rows:", err)
			os.Exit(1)
		}

	case "trades":
		rows, err := db.Query(`SELECT trade_id,from_agent,to_agent,created_tick,offer_json,request_json FROM snapshot_trades WHERE tick=? ORDER BY trade_id`, *tick)
		if err != nil {
			fmt.Fprintln(os.Stderr, "query:", err)
			os.Exit(1)
		}
		defer rows.Close()
		for rows.Next() {
			var r struct {
				Tick        uint64 `json:"tick"`
				TradeID     string `json:"trade_id"`
				FromAgent   string `json:"from"`
				ToAgent     string `json:"to"`
				CreatedTick int64  `json:"created_tick"`
				OfferJSON   string `json:"offer_json"`
				RequestJSON string `json:"request_json"`
			}
			if err := rows.Scan(&r.TradeID, &r.FromAgent, &r.ToAgent, &r.CreatedTick, &r.OfferJSON, &r.RequestJSON); err != nil {
				fmt.Fprintln(os.Stderr, "scan:", err)
				os.Exit(1)
			}
			r.Tick = *tick
			printJSON(r)
		}
		if err := rows.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "rows:", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintln(os.Stderr, "unknown query:", q)
		fmt.Fprintln(os.Stderr, "usage: admin db [-data ./data] [-world WORLD|-db PATH] [-tick T] snapshots|world|agents|boards|board_posts|trades")
		os.Exit(2)
	}
}

func latestSnapshotTick(db *sql.DB) (uint64, error) {
	if db == nil {
		return 0, fmt.Errorf("nil db")
	}
	var t int64
	if err := db.QueryRow(`SELECT COALESCE(MAX(tick),0) FROM snapshots`).Scan(&t); err != nil {
		return 0, err
	}
	if t < 0 {
		return 0, nil
	}
	return uint64(t), nil
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
