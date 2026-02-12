package runtime

import "testing"

func TestNextTransitionNoticeToVoting(t *testing.T) {
	tr := NextTransition(TransitionInput{
		Status:         "NOTICE",
		NowTick:        10,
		NoticeEndsTick: 10,
	})
	if !tr.ShouldTransition || tr.NextStatus != "VOTING" || tr.EventKind != "VOTING" {
		t.Fatalf("unexpected transition: %+v", tr)
	}
}

func TestNextTransitionVotingBoundary(t *testing.T) {
	tr := NextTransition(TransitionInput{
		Status:       "VOTING",
		NowTick:      12,
		VoteEndsTick: 12,
	})
	if !tr.ShouldTransition {
		t.Fatalf("expected transition at vote end")
	}
}

func TestVotePassed(t *testing.T) {
	if !VotePassed(3, 2) {
		t.Fatalf("expected pass")
	}
	if VotePassed(2, 2) {
		t.Fatalf("expected tie not pass")
	}
}
