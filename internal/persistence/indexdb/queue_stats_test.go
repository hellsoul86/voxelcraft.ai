package indexdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/world"
)

func TestSQLiteIndex_QueueDropStats(t *testing.T) {
	s := &SQLiteIndex{ch: make(chan req, 1)}
	s.ch <- req{kind: reqTick, tick: world.TickLogEntry{Tick: 1}}

	_ = s.WriteTick(world.TickLogEntry{Tick: 2})
	_ = s.WriteAudit(world.AuditEntry{Tick: 2})
	s.RecordSnapshot("/tmp/2.snap.zst", snapshot.SnapshotV1{})
	s.RecordSnapshotState(snapshot.SnapshotV1{})
	s.RecordSeason(1, 2, "/tmp/2.snap.zst", 42)

	st := s.Stats()
	if st.DropTickTotal != 1 {
		t.Fatalf("DropTickTotal=%d want=1", st.DropTickTotal)
	}
	if st.DropAuditTotal != 1 {
		t.Fatalf("DropAuditTotal=%d want=1", st.DropAuditTotal)
	}
	if st.DropSnapshotTotal != 1 {
		t.Fatalf("DropSnapshotTotal=%d want=1", st.DropSnapshotTotal)
	}
	if st.DropSnapshotStateTotal != 1 {
		t.Fatalf("DropSnapshotStateTotal=%d want=1", st.DropSnapshotStateTotal)
	}
	if st.DropSeasonTotal != 1 {
		t.Fatalf("DropSeasonTotal=%d want=1", st.DropSeasonTotal)
	}
	if st.QueueDepth != 1 || st.QueueCapacity != 1 {
		t.Fatalf("queue stats mismatch: depth=%d cap=%d", st.QueueDepth, st.QueueCapacity)
	}
}

func TestD1Index_RetainsBatchOnFlushFailure(t *testing.T) {
	var mu sync.Mutex
	reqCount := 0
	applied := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqCount++
		thisReq := reqCount
		mu.Unlock()

		if thisReq <= 3 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}

		var body struct {
			Events []d1Event `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		applied += len(body.Events)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	idx, err := OpenD1(D1Config{
		Endpoint:      srv.URL,
		WorldID:       "world_1",
		BatchSize:     1,
		FlushInterval: 20 * time.Millisecond,
		HTTPTimeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("OpenD1: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if err := idx.WriteTick(world.TickLogEntry{Tick: 123, Digest: "abc"}); err != nil {
		t.Fatalf("WriteTick: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := applied >= 1
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	finalApplied := applied
	finalReqCount := reqCount
	mu.Unlock()

	if finalApplied < 1 {
		t.Fatalf("expected retained batch to be eventually delivered; applied=%d reqCount=%d", finalApplied, finalReqCount)
	}

	st := idx.Stats()
	if st.FlushFailTotal == 0 {
		t.Fatalf("expected flush failures to be recorded, got 0")
	}
	if st.QueueDroppedTotal != 0 {
		t.Fatalf("unexpected queue drops: %d", st.QueueDroppedTotal)
	}
}
