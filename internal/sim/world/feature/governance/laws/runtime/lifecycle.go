package runtime

import (
	"sort"

	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
)

type TickLawsHooks struct {
	OnEnterVoting func(law *lawspkg.Law)
	OnActivate    func(law *lawspkg.Law, yes int, no int) error
	OnActivated   func(law *lawspkg.Law, yes int, no int)
	OnRejected    func(law *lawspkg.Law, yes int, no int, reason string, cause error)
}

func TickLaws(nowTick uint64, laws map[string]*lawspkg.Law, hooks TickLawsHooks) {
	if len(laws) == 0 {
		return
	}
	ids := make([]string, 0, len(laws))
	for id := range laws {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		law := laws[id]
		if law == nil {
			continue
		}
		tr := NextTransition(TransitionInput{
			Status:         string(law.Status),
			NowTick:        nowTick,
			NoticeEndsTick: law.NoticeEndsTick,
			VoteEndsTick:   law.VoteEndsTick,
		})
		if tr.ShouldTransition && tr.NextStatus == string(lawspkg.StatusVoting) {
			law.Status = lawspkg.StatusVoting
			if hooks.OnEnterVoting != nil {
				hooks.OnEnterVoting(law)
			}
			continue
		}
		if !(tr.ShouldTransition && law.Status == lawspkg.StatusVoting) {
			continue
		}

		yes, no := lawspkg.CountVotes(law.Votes)
		if VotePassed(yes, no) {
			if hooks.OnActivate != nil {
				if err := hooks.OnActivate(law, yes, no); err != nil {
					law.Status = lawspkg.StatusRejected
					if hooks.OnRejected != nil {
						hooks.OnRejected(law, yes, no, "activate failed", err)
					}
					continue
				}
			}
			law.Status = lawspkg.StatusActive
			if hooks.OnActivated != nil {
				hooks.OnActivated(law, yes, no)
			}
			continue
		}

		law.Status = lawspkg.StatusRejected
		if hooks.OnRejected != nil {
			hooks.OnRejected(law, yes, no, "vote failed", nil)
		}
	}
}
