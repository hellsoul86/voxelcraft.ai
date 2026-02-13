package stats

import (
	"math"
	"strconv"
)

type Bucket struct {
	Trades             int
	Denied             int
	ChunksDiscovered   int
	BlueprintsComplete int
}

type ChunkKey struct {
	CX int
	CZ int
}

type WorldStats struct {
	BucketTicks  uint64
	WindowTicksV uint64

	Buckets []Bucket
	CurIdx  int
	CurBase uint64

	SeenChunks map[ChunkKey]bool
}

func NewWorldStats(bucketTicks, windowTicks uint64) *WorldStats {
	if bucketTicks == 0 {
		bucketTicks = 300
	}
	if windowTicks < bucketTicks {
		windowTicks = bucketTicks
	}
	n := int(windowTicks / bucketTicks)
	if n < 1 {
		n = 1
	}
	return &WorldStats{
		BucketTicks:  bucketTicks,
		WindowTicksV: uint64(n) * bucketTicks,
		Buckets:      make([]Bucket, n),
		CurIdx:       0,
		CurBase:      0,
		SeenChunks:   map[ChunkKey]bool{},
	}
}

func (s *WorldStats) rotate(nowTick uint64) {
	if s == nil {
		return
	}
	for nowTick >= s.CurBase+s.BucketTicks {
		s.CurIdx = (s.CurIdx + 1) % len(s.Buckets)
		s.Buckets[s.CurIdx] = Bucket{}
		s.CurBase += s.BucketTicks
	}
}

func (s *WorldStats) ObservePos(nowTick uint64, x, z int) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	cx := floorDiv(x, 16)
	cz := floorDiv(z, 16)
	k := ChunkKey{CX: cx, CZ: cz}
	if s.SeenChunks[k] {
		return
	}
	s.SeenChunks[k] = true
	s.Buckets[s.CurIdx].ChunksDiscovered++
}

func (s *WorldStats) RecordTrade(nowTick uint64) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	s.Buckets[s.CurIdx].Trades++
}

func (s *WorldStats) RecordDenied(nowTick uint64) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	s.Buckets[s.CurIdx].Denied++
}

func (s *WorldStats) RecordBlueprintComplete(nowTick uint64) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	s.Buckets[s.CurIdx].BlueprintsComplete++
}

func (s *WorldStats) WindowTicks() uint64 {
	if s == nil {
		return 0
	}
	return s.WindowTicksV
}

func (s *WorldStats) Summarize(nowTick uint64) Bucket {
	if s == nil {
		return Bucket{}
	}
	s.rotate(nowTick)
	var out Bucket
	for _, b := range s.Buckets {
		out.Trades += b.Trades
		out.Denied += b.Denied
		out.ChunksDiscovered += b.ChunksDiscovered
		out.BlueprintsComplete += b.BlueprintsComplete
	}
	return out
}

func floorDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	q := a / b
	r := a % b
	if r != 0 && ((r > 0) != (b > 0)) {
		q--
	}
	return q
}

func SocialFunFactor(repTrade int) float64 {
	if repTrade >= 500 {
		return 1.0
	}
	if repTrade <= 0 {
		return 0.5
	}
	return 0.5 + 0.5*(float64(repTrade)/500.0)
}

func ScaleByFactor(base int, factor float64) int {
	if base <= 0 {
		return 0
	}
	return int(math.Round(float64(base) * factor))
}

func InfluenceUsagePoints(users int) int {
	if users <= 0 {
		return 0
	}
	return int(math.Round(minFloat(15, 3*math.Sqrt(float64(users)))))
}

func StructureID(builderID string, nowTick uint64, blueprintID string, anchorX, anchorY, anchorZ int) string {
	return "STRUCT_" + builderID + "_" + strconv.FormatUint(nowTick, 10) + "_" + blueprintID + "_" +
		strconv.Itoa(anchorX) + "_" + strconv.Itoa(anchorY) + "_" + strconv.Itoa(anchorZ)
}

func StructureUniqueUsers(usedBy map[string]uint64, builderID string, nowTick uint64, window uint64) int {
	if len(usedBy) == 0 {
		return 0
	}
	cutoff := uint64(0)
	if nowTick > window {
		cutoff = nowTick - window
	}
	n := 0
	for aid, last := range usedBy {
		if aid == "" || aid == builderID {
			continue
		}
		if last >= cutoff {
			n++
		}
	}
	return n
}

type CreationScoreInput struct {
	UniqueBlockTypes int
	HasStorage       bool
	HasLight         bool
	HasWorkshop      bool
	HasGovernance    bool
	Stable           bool
	Users            int
}

func CreationScore(in CreationScoreInput) int {
	if in.UniqueBlockTypes < 0 {
		in.UniqueBlockTypes = 0
	}
	base := 5
	complexity := int(math.Round(math.Log(1+float64(in.UniqueBlockTypes)) * 2))
	modules := 0
	if in.HasStorage {
		modules += 2
	}
	if in.HasLight {
		modules += 2
	}
	if in.HasWorkshop {
		modules += 2
	}
	if in.HasGovernance {
		modules += 2
	}
	stability := 0
	if in.Stable {
		stability = 3
	}
	usageBonus := minInt(10, 2*in.Users)
	return base + complexity + modules + stability + usageBonus
}

type Vec3 struct {
	X int
	Y int
	Z int
}

func IsStructureStable(positions []Vec3, isSolid func(x, y, z int) bool) bool {
	if len(positions) == 0 {
		return true
	}
	index := make(map[Vec3]int, len(positions))
	for i, p := range positions {
		index[p] = i
	}

	visited := make([]bool, len(positions))
	queue := make([]int, 0, len(positions))

	for i, p := range positions {
		if p.Y <= 1 {
			visited[i] = true
			queue = append(queue, i)
			continue
		}
		below := Vec3{X: p.X, Y: p.Y - 1, Z: p.Z}
		if _, ok := index[below]; ok {
			continue
		}
		if isSolid(below.X, below.Y, below.Z) {
			visited[i] = true
			queue = append(queue, i)
		}
	}

	dirs := []Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		p := positions[i]
		for _, d := range dirs {
			np := Vec3{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
			ni, ok := index[np]
			if !ok || visited[ni] {
				continue
			}
			visited[ni] = true
			queue = append(queue, ni)
		}
	}

	supported := 0
	for _, v := range visited {
		if v {
			supported++
		}
	}
	if supported == 0 {
		return false
	}
	return float64(supported)/float64(len(positions)) >= 0.95
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
