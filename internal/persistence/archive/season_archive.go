package archive

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"voxelcraft.ai/internal/persistence/snapshot"
)

type SeasonArchiveMeta struct {
	Season      int    `json:"season"`
	EndTick     uint64 `json:"end_tick"`
	Seed        int64  `json:"seed"`
	Snapshot    string `json:"snapshot"`
	CreatedAt   string `json:"created_at"`
	DayTicks    int    `json:"day_ticks"`
	SeasonTicks int    `json:"season_length_ticks"`
}

// ArchiveSeasonSnapshot copies a season-end snapshot into `worldDir/archives/season_<NNN>/`.
// It returns (season, archivedPath, archived=true) when the snapshot represents a season end.
func ArchiveSeasonSnapshot(worldDir, snapshotPath string, snap snapshot.SnapshotV1) (season int, archivedPath string, archived bool, err error) {
	if snap.SeasonLengthTicks <= 0 {
		return 0, "", false, nil
	}
	seasonLen := uint64(snap.SeasonLengthTicks)
	if seasonLen == 0 {
		return 0, "", false, nil
	}
	// Snapshots represent the last executed tick. Season boundaries happen at tick multiples, so
	// the season-end snapshot is at tick = seasonLen*k - 1.
	if (snap.Header.Tick+1)%seasonLen != 0 {
		return 0, "", false, nil
	}
	season = int((snap.Header.Tick + 1) / seasonLen)
	if season <= 0 {
		return 0, "", false, nil
	}

	archiveDir := filepath.Join(worldDir, "archives", fmt.Sprintf("season_%03d", season))
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return 0, "", false, err
	}

	dst := filepath.Join(archiveDir, filepath.Base(snapshotPath))
	if err := copyFile(snapshotPath, dst); err != nil {
		return 0, "", false, err
	}

	meta := SeasonArchiveMeta{
		Season:      season,
		EndTick:     snap.Header.Tick,
		Seed:        snap.Seed,
		Snapshot:    filepath.Base(dst),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		DayTicks:    snap.DayTicks,
		SeasonTicks: snap.SeasonLengthTicks,
	}
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(archiveDir, "meta.json"), b, 0o644)
	}

	return season, dst, true, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
