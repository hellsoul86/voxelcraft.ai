package world

import "voxelcraft.ai/internal/persistence/snapshot"

func (w *World) importSnapshotContracts(s snapshot.SnapshotV1) (maxContract uint64) {
	w.contracts = map[string]*Contract{}
	for _, c := range s.Contracts {
		req := map[string]int{}
		for item, n := range c.Requirements {
			if n > 0 {
				req[item] = n
			}
		}
		reward := map[string]int{}
		for item, n := range c.Reward {
			if n > 0 {
				reward[item] = n
			}
		}
		dep := map[string]int{}
		for item, n := range c.Deposit {
			if n > 0 {
				dep[item] = n
			}
		}
		cc := &Contract{
			ContractID:   c.ContractID,
			TerminalPos:  Vec3i{X: c.TerminalPos[0], Y: c.TerminalPos[1], Z: c.TerminalPos[2]},
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			Kind:         c.Kind,
			State:        ContractState(c.State),
			Requirements: req,
			Reward:       reward,
			Deposit:      dep,
			BlueprintID:  c.BlueprintID,
			Anchor:       Vec3i{X: c.Anchor[0], Y: c.Anchor[1], Z: c.Anchor[2]},
			Rotation:     c.Rotation,
			CreatedTick:  c.CreatedTick,
			DeadlineTick: c.DeadlineTick,
		}
		w.contracts[cc.ContractID] = cc
		if n, ok := parseUintAfterPrefix("C", cc.ContractID); ok && n > maxContract {
			maxContract = n
		}
	}
	return maxContract
}

func (w *World) importSnapshotLaws(s snapshot.SnapshotV1) (maxLaw uint64) {
	w.laws = map[string]*Law{}
	for _, l := range s.Laws {
		params := map[string]string{}
		for k, v := range l.Params {
			if k != "" && v != "" {
				params[k] = v
			}
		}
		votes := map[string]string{}
		for k, v := range l.Votes {
			if k != "" && v != "" {
				votes[k] = v
			}
		}
		ll := &Law{
			LawID:          l.LawID,
			LandID:         l.LandID,
			TemplateID:     l.TemplateID,
			Title:          l.Title,
			Params:         params,
			ProposedBy:     l.ProposedBy,
			ProposedTick:   l.ProposedTick,
			NoticeEndsTick: l.NoticeEndsTick,
			VoteEndsTick:   l.VoteEndsTick,
			Status:         LawStatus(l.Status),
			Votes:          votes,
		}
		w.laws[ll.LawID] = ll
		if n, ok := parseUintAfterPrefix("LAW", ll.LawID); ok && n > maxLaw {
			maxLaw = n
		}
	}
	return maxLaw
}
