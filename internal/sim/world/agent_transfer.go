package world

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/protocol"
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
	Memory    map[string]memoryEntry
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

func (w *World) RequestTransferOut(ctx context.Context, agentID string) (AgentTransfer, error) {
	if w == nil || w.transferOut == nil {
		return AgentTransfer{}, errors.New("transfer out not available")
	}
	req := transferOutReq{
		AgentID: agentID,
		Resp:    make(chan transferOutResp, 1),
	}
	select {
	case w.transferOut <- req:
	case <-ctx.Done():
		return AgentTransfer{}, ctx.Err()
	}
	select {
	case r := <-req.Resp:
		if r.Err != "" {
			return AgentTransfer{}, errors.New(r.Err)
		}
		return r.Transfer, nil
	case <-ctx.Done():
		return AgentTransfer{}, ctx.Err()
	}
}

func (w *World) RequestTransferIn(ctx context.Context, t AgentTransfer, out chan []byte, delta bool) error {
	if w == nil || w.transferIn == nil {
		return errors.New("transfer in not available")
	}
	req := transferInReq{
		Transfer:    t,
		Out:         out,
		DeltaVoxels: delta,
		Resp:        make(chan transferInResp, 1),
	}
	select {
	case w.transferIn <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case r := <-req.Resp:
		if r.Err != "" {
			return errors.New(r.Err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

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

	inv := map[string]int{}
	for item, n := range a.Inventory {
		if item == "" || n <= 0 {
			continue
		}
		inv[item] = n
	}
	mem := map[string]memoryEntry{}
	for k, v := range a.Memory {
		if k == "" {
			continue
		}
		mem[k] = v
	}

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
		Inventory:                    map[string]int{},
		Equipment:                    t.Equipment,
		Memory:                       map[string]memoryEntry{},
	}
	if a.Pos.Y != 0 {
		a.Pos.Y = 0
	}
	if a.OrgID == "" && t.Org != nil && t.Org.OrgID != "" {
		a.OrgID = t.Org.OrgID
	}
	a.MoveTask = nil
	a.WorkTask = nil
	for item, n := range t.Inventory {
		if item == "" || n <= 0 {
			continue
		}
		a.Inventory[item] = n
	}
	for k, v := range t.Memory {
		if k == "" {
			continue
		}
		a.Memory[k] = v
	}
	a.initDefaults()
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
				if n, ok := parseUintAfterPrefix("ORG", org.OrgID); ok && n > w.nextOrgNum.Load() {
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
	if t.FromWorldID != "" {
		ev := protocol.Event{
			"t":        w.tick.Load(),
			"type":     "WORLD_SWITCH",
			"from":     t.FromWorldID,
			"to":       w.cfg.ID,
			"agent_id": a.ID,
			"world_id": w.cfg.ID,
		}
		if t.FromEntryPointID != "" {
			ev["from_entry_id"] = t.FromEntryPointID
		}
		if t.ToEntryPointID != "" {
			ev["to_entry_id"] = t.ToEntryPointID
		}
		a.AddEvent(ev)
	}

	w.agents[a.ID] = a
	if req.Out != nil {
		w.clients[a.ID] = &clientState{Out: req.Out, DeltaVoxels: req.DeltaVoxels}
	}
}
