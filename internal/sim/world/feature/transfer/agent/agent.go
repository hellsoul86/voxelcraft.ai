package agent

import "voxelcraft.ai/internal/protocol"

func BuildWorldSwitchEvent(nowTick uint64, fromWorldID string, toWorldID string, agentID string, fromEntryID string, toEntryID string) (protocol.Event, bool) {
	if fromWorldID == "" {
		return nil, false
	}
	ev := protocol.Event{
		"t":        nowTick,
		"type":     "WORLD_SWITCH",
		"from":     fromWorldID,
		"to":       toWorldID,
		"agent_id": agentID,
		"world_id": toWorldID,
	}
	if fromEntryID != "" {
		ev["from_entry_id"] = fromEntryID
	}
	if toEntryID != "" {
		ev["to_entry_id"] = toEntryID
	}
	return ev, true
}
