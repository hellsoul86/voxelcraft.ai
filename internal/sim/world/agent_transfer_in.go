package world

import (
	transferagentpkg "voxelcraft.ai/internal/sim/world/feature/transfer/agent"
	transfermapspkg "voxelcraft.ai/internal/sim/world/feature/transfer/maps"
	idspkg "voxelcraft.ai/internal/sim/world/logic/ids"
)

type AgentTransfer struct {
	ID    string
	Name  string
	OrgID string
	Org   *OrgTransfer

	FromWorldID                  string
	CurrentWorldID               string
	FromEntryPointID             string
	ToEntryPointID               string
	WorldSwitchCooldownUntilTick uint64

	Pos Vec3i
	Yaw int

	HP           int
	Hunger       int
	StaminaMilli int

	RepTrade  int
	RepBuild  int
	RepSocial int
	RepLaw    int

	Fun       FunScore
	Inventory map[string]int
	Equipment Equipment
	Memory    map[string]MemoryEntry
}

type OrgTransfer struct {
	OrgID       string
	Kind        OrgKind
	Name        string
	CreatedTick uint64
	MetaVersion uint64
	Members     map[string]OrgRole
}

type transferOutReq struct {
	AgentID string
	Resp    chan transferOutResp
}

type transferOutResp struct {
	Transfer AgentTransfer
	Err      string
}

type transferInReq struct {
	Transfer    AgentTransfer
	Out         chan []byte
	DeltaVoxels bool
	Resp        chan transferInResp
}

type transferInResp struct {
	Err string
}

func (w *World) handleTransferIn(req transferInReq) {
	resp := transferInResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()

	t := req.Transfer
	if t.ID == "" {
		resp.Err = "missing agent id"
		return
	}
	if _, ok := w.agents[t.ID]; ok {
		resp.Err = "agent already present"
		return
	}

	a := &Agent{
		ID:                           t.ID,
		Name:                         t.Name,
		OrgID:                        t.OrgID,
		CurrentWorldID:               w.cfg.ID,
		WorldSwitchCooldownUntilTick: t.WorldSwitchCooldownUntilTick,
		Pos:                          t.Pos,
		Yaw:                          t.Yaw,
		HP:                           t.HP,
		Hunger:                       t.Hunger,
		StaminaMilli:                 t.StaminaMilli,
		RepTrade:                     t.RepTrade,
		RepBuild:                     t.RepBuild,
		RepSocial:                    t.RepSocial,
		RepLaw:                       t.RepLaw,
		Fun:                          t.Fun,
		Inventory:                    transfermapspkg.CopyPositiveIntMap(t.Inventory),
		Equipment:                    t.Equipment,
		Memory:                       transfermapspkg.CopyMap(t.Memory, func(k string, _ MemoryEntry) bool { return k != "" }),
	}
	if a.Pos.Y != 0 {
		a.Pos.Y = 0
	}
	if a.OrgID == "" && t.Org != nil && t.Org.OrgID != "" {
		a.OrgID = t.Org.OrgID
	}
	a.MoveTask = nil
	a.WorkTask = nil
	a.InitDefaults()
	if a.OrgID != "" {
		var org *Organization
		if t.Org != nil && t.Org.OrgID != "" {
			org = w.orgByID(t.Org.OrgID)
			if org == nil {
				org = &Organization{
					OrgID:           t.Org.OrgID,
					Kind:            t.Org.Kind,
					Name:            t.Org.Name,
					CreatedTick:     t.Org.CreatedTick,
					MetaVersion:     t.Org.MetaVersion,
					Members:         map[string]OrgRole{},
					Treasury:        map[string]int{},
					TreasuryByWorld: map[string]map[string]int{},
				}
				w.orgs[org.OrgID] = org
					if n, ok := idspkg.ParseUintAfterPrefix("ORG", org.OrgID); ok && n > w.nextOrgNum.Load() {
						w.nextOrgNum.Store(n)
					}
				}
			if org.Kind == "" {
				org.Kind = t.Org.Kind
			}
			if org.Name == "" {
				org.Name = t.Org.Name
			}
			if org.CreatedTick == 0 {
				org.CreatedTick = t.Org.CreatedTick
			}
			if t.Org.MetaVersion > org.MetaVersion {
				org.MetaVersion = t.Org.MetaVersion
			}
			if org.Members == nil {
				org.Members = map[string]OrgRole{}
			}
			for aid, role := range t.Org.Members {
				if aid == "" || role == "" {
					continue
				}
				org.Members[aid] = role
			}
		} else {
			org = w.orgByID(a.OrgID)
			if org == nil {
				org = &Organization{
					OrgID:           a.OrgID,
					Kind:            OrgGuild,
					Name:            a.OrgID,
					MetaVersion:     1,
					Members:         map[string]OrgRole{},
					Treasury:        map[string]int{},
					TreasuryByWorld: map[string]map[string]int{},
				}
				w.orgs[org.OrgID] = org
			}
		}
		if org != nil {
			if org.Members == nil {
				org.Members = map[string]OrgRole{}
			}
			if _, ok := org.Members[a.ID]; !ok {
				org.Members[a.ID] = OrgMember
			}
			_ = w.orgTreasury(org)
		}
	}
	if ev, ok := transferagentpkg.BuildWorldSwitchEvent(w.tick.Load(), t.FromWorldID, w.cfg.ID, a.ID, t.FromEntryPointID, t.ToEntryPointID); ok {
		a.AddEvent(ev)
	}

	w.agents[a.ID] = a
	if req.Out != nil {
		w.clients[a.ID] = &clientState{Out: req.Out, DeltaVoxels: req.DeltaVoxels}
	}
}
