package indexdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteIndex_RecordSeason(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.db")

	idx, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	idx.RecordSeason(1, 5999, "/abs/path/5999.snap.zst", 42)
	if err := idx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	var (
		season int
		end    int64
		seed   int64
		snap   string
	)
	row := db.QueryRow(`SELECT season,end_tick,seed,snapshot_path FROM seasons WHERE season=1`)
	if err := row.Scan(&season, &end, &seed, &snap); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if season != 1 || end != 5999 || seed != 42 || snap != "/abs/path/5999.snap.zst" {
		t.Fatalf("row mismatch: season=%d end=%d seed=%d snap=%q", season, end, seed, snap)
	}
}
