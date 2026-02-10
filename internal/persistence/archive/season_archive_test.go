package archive

import (
	"os"
	"path/filepath"
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func TestArchiveSeasonSnapshot_CopiesSeasonEndSnapshot(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "worlds", "w1")
	if err := os.MkdirAll(worldDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a dummy snapshot file.
	src := filepath.Join(worldDir, "snapshots", "2.snap.zst")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir snapshots: %v", err)
	}
	want := []byte("dummy")
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	snap := snapshot.SnapshotV1{
		Header:            snapshot.Header{Version: 1, WorldID: "w1", Tick: 2},
		Seed:              42,
		DayTicks:          3,
		SeasonLengthTicks: 3,
	}

	season, archivedPath, ok, err := ArchiveSeasonSnapshot(worldDir, src, snap)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !ok {
		t.Fatalf("expected archived=true")
	}
	if season != 1 {
		t.Fatalf("season=%d want 1", season)
	}

	got, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("read archived: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("archived content mismatch: got=%q want=%q", string(got), string(want))
	}

	metaPath := filepath.Join(filepath.Dir(archivedPath), "meta.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("expected meta.json to exist: %v", err)
	}
}
