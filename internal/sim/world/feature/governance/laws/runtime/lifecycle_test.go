package runtime

import (
	"errors"
	"testing"

	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
)

func TestTickLawsTransitionsAndActivation(t *testing.T) {
	laws := map[string]*lawspkg.Law{
		"L1": {
			LawID:          "L1",
			Status:         lawspkg.StatusNotice,
			NoticeEndsTick: 5,
		},
		"L2": {
			LawID:        "L2",
			Status:       lawspkg.StatusVoting,
			VoteEndsTick: 5,
			Votes:        map[string]string{"a": "YES", "b": "NO", "c": "YES"},
		},
		"L3": {
			LawID:        "L3",
			Status:       lawspkg.StatusVoting,
			VoteEndsTick: 5,
			Votes:        map[string]string{"a": "NO"},
		},
	}

	var voting, activated, rejected int
	TickLaws(5, laws, TickLawsHooks{
		OnEnterVoting: func(_ *lawspkg.Law) { voting++ },
		OnActivate:    func(_ *lawspkg.Law, _ int, _ int) error { return nil },
		OnActivated:   func(_ *lawspkg.Law, _ int, _ int) { activated++ },
		OnRejected:    func(_ *lawspkg.Law, _ int, _ int, _ string, _ error) { rejected++ },
	})

	if laws["L1"].Status != lawspkg.StatusVoting {
		t.Fatalf("L1 expected VOTING, got %s", laws["L1"].Status)
	}
	if laws["L2"].Status != lawspkg.StatusActive {
		t.Fatalf("L2 expected ACTIVE, got %s", laws["L2"].Status)
	}
	if laws["L3"].Status != lawspkg.StatusRejected {
		t.Fatalf("L3 expected REJECTED, got %s", laws["L3"].Status)
	}
	if voting != 1 || activated != 1 || rejected != 1 {
		t.Fatalf("unexpected hook counters voting=%d activated=%d rejected=%d", voting, activated, rejected)
	}
}

func TestTickLawsActivateFailRejects(t *testing.T) {
	laws := map[string]*lawspkg.Law{
		"L1": {
			LawID:        "L1",
			Status:       lawspkg.StatusVoting,
			VoteEndsTick: 10,
			Votes:        map[string]string{"a": "YES"},
		},
	}
	var gotErr bool
	TickLaws(10, laws, TickLawsHooks{
		OnActivate: func(_ *lawspkg.Law, _ int, _ int) error {
			return errors.New("boom")
		},
		OnRejected: func(_ *lawspkg.Law, _ int, _ int, reason string, cause error) {
			if reason != "activate failed" || cause == nil {
				t.Fatalf("expected activate failed with cause, got reason=%q cause=%v", reason, cause)
			}
			gotErr = true
		},
	})
	if !gotErr {
		t.Fatalf("expected rejected hook with error")
	}
	if laws["L1"].Status != lawspkg.StatusRejected {
		t.Fatalf("expected rejected status, got %s", laws["L1"].Status)
	}
}
