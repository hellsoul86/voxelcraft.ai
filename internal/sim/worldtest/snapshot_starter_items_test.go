package worldtest

import (
	"reflect"
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestSnapshotImport_RestoresStarterItems(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := world.WorldConfig{
		ID:        "test",
		Seed:      7,
		Height:    1,
		DayTicks:  6000,
		ObsRadius: 7,
		BoundaryR: 4000,
		StarterItems: map[string]int{
			"PLANK":   1,
			"COAL":    2,
			"BERRIES": 3,
		},
	}
	w1, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}
	snap := w1.ExportSnapshot(0)

	cfg2 := cfg
	cfg2.StarterItems = map[string]int{"PLANK": 99}
	w2, err := world.New(cfg2, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}

	if got := w2.Config().StarterItems; !reflect.DeepEqual(got, cfg.StarterItems) {
		t.Fatalf("StarterItems mismatch: got=%v want=%v", got, cfg.StarterItems)
	}
}
