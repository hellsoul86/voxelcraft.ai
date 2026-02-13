package mcp

import (
	"sync"
	"time"
)

type replayGuard struct {
	mu        sync.Mutex
	seen      map[string]int64
	ttl       time.Duration
	lastPrune int64
}

func newReplayGuard(ttl time.Duration) *replayGuard {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &replayGuard{
		seen: map[string]int64{},
		ttl:  ttl,
	}
}

func (g *replayGuard) allow(sessionKey string, signature string, now time.Time) bool {
	if g == nil {
		return true
	}
	sig := signature
	if sig == "" {
		return true
	}
	key := sessionKey + "|" + sig
	nowMS := now.UnixMilli()
	expiresAt := nowMS + g.ttl.Milliseconds()

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.shouldPruneLocked(nowMS) {
		g.pruneLocked(nowMS)
	}
	if exp, ok := g.seen[key]; ok && exp > nowMS {
		return false
	}
	g.seen[key] = expiresAt
	if len(g.seen) > 65536 {
		// Hard cap in case of unexpectedly high-cardinality traffic.
		g.seen = map[string]int64{key: expiresAt}
		g.lastPrune = nowMS
	}
	return true
}

func (g *replayGuard) shouldPruneLocked(nowMS int64) bool {
	if len(g.seen) == 0 {
		return false
	}
	if len(g.seen) > 4096 {
		return true
	}
	return nowMS-g.lastPrune > g.ttl.Milliseconds()/2
}

func (g *replayGuard) pruneLocked(nowMS int64) {
	for k, exp := range g.seen {
		if exp <= nowMS {
			delete(g.seen, k)
		}
	}
	g.lastPrune = nowMS
}
