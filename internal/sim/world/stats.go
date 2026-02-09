package world

type StatsBucket struct {
	Trades             int
	Denied             int
	ChunksDiscovered   int
	BlueprintsComplete int
}

type WorldStats struct {
	bucketTicks uint64
	windowTicks uint64

	buckets []StatsBucket
	curIdx  int
	curBase uint64 // start tick (inclusive) of current bucket

	seenChunks map[ChunkKey]bool
}

func NewWorldStats(bucketTicks, windowTicks uint64) *WorldStats {
	if bucketTicks <= 0 {
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
		bucketTicks: bucketTicks,
		windowTicks: uint64(n) * bucketTicks,
		buckets:     make([]StatsBucket, n),
		curIdx:      0,
		curBase:     0,
		seenChunks:  map[ChunkKey]bool{},
	}
}

func (s *WorldStats) rotate(nowTick uint64) {
	if s == nil {
		return
	}
	// Move forward until nowTick is in [curBase, curBase+bucketTicks).
	for nowTick >= s.curBase+s.bucketTicks {
		s.curIdx = (s.curIdx + 1) % len(s.buckets)
		s.buckets[s.curIdx] = StatsBucket{}
		s.curBase += s.bucketTicks
	}
}

func (s *WorldStats) ObserveAgents(nowTick uint64, agents map[string]*Agent) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	for _, a := range agents {
		if a == nil {
			continue
		}
		cx := floorDiv(a.Pos.X, 16)
		cz := floorDiv(a.Pos.Z, 16)
		k := ChunkKey{CX: cx, CZ: cz}
		if s.seenChunks[k] {
			continue
		}
		s.seenChunks[k] = true
		s.buckets[s.curIdx].ChunksDiscovered++
	}
}

func (s *WorldStats) RecordTrade(nowTick uint64) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	s.buckets[s.curIdx].Trades++
}

func (s *WorldStats) RecordDenied(nowTick uint64) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	s.buckets[s.curIdx].Denied++
}

func (s *WorldStats) RecordBlueprintComplete(nowTick uint64) {
	if s == nil {
		return
	}
	s.rotate(nowTick)
	s.buckets[s.curIdx].BlueprintsComplete++
}

func (s *WorldStats) WindowTicks() uint64 {
	if s == nil {
		return 0
	}
	return s.windowTicks
}

func (s *WorldStats) Summarize(nowTick uint64) StatsBucket {
	if s == nil {
		return StatsBucket{}
	}
	s.rotate(nowTick)
	var out StatsBucket
	for _, b := range s.buckets {
		out.Trades += b.Trades
		out.Denied += b.Denied
		out.ChunksDiscovered += b.ChunksDiscovered
		out.BlueprintsComplete += b.BlueprintsComplete
	}
	return out
}
