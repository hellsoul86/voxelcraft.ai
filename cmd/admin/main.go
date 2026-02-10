package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/world"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "rollback":
			rollbackCmd(os.Args[2:])
			return
		case "db":
			dbCmd(os.Args[2:])
			return
		case "state":
			stateCmd(os.Args[2:])
			return
		case "snapshot":
			snapshotCmd(os.Args[2:])
			return
		}
	}
	listCmd(os.Args[1:])
}

func listCmd(args []string) {
	fs := flag.NewFlagSet("admin", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "runtime data directory")
	worldID := fs.String("world", "", "world id (optional)")
	_ = fs.Parse(args)

	base := filepath.Join(*dataDir, "worlds")
	if *worldID != "" {
		base = filepath.Join(base, *worldID)
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	for _, e := range entries {
		fmt.Println(e.Name())
	}
}

func rollbackCmd(args []string) {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "runtime data directory")
	worldID := fs.String("world", "", "world id")
	snapPath := fs.String("snapshot", "", "snapshot path to rollback from (optional; defaults to latest)")
	aabb := fs.String("aabb", "", "AABB filter: x1,y1,z1:x2,y2,z2 (required)")
	sinceTick := fs.Uint64("since_tick", 0, "rollback changes since tick (inclusive)")
	toTick := fs.Uint64("to_tick", 0, "rollback changes up to tick (inclusive, optional; defaults to snapshot tick)")
	outPath := fs.String("out", "", "output snapshot path (optional)")
	onlyIllegal := fs.Bool("only_illegal", false, "rollback only illegal operations (unsupported in v0.9; illegal edits are rejected before audit)")
	_ = fs.Parse(args)

	if strings.TrimSpace(*worldID) == "" {
		fmt.Fprintln(os.Stderr, "missing -world")
		os.Exit(2)
	}
	if strings.TrimSpace(*aabb) == "" {
		fmt.Fprintln(os.Stderr, "missing -aabb")
		os.Exit(2)
	}
	if *onlyIllegal {
		fmt.Fprintln(os.Stderr, "error: -only_illegal is not supported in v0.9 (illegal edits are rejected before audit logging)")
		os.Exit(2)
	}

	worldDir := filepath.Join(*dataDir, "worlds", *worldID)
	snapshotToLoad := strings.TrimSpace(*snapPath)
	if snapshotToLoad == "" {
		snapshotToLoad = latestSnapshot(worldDir)
	}
	if snapshotToLoad == "" {
		fmt.Fprintln(os.Stderr, "no snapshot found; provide -snapshot or run server until it writes one")
		os.Exit(2)
	}

	snap, err := snapshot.ReadSnapshot(snapshotToLoad)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read snapshot:", err)
		os.Exit(1)
	}

	min, max, err := parseAABB(*aabb)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad -aabb:", err)
		os.Exit(2)
	}

	endTick := *toTick
	if endTick == 0 || endTick > snap.Header.Tick {
		endTick = snap.Header.Tick
	}

	recs, err := readAudit(worldDir, *sinceTick, endTick, min, max)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read audit:", err)
		os.Exit(1)
	}
	if len(recs) == 0 {
		fmt.Println("no matching audit entries; nothing to rollback")
		return
	}

	applied, skipped := applyRollback(&snap, recs)

	if strings.TrimSpace(*outPath) == "" {
		*outPath = filepath.Join(worldDir, "snapshots", fmt.Sprintf("%d.rollback.snap.zst", snap.Header.Tick))
	}
	if err := snapshot.WriteSnapshot(*outPath, snap); err != nil {
		fmt.Fprintln(os.Stderr, "write snapshot:", err)
		os.Exit(1)
	}

	fmt.Printf("rollback ok: snapshot=%s tick=%d aabb=%s since=%d to=%d entries=%d applied=%d skipped=%d out=%s\n",
		filepath.Base(snapshotToLoad), snap.Header.Tick, *aabb, *sinceTick, endTick, len(recs), applied, skipped, *outPath)
}

type auditRec struct {
	Seq   uint64
	Entry world.AuditEntry
}

