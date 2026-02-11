package multiworld

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
)

func TestManagerOrgMutationTriggersRefresh(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 61)
	wMine := newTestWorldForManager(t, "MINE_L1", 62)
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
		t.Fatalf("join creator: %v", err)
	}
	obs := waitObsMsg(t, out, 3*time.Second)
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_CREATE_ORG", Type: "CREATE_ORG", OrgKind: "GUILD", OrgName: "TriggerGuild"},
		},
	})
	if err != nil {
		t.Fatalf("route create org: %v", err)
	}
	ev, _ := waitActionResult(t, out, "I_CREATE_ORG", 3*time.Second)
	orgID, _ := ev["org_id"].(string)
	if orgID == "" {
		t.Fatalf("missing org_id from create org result: %+v", ev)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mgr.mu.RLock()
		_, ok := mgr.globalOrgMeta[orgID]
		mgr.mu.RUnlock()
		if ok {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("expected manager globalOrgMeta to be refreshed for %s", orgID)
}
