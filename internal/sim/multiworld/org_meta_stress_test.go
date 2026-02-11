package multiworld

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world"
)

func orgMetaByID(t *testing.T, mgr *Manager, orgID string) OrgMeta {
	t.Helper()
	for _, tr := range mgr.snapshotOrgTransfers() {
		if tr.OrgID != orgID {
			continue
		}
		meta := OrgMeta{
			OrgID:       tr.OrgID,
			Kind:        tr.Kind,
			Name:        tr.Name,
			CreatedTick: tr.CreatedTick,
			MetaVersion: tr.MetaVersion,
			Members:     map[string]world.OrgRole{},
		}
		for aid, role := range tr.Members {
			meta.Members[aid] = role
		}
		return meta
	}
	t.Fatalf("org %s missing in manager snapshot", orgID)
	return OrgMeta{}
}

func waitActionResultForRefs(out <-chan []byte, timeout time.Duration, refs ...string) (protocol.Event, error) {
	refSet := map[string]bool{}
	for _, r := range refs {
		refSet[r] = true
	}
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting ACTION_RESULT refs=%v", refs)
		case b := <-out:
			var base protocol.BaseMessage
			if err := json.Unmarshal(b, &base); err != nil || base.Type != protocol.TypeObs {
				continue
			}
			var obs protocol.ObsMsg
			if err := json.Unmarshal(b, &obs); err != nil {
				continue
			}
			for _, ev := range obs.Events {
				typ, _ := ev["type"].(string)
				ref, _ := ev["ref"].(string)
				if typ == "ACTION_RESULT" && refSet[ref] {
					return ev, nil
				}
			}
		}
	}
}

func sendInstant(mgr *Manager, sess *Session, out chan []byte, ref string, inst protocol.InstantReq) (protocol.Event, error) {
	for attempt := 0; attempt < 3; attempt++ {
		rt := mgr.runtime(sess.CurrentWorld)
		if rt == nil || rt.World == nil {
			return nil, fmt.Errorf("world runtime missing: %s", sess.CurrentWorld)
		}
		actTick := rt.World.CurrentTick()
		_, err := mgr.RouteAct(context.Background(), sess, protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            actTick,
			AgentID:         sess.AgentID,
			Instants: []protocol.InstantReq{
				{
					ID:            ref,
					Type:          inst.Type,
					OrgID:         inst.OrgID,
					OrgKind:       inst.OrgKind,
					OrgName:       inst.OrgName,
					TargetWorldID: inst.TargetWorldID,
					EntryPointID:  inst.EntryPointID,
				},
			},
		})
		if err != nil {
			return nil, err
		}
		ev, err := waitActionResultForRefs(out, 3*time.Second, ref, "ACT")
		if err != nil {
			return nil, err
		}
		gotRef, _ := ev["ref"].(string)
		if gotRef == "ACT" {
			code, _ := ev["code"].(string)
			if code == "E_STALE" {
				continue
			}
			return nil, fmt.Errorf("unexpected ACT-level failure: %+v", ev)
		}
		if ok, _ := ev["ok"].(bool); !ok {
			return nil, fmt.Errorf("action failed: %+v", ev)
		}
		return ev, nil
	}
	return nil, fmt.Errorf("action %s exhausted retries", ref)
}

