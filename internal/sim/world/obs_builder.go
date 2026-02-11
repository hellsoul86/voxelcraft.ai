package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/feature/governance"
)

func (w *World) buildObs(a *Agent, cl *clientState, nowTick uint64) protocol.ObsMsg {
	center := a.Pos
	vox, sensorsNear := w.buildObsVoxels(center, cl)

	land, perms := w.permissionsFor(a.ID, a.Pos)
	if land != nil && land.CurfewEnabled {
		t := w.timeOfDay(nowTick)
		if governance.InWindow(t, land.CurfewStart, land.CurfewEnd) {
			perms["can_build"] = false
			perms["can_break"] = false
		}
	}

	tasksObs := w.buildObsTasks(a, nowTick)
	ents := w.buildObsEntities(a, sensorsNear)

	// Public boards (global, MVP).
	publicBoards := make([]protocol.BoardObs, 0, len(w.boards))
	if len(w.boards) > 0 {
		boardIDs := make([]string, 0, len(w.boards))
		for id := range w.boards {
			// For physical boards, only include nearby boards in OBS to keep payloads small.
			if typ, pos, ok := parseContainerID(id); ok && typ == "BULLETIN_BOARD" {
				if Manhattan(pos, a.Pos) > 32 {
					continue
				}
			}
			boardIDs = append(boardIDs, id)
		}
		sort.Strings(boardIDs)
		for _, bid := range boardIDs {
			b := w.boards[bid]
			if b == nil || len(b.Posts) == 0 {
				continue
			}
			posts := make([]protocol.BoardPost, 0, 5)
			// Newest first.
			for i := len(b.Posts) - 1; i >= 0 && len(posts) < 5; i-- {
				p := b.Posts[i]
				summary := p.Body
				if len(summary) > 120 {
					summary = summary[:120]
				}
				posts = append(posts, protocol.BoardPost{
					PostID:  p.PostID,
					Author:  p.Author,
					Title:   p.Title,
					Summary: summary,
				})
			}
			publicBoards = append(publicBoards, protocol.BoardObs{BoardID: bid, TopPosts: posts})
		}
	}

	localRules := protocol.LocalRulesObs{Permissions: perms}
	if land != nil {
		localRules.LandID = land.LandID
		localRules.Owner = land.Owner
		if land.Owner == a.ID {
			localRules.Role = "OWNER"
		} else if w.isLandMember(a.ID, land) {
			localRules.Role = "MEMBER"
		} else {
			localRules.Role = "VISITOR"
		}
		localRules.Tax = map[string]float64{"market": land.MarketTax}
		localRules.MaintenanceDueTick = land.MaintenanceDueTick
		localRules.MaintenanceStage = land.MaintenanceStage
	} else {
		localRules.Role = "WILD"
		localRules.Tax = map[string]float64{"market": 0.0}
	}

	status := make([]string, 0, 4)
	if a.Hunger == 0 {
		status = append(status, "STARVING")
	} else if a.Hunger < 5 {
		status = append(status, "HUNGRY")
	}
	if a.StaminaMilli < 200 {
		status = append(status, "TIRED")
	}
	if w.weather == "STORM" {
		status = append(status, "STORM")
	} else if w.weather == "COLD" {
		status = append(status, "COLD")
	}
	if len(status) == 0 {
		status = append(status, "NONE")
	}

	obs := protocol.ObsMsg{
		Type:            protocol.TypeObs,
		ProtocolVersion: protocol.Version,
		Tick:            nowTick,
		AgentID:         a.ID,
		WorldID:         w.cfg.ID,
		WorldClock:      nowTick,
		World: protocol.WorldObs{
			TimeOfDay:           float64(int(nowTick)%w.cfg.DayTicks) / float64(w.cfg.DayTicks),
			Weather:             w.weather,
			SeasonDay:           w.seasonDay(nowTick),
			Biome:               biomeAt(w.cfg.Seed, a.Pos.X, a.Pos.Z, w.cfg.BiomeRegionSize),
			ActiveEvent:         w.activeEventID,
			ActiveEventEndsTick: w.activeEventEnds,
		},
		Self: protocol.SelfObs{
			Pos:     a.Pos.ToArray(),
			Yaw:     a.Yaw,
			HP:      a.HP,
			Hunger:  a.Hunger,
			Stamina: float64(a.StaminaMilli) / 1000.0,
			Status:  status,
			Reputation: protocol.ReputationObs{
				Trade:  float64(a.RepTrade) / 1000.0,
				Build:  float64(a.RepBuild) / 1000.0,
				Social: float64(a.RepSocial) / 1000.0,
				Law:    float64(a.RepLaw) / 1000.0,
			},
		},
		Inventory: a.InventoryList(),
		Equipment: protocol.EquipmentObs{
			MainHand: a.Equipment.MainHand,
			Armor:    []string{a.Equipment.Armor[0], a.Equipment.Armor[1], a.Equipment.Armor[2], a.Equipment.Armor[3]},
		},
		LocalRules:   localRules,
		Voxels:       vox,
		Entities:     ents,
		Tasks:        tasksObs,
		PublicBoards: publicBoards,
	}
	w.attachObsEventsAndMeta(a, &obs, nowTick)
	return obs
}
