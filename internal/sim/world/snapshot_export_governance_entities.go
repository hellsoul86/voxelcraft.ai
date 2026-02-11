package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) exportSnapshotContracts() []snapshot.ContractV1 {
	contractIDs := make([]string, 0, len(w.contracts))
	for id := range w.contracts {
		contractIDs = append(contractIDs, id)
	}
	sort.Strings(contractIDs)
	contractSnaps := make([]snapshot.ContractV1, 0, len(contractIDs))
	for _, id := range contractIDs {
		c := w.contracts[id]
		req := map[string]int{}
		for k, v := range c.Requirements {
			if v != 0 {
				req[k] = v
			}
		}
		reward := map[string]int{}
		for k, v := range c.Reward {
			if v != 0 {
				reward[k] = v
			}
		}
		dep := map[string]int{}
		for k, v := range c.Deposit {
			if v != 0 {
				dep[k] = v
			}
		}
		contractSnaps = append(contractSnaps, snapshot.ContractV1{
			ContractID:   c.ContractID,
			TerminalPos:  c.TerminalPos.ToArray(),
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			Kind:         c.Kind,
			State:        string(c.State),
			Requirements: req,
			Reward:       reward,
			Deposit:      dep,
			BlueprintID:  c.BlueprintID,
			Anchor:       c.Anchor.ToArray(),
			Rotation:     c.Rotation,
			CreatedTick:  c.CreatedTick,
			DeadlineTick: c.DeadlineTick,
		})
	}
	return contractSnaps
}

func (w *World) exportSnapshotLaws() []snapshot.LawV1 {
	lawIDs := make([]string, 0, len(w.laws))
	for id := range w.laws {
		lawIDs = append(lawIDs, id)
	}
	sort.Strings(lawIDs)
	lawSnaps := make([]snapshot.LawV1, 0, len(lawIDs))
	for _, id := range lawIDs {
		l := w.laws[id]
		if l == nil {
			continue
		}
		params := map[string]string{}
		for k, v := range l.Params {
			if v != "" {
				params[k] = v
			}
		}
		votes := map[string]string{}
		for k, v := range l.Votes {
			if v != "" {
				votes[k] = v
			}
		}
		lawSnaps = append(lawSnaps, snapshot.LawV1{
			LawID:          l.LawID,
			LandID:         l.LandID,
			TemplateID:     l.TemplateID,
			Title:          l.Title,
			Params:         params,
			Status:         string(l.Status),
			ProposedBy:     l.ProposedBy,
			ProposedTick:   l.ProposedTick,
			NoticeEndsTick: l.NoticeEndsTick,
			VoteEndsTick:   l.VoteEndsTick,
			Votes:          votes,
		})
	}
	return lawSnaps
}
