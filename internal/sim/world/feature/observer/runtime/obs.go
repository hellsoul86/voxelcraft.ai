package runtime

import "voxelcraft.ai/internal/protocol"

type ComposeObsInput struct {
	Tick    uint64
	AgentID string
	WorldID string

	World      protocol.WorldObs
	Self       protocol.SelfObs
	Inventory  []protocol.ItemStack
	Equipment  protocol.EquipmentObs
	LocalRules protocol.LocalRulesObs
	Voxels     protocol.VoxelsObs
	Entities   []protocol.EntityObs
	Tasks      []protocol.TaskObs

	PublicBoards []protocol.BoardObs
}

func ComposeObs(in ComposeObsInput) protocol.ObsMsg {
	return protocol.ObsMsg{
		Type:            protocol.TypeObs,
		ProtocolVersion: protocol.Version,
		Tick:            in.Tick,
		AgentID:         in.AgentID,
		WorldID:         in.WorldID,
		WorldClock:      in.Tick,
		World:           in.World,
		Self:            in.Self,
		Inventory:       in.Inventory,
		Equipment:       in.Equipment,
		LocalRules:      in.LocalRules,
		Voxels:          in.Voxels,
		Entities:        in.Entities,
		Tasks:           in.Tasks,
		PublicBoards:    in.PublicBoards,
	}
}
