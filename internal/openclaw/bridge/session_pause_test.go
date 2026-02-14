package bridge

import "testing"

func TestDisconnectAndPauseSetsPaused(t *testing.T) {
	s := NewSession(SessionConfig{Key: "t", WorldWSURL: "ws://example.invalid"}, nil)

	s.DisconnectAndPause()

	s.mu.RLock()
	paused := s.paused
	connected := s.connected
	s.mu.RUnlock()

	if !paused {
		t.Fatalf("expected paused=true after DisconnectAndPause")
	}
	if connected {
		t.Fatalf("expected connected=false after DisconnectAndPause")
	}
}

func TestResumeReconnectClearsPausedAndSignals(t *testing.T) {
	s := NewSession(SessionConfig{Key: "t", WorldWSURL: "ws://example.invalid"}, nil)
	s.DisconnectAndPause()

	s.ResumeReconnect()

	s.mu.RLock()
	paused := s.paused
	s.mu.RUnlock()
	if paused {
		t.Fatalf("expected paused=false after ResumeReconnect")
	}

	select {
	case <-s.resumeNotify:
		// expected
	default:
		t.Fatalf("expected resumeNotify to be signaled when transitioning paused->running")
	}
}

func TestDisconnectDoesNotPause(t *testing.T) {
	s := NewSession(SessionConfig{Key: "t", WorldWSURL: "ws://example.invalid"}, nil)

	s.Disconnect()

	s.mu.RLock()
	paused := s.paused
	s.mu.RUnlock()
	if paused {
		t.Fatalf("expected paused=false after Disconnect")
	}
}
