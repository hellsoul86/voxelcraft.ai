package world

import "voxelcraft.ai/internal/sim/world/feature/transfer"

func (w *World) handleTransferOut(req transferOutReq) {
	resp := transferOutResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()

	a := w.agents[req.AgentID]
	if a == nil {
		resp.Err = "agent not found"
		return
	}

	// Cancel tasks on world switch.
	a.MoveTask = nil
	a.WorkTask = nil

	inv := transfer.CopyPositiveIntMap(a.Inventory)
	mem := transfer.CopyMap(a.Memory, func(k string, _ memoryEntry) bool { return k != "" })

	resp.Transfer = AgentTransfer{
		ID:                           a.ID,
		Name:                         a.Name,
		OrgID:                        a.OrgID,
		FromWorldID:                  w.cfg.ID,
		CurrentWorldID:               a.CurrentWorldID,
		WorldSwitchCooldownUntilTick: a.WorldSwitchCooldownUntilTick,
		Pos:                          a.Pos,
		Yaw:                          a.Yaw,
		HP:                           a.HP,
		Hunger:                       a.Hunger,
		StaminaMilli:                 a.StaminaMilli,
		RepTrade:                     a.RepTrade,
		RepBuild:                     a.RepBuild,
		RepSocial:                    a.RepSocial,
		RepLaw:                       a.RepLaw,
		Fun:                          a.Fun,
		Inventory:                    inv,
		Equipment:                    a.Equipment,
		Memory:                       mem,
	}
	if a.OrgID != "" {
		if org := w.orgByID(a.OrgID); org != nil {
			members := map[string]OrgRole{}
			for aid, role := range org.Members {
				if aid == "" || role == "" {
					continue
				}
				members[aid] = role
			}
			resp.Transfer.Org = &OrgTransfer{
				OrgID:       org.OrgID,
				Kind:        org.Kind,
				Name:        org.Name,
				CreatedTick: org.CreatedTick,
				MetaVersion: org.MetaVersion,
				Members:     members,
			}
		}
	}

	delete(w.clients, a.ID)
	delete(w.agents, a.ID)

	// Clear open trades involving this agent in this world.
	for tid, tr := range w.trades {
		if tr == nil {
			continue
		}
		if tr.From == a.ID || tr.To == a.ID {
			delete(w.trades, tid)
		}
	}
}
