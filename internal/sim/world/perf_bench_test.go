package world

import (
	"path/filepath"
	"runtime"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func mustBenchmarkCatalogs(b *testing.B) *catalogs.Catalogs {
	b.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatalf("runtime caller unavailable")
	}
	cfgDir := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../configs"))
	cats, err := catalogs.Load(cfgDir)
	if err != nil {
		b.Fatalf("load catalogs: %v", err)
	}
	return cats
}

func newBenchmarkWorld(b *testing.B, id string) *World {
	b.Helper()
	cats := mustBenchmarkCatalogs(b)
	w, err := New(WorldConfig{
		ID:                     id,
		Seed:                   1337,
		BoundaryR:              64,
		ObsRadius:              7,
		BlueprintBlocksPerTick: 2,
	}, cats)
	if err != nil {
		b.Fatalf("new world: %v", err)
	}
	return w
}

func seedBenchmarkWorld(w *World) {
	stone := w.catalogs.Blocks.Index["STONE"]
	for x := -12; x <= 12; x++ {
		for z := -12; z <= 12; z++ {
			p := Vec3i{X: x, Y: 0, Z: z}
			if (x+z)%9 == 0 {
				w.chunks.SetBlock(p, stone)
			}
		}
	}
}

func BenchmarkPerfWorldStep(b *testing.B) {
	w := newBenchmarkWorld(b, "bench_step")
	seedBenchmarkWorld(w)
	out := make(chan []byte, 1)
	resp := w.joinAgent("bench-agent", false, out)
	a := w.agents[resp.Welcome.AgentID]
	if a == nil {
		b.Fatalf("joined agent missing")
	}
	a.Pos = Vec3i{X: 0, Y: 0, Z: 0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.stepInternal(nil, nil, nil, nil, nil, nil)
	}
}

func BenchmarkPerfSnapshotExport(b *testing.B) {
	w := newBenchmarkWorld(b, "bench_snap_export")
	seedBenchmarkWorld(w)
	out := make(chan []byte, 1)
	resp := w.joinAgent("bench-agent", false, out)
	a := w.agents[resp.Welcome.AgentID]
	a.Inventory["PLANK"] = 128
	a.Inventory["STONE"] = 64
	a.AddEvent(protocol.Event{"type": "BENCH", "v": "seed"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.ExportSnapshot(w.CurrentTick())
	}
}

func BenchmarkPerfSnapshotImport(b *testing.B) {
	src := newBenchmarkWorld(b, "bench_snap_src")
	seedBenchmarkWorld(src)
	out := make(chan []byte, 1)
	resp := src.joinAgent("bench-agent", false, out)
	a := src.agents[resp.Welcome.AgentID]
	a.Inventory["PLANK"] = 128
	a.Inventory["STONE"] = 64
	snap := src.ExportSnapshot(src.CurrentTick())

	dst := newBenchmarkWorld(b, "bench_snap_src")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := dst.ImportSnapshot(snap); err != nil {
			b.Fatalf("import snapshot: %v", err)
		}
	}
}

func BenchmarkPerfEventCursorQuery(b *testing.B) {
	a := &Agent{ID: "A1"}
	a.initDefaults()
	for i := 0; i < 4000; i++ {
		a.AddEvent(protocol.Event{
			"t":    i,
			"type": "BENCH",
			"seq":  i,
		})
	}
	cursor := uint64(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, next := a.EventsAfter(cursor, 128)
		if len(items) == 0 {
			cursor = 0
			continue
		}
		cursor = next
	}
}
