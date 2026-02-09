package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world"
)

func main() {
	var (
		snapPath  = flag.String("snapshot", "", "path to .snap.zst")
		eventsDir = flag.String("events", "", "events dir containing events-*.jsonl.zst (optional)")
		configDir = flag.String("configs", "./configs", "config directory")
		fromTick  = flag.Uint64("from_tick", 0, "start verifying from tick (inclusive, optional)")
		toTick    = flag.Uint64("to_tick", 0, "stop at tick (inclusive, optional)")
	)
	flag.Parse()

	if *snapPath == "" {
		fmt.Fprintln(os.Stderr, "missing -snapshot")
		os.Exit(2)
	}

	snap, err := snapshot.ReadSnapshot(*snapPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read snapshot:", err)
		os.Exit(1)
	}

	fmt.Printf("snapshot v%d world=%s tick=%d seed=%d height=%d chunks=%d agents=%d claims=%d containers=%d contracts=%d laws=%d orgs=%d\n",
		snap.Header.Version, snap.Header.WorldID, snap.Header.Tick, snap.Seed, snap.Height,
		len(snap.Chunks), len(snap.Agents), len(snap.Claims), len(snap.Containers), len(snap.Contracts), len(snap.Laws), len(snap.Orgs))

	if *eventsDir == "" {
		return
	}

	cats, err := catalogs.Load(*configDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load catalogs:", err)
		os.Exit(1)
	}

	w, err := world.New(world.WorldConfig{
		ID:         snap.Header.WorldID,
		TickRateHz: snap.TickRate,
		DayTicks:   snap.DayTicks,
		ObsRadius:  snap.ObsRadius,
		Height:     snap.Height,
		Seed:       snap.Seed,
		BoundaryR:  snap.BoundaryR,
	}, cats)
	if err != nil {
		fmt.Fprintln(os.Stderr, "world:", err)
		os.Exit(1)
	}
	if err := w.ImportSnapshot(snap); err != nil {
		fmt.Fprintln(os.Stderr, "import snapshot:", err)
		os.Exit(1)
	}

	startTick := w.CurrentTick()
	verifyFrom := *fromTick
	if verifyFrom == 0 {
		verifyFrom = startTick
	}

	files, err := listEventFiles(*eventsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list events:", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no events files found in", *eventsDir)
		os.Exit(1)
	}

	var checked uint64
	for _, path := range files {
		if err := replayFile(w, path, startTick, verifyFrom, *toTick, &checked); err != nil {
			fmt.Fprintln(os.Stderr, "replay:", err)
			os.Exit(1)
		}
		if *toTick != 0 && w.CurrentTick() > *toTick {
			break
		}
	}
	fmt.Printf("replay ok: checked=%d ticks (from snapshot tick=%d)\n", checked, snap.Header.Tick)
}

func listEventFiles(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "events-") && strings.HasSuffix(name, ".jsonl.zst") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.Join(dir, name))
	}
	return out, nil
}

func replayFile(w *world.World, path string, startTick, verifyFrom, toTick uint64, checked *uint64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dec, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer dec.Close()

	sc := bufio.NewScanner(dec)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)

	for sc.Scan() {
		line := sc.Bytes()
		var entry world.TickLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return fmt.Errorf("%s: unmarshal: %w", filepath.Base(path), err)
		}
		if entry.Tick < startTick {
			continue
		}
		if toTick != 0 && entry.Tick > toTick {
			return nil
		}
		if entry.Tick != w.CurrentTick() {
			return fmt.Errorf("tick mismatch: want=%d got=%d (file=%s)", w.CurrentTick(), entry.Tick, filepath.Base(path))
		}

		joins := make([]world.JoinRequest, 0, len(entry.Joins))
		for _, j := range entry.Joins {
			joins = append(joins, world.JoinRequest{Name: j.Name})
		}
		leaves := entry.Leaves

		acts := make([]world.ActionEnvelope, 0, len(entry.Actions))
		for _, ra := range entry.Actions {
			acts = append(acts, world.ActionEnvelope{AgentID: ra.AgentID, Act: ra.Act})
		}

		tick, gotDigest := w.StepOnce(joins, leaves, acts)

		// Sanity check: StepOnce should have stepped the same tick.
		if tick != entry.Tick {
			return fmt.Errorf("internal tick mismatch: stepped=%d entry=%d (file=%s)", tick, entry.Tick, filepath.Base(path))
		}

		if tick >= verifyFrom {
			*checked++
			if gotDigest != entry.Digest {
				return fmt.Errorf("digest mismatch at tick %d: got=%s want=%s", tick, gotDigest, entry.Digest)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}
