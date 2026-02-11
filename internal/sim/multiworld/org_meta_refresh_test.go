package multiworld

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
)

func TestManagerRefreshOrgMeta_PullsWithoutWorldSwitch(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 41)
	wMine := newTestWorldForManager(t, "MINE_L1", 42)
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

	out := make(chan []byte, 256)
	sess, _, err := mgr.Join("creator", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	obs := waitObsMsg(t, out, 3*time.Second)
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_CREATE_ORG", Type: "CREATE_ORG", OrgKind: "GUILD", OrgName: "Refreshed Guild"},
		},
	})
	if err != nil {
		t.Fatalf("route act create org: %v", err)
	}
	ev, _ := waitActionResult(t, out, "I_CREATE_ORG", 3*time.Second)
	orgID, _ := ev["org_id"].(string)
	if orgID == "" {
		t.Fatalf("missing org_id in create org result: %+v", ev)
	}

	if _, ok := mgr.globalOrgMeta[orgID]; ok {
		t.Fatalf("org metadata should not exist before refresh")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if err := mgr.RefreshOrgMeta(ctx2); err != nil {
		t.Fatalf("refresh org meta: %v", err)
	}

	meta, ok := mgr.globalOrgMeta[orgID]
	if !ok {
		t.Fatalf("org metadata missing after refresh")
	}
	if meta.Name != "Refreshed Guild" {
		t.Fatalf("org name mismatch after refresh: %q", meta.Name)
	}
	if role := meta.Members[sess.AgentID]; role != "LEADER" {
		t.Fatalf("creator role mismatch after refresh: %q", role)
	}
}
