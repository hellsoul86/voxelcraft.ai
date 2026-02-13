package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"voxelcraft.ai/internal/persistence/indexdb"
	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tuning"
	"voxelcraft.ai/internal/sim/world"
)

type runtimeIndex interface {
	world.TickLogger
	world.AuditLogger
	Close() error
	UpsertCatalogs(configDir string, cats *catalogs.Catalogs, tune tuning.Tuning) error
	RecordSnapshot(path string, snap snapshot.SnapshotV1)
	RecordSnapshotState(snap snapshot.SnapshotV1)
	RecordSeason(season int, endTick uint64, archivedSnapshotPath string, seed int64)
}

func openRuntimeIndex(worldDir, worldID string, disableDB bool, logger *log.Logger) (runtimeIndex, error) {
	if disableDB {
		return nil, nil
	}

	backend := strings.ToLower(strings.TrimSpace(os.Getenv("VC_INDEX_BACKEND")))
	if backend == "" {
		backend = "sqlite"
	}

	switch backend {
	case "none", "off", "disabled":
		return nil, nil
	case "sqlite":
		dbPath := filepath.Join(worldDir, "index", "world.sqlite")
		return indexdb.OpenSQLite(dbPath)
	case "d1":
		endpoint := strings.TrimSpace(os.Getenv("VC_INDEX_D1_INGEST_URL"))
		token := strings.TrimSpace(os.Getenv("VC_INDEX_D1_TOKEN"))
		if endpoint == "" {
			return nil, fmt.Errorf("VC_INDEX_BACKEND=d1 but VC_INDEX_D1_INGEST_URL is empty")
		}
		flushMS := envInt("VC_INDEX_D1_FLUSH_MS", 500)
		batchSize := envInt("VC_INDEX_D1_BATCH_SIZE", 128)
		idx, err := indexdb.OpenD1(indexdb.D1Config{
			Endpoint:      endpoint,
			Token:         token,
			WorldID:       worldID,
			BatchSize:     batchSize,
			FlushInterval: time.Duration(flushMS) * time.Millisecond,
			Logger:        logger,
		})
		if err != nil {
			return nil, err
		}
		return idx, nil
	default:
		return nil, fmt.Errorf("unsupported VC_INDEX_BACKEND: %s", backend)
	}
}