func readAudit(worldDir string, sinceTick, toTick uint64, min, max [3]int) ([]auditRec, error) {
	dir := filepath.Join(worldDir, "audit")
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
		if strings.HasPrefix(name, "audit-") && strings.HasSuffix(name, ".jsonl.zst") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	out := make([]auditRec, 0, 1024)
	var seq uint64

	for _, name := range names {
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		dec, err := zstd.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		sc := bufio.NewScanner(dec)
		sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
		for sc.Scan() {
			var e world.AuditEntry
			if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
				dec.Close()
				_ = f.Close()
				return nil, fmt.Errorf("%s: unmarshal: %w", filepath.Base(path), err)
			}
			seq++
			if e.Action != "SET_BLOCK" {
				continue
			}
			if e.Tick < sinceTick || e.Tick > toTick {
				continue
			}
			if !withinAABB(e.Pos, min, max) {
				continue
			}
			out = append(out, auditRec{Seq: seq, Entry: e})
		}
		if err := sc.Err(); err != nil {
			dec.Close()
			_ = f.Close()
			return nil, err
		}
		dec.Close()
		_ = f.Close()
	}

	// Reverse chronological apply: highest tick first; for same tick use reverse read order.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Entry.Tick != out[j].Entry.Tick {
			return out[i].Entry.Tick > out[j].Entry.Tick
		}
		return out[i].Seq > out[j].Seq
	})
	return out, nil
}

func applyRollback(snap *snapshot.SnapshotV1, recs []auditRec) (applied, skipped int) {
	if snap == nil || len(recs) == 0 {
		return 0, 0
	}
	chunks := map[[2]int]*snapshot.ChunkV1{}
	for i := range snap.Chunks {
		ch := &snap.Chunks[i]
		chunks[[2]int{ch.CX, ch.CZ}] = ch
	}

	for _, r := range recs {
		p := r.Entry.Pos
		cx := floorDiv(p[0], 16)
		cz := floorDiv(p[2], 16)
		lx := mod(p[0], 16)
		lz := mod(p[2], 16)
		y := p[1]
		ch := chunks[[2]int{cx, cz}]
		if ch == nil || y < 0 || y >= ch.Height {
			skipped++
			continue
		}
		i := lx + lz*16 + y*16*16
		if i < 0 || i >= len(ch.Blocks) {
			skipped++
			continue
		}
		ch.Blocks[i] = r.Entry.From
		applied++
	}
	return applied, skipped
}

func withinAABB(pos [3]int, min, max [3]int) bool {
	return pos[0] >= min[0] && pos[0] <= max[0] &&
		pos[1] >= min[1] && pos[1] <= max[1] &&
		pos[2] >= min[2] && pos[2] <= max[2]
}

func parseAABB(s string) (min, max [3]int, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return min, max, fmt.Errorf("expected x1,y1,z1:x2,y2,z2")
	}
	a, err := parseVec3(parts[0])
	if err != nil {
		return min, max, err
	}
	b, err := parseVec3(parts[1])
	if err != nil {
		return min, max, err
	}
	for i := 0; i < 3; i++ {
		if a[i] <= b[i] {
			min[i], max[i] = a[i], b[i]
		} else {
			min[i], max[i] = b[i], a[i]
		}
	}
	return min, max, nil
}

func parseVec3(s string) ([3]int, error) {
	var v [3]int
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 3 {
		return v, fmt.Errorf("expected x,y,z")
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return v, err
		}
		v[i] = n
	}
	return v, nil
}

func latestSnapshot(worldDir string) string {
	dir := filepath.Join(worldDir, "snapshots")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestTick uint64
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".snap.zst") {
			continue
		}
		base := strings.TrimSuffix(name, ".snap.zst")
		tick, err := strconv.ParseUint(base, 10, 64)
		if err != nil {
			continue
		}
		if best == "" || tick > bestTick {
			bestTick = tick
			best = filepath.Join(dir, name)
		}
	}
	return best
}

func floorDiv(a, b int) int {
	// b > 0
	q := a / b
	r := a % b
	if r < 0 {
		q--
	}
	return q
}

func mod(a, b int) int {
	// b > 0
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}
