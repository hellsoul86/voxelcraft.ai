package multiworld

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
)

func TestManagerRefreshOrgMeta_AllowsCrossWorldJoinWithoutSwitch(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 51)
	wMine := newTestWorldForManager(t, "MINE_L1", 52)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

	cfg := testManagerConfig()
	mgr, err := NewManager(cfg, map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	overOut := make(chan []byte, 256)
	overSess, _, err := mgr.Join("leader", true, overOut, "OVERWORLD")
	if err != nil {
		t.Fatalf("join leader: %v", err)
	}
	overObs := waitObsMsg(t, overOut, 3*time.Second)
	_, err = mgr.RouteAct(context.Background(), &overSess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            overObs.Tick,
		AgentID:         overSess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_CREATE", Type: "CREATE_ORG", OrgKind: "GUILD", OrgName: "GlobalGuild"},
		},
	})
	if err != nil {
		t.Fatalf("route create org: %v", err)
	}
	evCreate, _ := waitActionResult(t, overOut, "I_CREATE", 3*time.Second)
	if ok, _ := evCreate["ok"].(bool); !ok {
		t.Fatalf("create org failed: %+v", evCreate)
	}
	orgID, _ := evCreate["org_id"].(string)
	if orgID == "" {
		t.Fatalf("missing org_id in create result: %+v", evCreate)
	}

	// Pull and propagate global org metadata to all worlds.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if err := mgr.RefreshOrgMeta(ctx2); err != nil {
		t.Fatalf("refresh org meta: %v", err)
	}

	mineOut := make(chan []byte, 256)
	mineSess, _, err := mgr.Join("member", true, mineOut, "MINE_L1")
	if err != nil {
		t.Fatalf("join member: %v", err)
	}
	mineObs := waitObsMsg(t, mineOut, 3*time.Second)
	_, err = mgr.RouteAct(context.Background(), &mineSess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            mineObs.Tick,
		AgentID:         mineSess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_JOIN_GLOBAL", Type: "JOIN_ORG", OrgID: orgID},
		},
	})
	if err != nil {
		t.Fatalf("route join org: %v", err)
	}
	evJoin, _ := waitActionResult(t, mineOut, "I_JOIN_GLOBAL", 3*time.Second)
	if ok, _ := evJoin["ok"].(bool); !ok {
		t.Fatalf("cross-world join failed: %+v", evJoin)
	}
}
