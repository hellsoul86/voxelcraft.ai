package observer

import (
	"fmt"

	"voxelcraft.ai/internal/protocol"
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
