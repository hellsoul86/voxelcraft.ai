package director

type StatsBucket struct {
	Trades             int
	Denied             int
	ChunksDiscovered   int
	BlueprintsComplete int
}

type StatsChunkKey struct {
	CX int
	CZ int
}

type WorldStats struct {
	BucketTicks  uint64
	WindowTicksV uint64

	Buckets []StatsBucket
	CurIdx  int
	CurBase uint64 // start tick (inclusive) of current bucket

	SeenChunks map[StatsChunkKey]bool
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
		Buckets:      make([]StatsBucket, n),
		CurIdx:       0,
		CurBase:      0,
		SeenChunks:   map[StatsChunkKey]bool{},
	}
}

func (s *WorldStats) rotate(nowTick uint64) {
	if s == nil {
		return
	}
	// Move forward until nowTick is in [curBase, curBase+bucketTicks).
	for nowTick >= s.CurBase+s.BucketTicks {
		s.CurIdx = (s.CurIdx + 1) % len(s.Buckets)
		s.Buckets[s.CurIdx] = StatsBucket{}
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
	k := StatsChunkKey{CX: cx, CZ: cz}
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

func (s *WorldStats) Summarize(nowTick uint64) StatsBucket {
	if s == nil {
		return StatsBucket{}
	}
	s.rotate(nowTick)
	var out StatsBucket
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
