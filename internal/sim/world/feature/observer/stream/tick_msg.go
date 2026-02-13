package stream

import (
	"encoding/json"

	"voxelcraft.ai/internal/observerproto"
)

type TickBuildInput struct {
	Tick                uint64
	TimeOfDay           float64
	Weather             string
	ActiveEventID       string
	ActiveEventEndsTick uint64
	Agents              []observerproto.AgentState
	Joins               []observerproto.JoinInfo
	Leaves              []string
	Actions             []observerproto.RecordedAction
	Audits              []observerproto.AuditEntry
}

func BuildTickMsgBytes(in TickBuildInput) ([]byte, error) {
	msg := observerproto.TickMsg{
		Type:                "TICK",
		ProtocolVersion:     observerproto.Version,
		Tick:                in.Tick,
		TimeOfDay:           in.TimeOfDay,
		Weather:             in.Weather,
		ActiveEventID:       in.ActiveEventID,
		ActiveEventEndsTick: in.ActiveEventEndsTick,
		Agents:              in.Agents,
		Joins:               in.Joins,
		Leaves:              in.Leaves,
		Actions:             in.Actions,
		Audits:              in.Audits,
	}
	return json.Marshal(msg)
}
