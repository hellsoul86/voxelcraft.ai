package meta

import (
	"fmt"

	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func ObsID(agentID string, tick uint64, cursor uint64) string {
	return fmt.Sprintf("%s:%d:%d", agentID, tick, cursor)
}

func FunScorePtr(novelty, creation, social, influence, narrative, riskRescue int) *protocol.FunScoreObs {
	if novelty == 0 && creation == 0 && social == 0 && influence == 0 && narrative == 0 && riskRescue == 0 {
		return nil
	}
	return &protocol.FunScoreObs{
		Novelty:    novelty,
		Creation:   creation,
		Social:     social,
		Influence:  influence,
		Narrative:  narrative,
		RiskRescue: riskRescue,
	}
}

func AttachObsEventsAndMeta(a *modelpkg.Agent, obs *protocol.ObsMsg, nowTick uint64) {
	if a == nil || obs == nil {
		return
	}

	ev := a.TakeEvents()
	obs.Events = ev
	obs.EventsCursor = a.EventCursor
	obs.ObsID = ObsID(a.ID, nowTick, a.EventCursor)
	obs.FunScore = FunScorePtr(
		a.Fun.Novelty,
		a.Fun.Creation,
		a.Fun.Social,
		a.Fun.Influence,
		a.Fun.Narrative,
		a.Fun.RiskRescue,
	)

	if len(a.PendingMemory) > 0 {
		obs.Memory = a.PendingMemory
		a.PendingMemory = nil
	}
}
