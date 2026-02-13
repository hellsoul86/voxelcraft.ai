package mcp

import (
	"testing"
	"time"
)

func TestReplayGuard_RejectsDuplicateWithinWindow(t *testing.T) {
	g := newReplayGuard(10 * time.Second)
	now := time.Unix(1700000000, 0)
	if !g.allow("agent_1", "sig_1", now) {
		t.Fatalf("expected first request to pass")
	}
	if g.allow("agent_1", "sig_1", now.Add(1*time.Second)) {
		t.Fatalf("expected duplicate signature to be rejected")
	}
	if !g.allow("agent_1", "sig_2", now.Add(1*time.Second)) {
		t.Fatalf("expected different signature to pass")
	}
}

func TestReplayGuard_AllowsAfterExpiry(t *testing.T) {
	g := newReplayGuard(2 * time.Second)
	now := time.Unix(1700000000, 0)
	if !g.allow("agent_1", "sig_1", now) {
		t.Fatalf("expected first request to pass")
	}
	if g.allow("agent_1", "sig_1", now.Add(1*time.Second)) {
		t.Fatalf("expected duplicate request in ttl to fail")
	}
	if !g.allow("agent_1", "sig_1", now.Add(3*time.Second)) {
		t.Fatalf("expected request after ttl expiry to pass")
	}
}
