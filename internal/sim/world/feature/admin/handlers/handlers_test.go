package handlers

import "testing"

func TestHandleSnapshot(t *testing.T) {
	ok := HandleSnapshot(SnapshotInput{CurrentTick: 10, HasSink: false})
	if ok.Err == "" {
		t.Fatalf("expected missing sink error")
	}

	enqueued := false
	resp := HandleSnapshot(SnapshotInput{
		CurrentTick: 10,
		HasSink:     true,
		Enqueue: func(snapshotTick uint64) bool {
			enqueued = snapshotTick == 9
			return true
		},
	})
	if resp.Err != "" || !enqueued || resp.Tick != 9 {
		t.Fatalf("snapshot response mismatch: %+v enqueued=%v", resp, enqueued)
	}
}

func TestHandleReset(t *testing.T) {
	called := false
	resp := HandleReset(ResetInput{
		CurrentTick: 20,
		HasSink:     true,
		Enqueue: func(archiveTick uint64) bool {
			return archiveTick == 19
		},
		OnReset: func(curTick uint64, archiveTick uint64) {
			called = curTick == 20 && archiveTick == 19
		},
	})
	if resp.Err != "" || !called || resp.Tick != 20 {
		t.Fatalf("reset response mismatch: %+v called=%v", resp, called)
	}
}