func TestOrgMeta_ConcurrentJoinLeaveConvergesAndVersionMonotonic(t *testing.T) {
	runtimes, stop := testRuntimes(t)
	defer stop()

	cfg := testManagerConfig()
	mgr, err := NewManager(cfg, runtimes, filepathJoinTemp(t, "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	leaderOut := make(chan []byte, 256)
	leaderSess, _, err := mgr.Join("leader", true, leaderOut, "OVERWORLD")
	if err != nil {
		t.Fatalf("join leader: %v", err)
	}
	evCreate, err := sendInstant(mgr, &leaderSess, leaderOut, "I_CREATE_STRESS", protocol.InstantReq{
		Type:    "CREATE_ORG",
		OrgKind: "GUILD",
		OrgName: "StressGuild",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	orgID, _ := evCreate["org_id"].(string)
	if orgID == "" {
		t.Fatalf("missing org_id in create event: %+v", evCreate)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := mgr.RefreshOrgMeta(ctx); err != nil {
		cancel()
		t.Fatalf("refresh org meta after create: %v", err)
	}
	cancel()
	v0 := orgMetaByID(t, mgr, orgID).MetaVersion

	minerOut := make(chan []byte, 256)
	minerSess, _, err := mgr.Join("miner", true, minerOut, "OVERWORLD")
	if err != nil {
		t.Fatalf("join miner: %v", err)
	}
	traderOut := make(chan []byte, 256)
	traderSess, _, err := mgr.Join("trader", true, traderOut, "OVERWORLD")
	if err != nil {
		t.Fatalf("join trader: %v", err)
	}
	if _, err := sendInstant(mgr, &minerSess, minerOut, "I_SWITCH_MINER_L1", protocol.InstantReq{
		Type:          "SWITCH_WORLD",
		TargetWorldID: "MINE_L1",
	}); err != nil {
		t.Fatalf("switch miner to mine_l1: %v", err)
	}

	stageJoin := []struct {
		name string
		sess *Session
		out  chan []byte
		ref  string
	}{
		{name: "miner", sess: &minerSess, out: minerOut, ref: "I_JOIN_MINER"},
		{name: "trader", sess: &traderSess, out: traderOut, ref: "I_JOIN_TRADER"},
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(stageJoin))
	for _, item := range stageJoin {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := sendInstant(mgr, item.sess, item.out, item.ref, protocol.InstantReq{
				Type:  "JOIN_ORG",
				OrgID: orgID,
			}); err != nil {
				errCh <- fmt.Errorf("%s: %w", item.ref, err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent join failed: %v", err)
		}
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	if err := mgr.RefreshOrgMeta(ctx); err != nil {
		cancel()
		t.Fatalf("refresh org meta after concurrent joins: %v", err)
	}
	cancel()
	metaJoin := orgMetaByID(t, mgr, orgID)
	if metaJoin.MetaVersion <= v0 {
		t.Fatalf("meta version should increase after joins: before=%d after=%d", v0, metaJoin.MetaVersion)
	}
	if len(metaJoin.Members) < 3 {
		t.Fatalf("expected at least 3 members after joins, got=%d members=%+v", len(metaJoin.Members), metaJoin.Members)
	}

	if _, err := sendInstant(mgr, &minerSess, minerOut, "I_LEAVE_MINER", protocol.InstantReq{Type: "LEAVE_ORG"}); err != nil {
		t.Fatalf("leave miner: %v", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	if err := mgr.RefreshOrgMeta(ctx); err != nil {
		cancel()
		t.Fatalf("refresh org meta after miner leave: %v", err)
	}
	cancel()
	if _, err := sendInstant(mgr, &traderSess, traderOut, "I_LEAVE_TRADER", protocol.InstantReq{Type: "LEAVE_ORG"}); err != nil {
		t.Fatalf("leave trader: %v", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	if err := mgr.RefreshOrgMeta(ctx); err != nil {
		cancel()
		t.Fatalf("refresh org meta after trader leave: %v", err)
	}
	cancel()

	metaLeave := orgMetaByID(t, mgr, orgID)
	if metaLeave.MetaVersion <= metaJoin.MetaVersion {
		t.Fatalf("meta version should increase after leaves: before=%d after=%d", metaJoin.MetaVersion, metaLeave.MetaVersion)
	}
	if len(metaLeave.Members) != 1 {
		t.Fatalf("expected only leader member after leaves, got=%d members=%+v", len(metaLeave.Members), metaLeave.Members)
	}

	wg = sync.WaitGroup{}
	errCh = make(chan error, len(stageJoin))
	for _, item := range stageJoin {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := sendInstant(mgr, item.sess, item.out, item.ref+"_AGAIN", protocol.InstantReq{
				Type:  "JOIN_ORG",
				OrgID: orgID,
			}); err != nil {
				errCh <- fmt.Errorf("%s: %w", item.ref+"_AGAIN", err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent rejoin failed: %v", err)
		}
	}
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	if err := mgr.RefreshOrgMeta(ctx); err != nil {
		cancel()
		t.Fatalf("refresh org meta after re-joins: %v", err)
	}
	cancel()
	metaRejoin := orgMetaByID(t, mgr, orgID)
	if metaRejoin.MetaVersion <= metaLeave.MetaVersion {
		t.Fatalf("meta version should increase after rejoin: before=%d after=%d", metaLeave.MetaVersion, metaRejoin.MetaVersion)
	}
	if len(metaRejoin.Members) < 3 {
		t.Fatalf("expected >=3 members after rejoin, got=%d members=%+v", len(metaRejoin.Members), metaRejoin.Members)
	}

	for worldID, rt := range runtimes {
		ctxWorld, cancelWorld := context.WithTimeout(context.Background(), 2*time.Second)
		orgs, err := rt.World.RequestOrgMetaSnapshot(ctxWorld)
		cancelWorld()
		if err != nil {
			t.Fatalf("world %s org snapshot: %v", worldID, err)
		}
		var found *world.OrgTransfer
		for i := range orgs {
			if orgs[i].OrgID == orgID {
				found = &orgs[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("world %s missing org %s after convergence", worldID, orgID)
		}
		if found.MetaVersion != metaRejoin.MetaVersion {
			t.Fatalf("world %s meta_version mismatch: got=%d want=%d", worldID, found.MetaVersion, metaRejoin.MetaVersion)
		}
		if len(found.Members) != len(metaRejoin.Members) {
			t.Fatalf("world %s member count mismatch: got=%d want=%d", worldID, len(found.Members), len(metaRejoin.Members))
		}
		for aid, role := range metaRejoin.Members {
			if found.Members[aid] != role {
				t.Fatalf("world %s member role mismatch aid=%s got=%s want=%s", worldID, aid, found.Members[aid], role)
			}
		}
	}
}
