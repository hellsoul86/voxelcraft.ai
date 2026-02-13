package runtime

import (
	transferagentpkg "voxelcraft.ai/internal/sim/world/feature/transfer/agent"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	idspkg "voxelcraft.ai/internal/sim/world/logic/ids"
)

type TransferInHandleInput struct {
	Req            TransferInReq
	WorldID        string
	NowTick        uint64
	Agents         map[string]*modelpkg.Agent
	Orgs           map[string]*modelpkg.Organization
	CurrentNextOrg uint64
}

type TransferInHandleOutput struct {
	Err                string
	JoinedAgentID      string
	NextOrg            uint64
	WorldSwitchApplied bool
}

func HandleTransferIn(input TransferInHandleInput) TransferInHandleOutput {
	out := TransferInHandleOutput{NextOrg: input.CurrentNextOrg}
	t := input.Req.Transfer
	if t.ID == "" {
		out.Err = "missing agent id"
		return out
	}
	if _, ok := input.Agents[t.ID]; ok {
		out.Err = "agent already present"
		return out
	}

	a := BuildIncomingAgent(t, input.WorldID)
	if a.OrgID != "" {
		orgID := a.OrgID
		if t.Org != nil && t.Org.OrgID != "" {
			orgID = t.Org.OrgID
		}
		_, existed := input.Orgs[orgID]
		org := UpsertIncomingOrg(input.Orgs, t.Org, a.OrgID, a.ID)
		if org != nil {
			if !existed {
				if n, ok := idspkg.ParseUintAfterPrefix("ORG", org.OrgID); ok && n > out.NextOrg {
					out.NextOrg = n
				}
			}
			_ = org.TreasuryFor(input.WorldID)
		}
	}
	if ev, ok := transferagentpkg.BuildWorldSwitchEvent(input.NowTick, t.FromWorldID, input.WorldID, a.ID, t.FromEntryPointID, t.ToEntryPointID); ok {
		a.AddEvent(ev)
		out.WorldSwitchApplied = true
	}
	input.Agents[a.ID] = a
	out.JoinedAgentID = a.ID
	return out
}

type TransferOutHandleInput struct {
	Req     TransferOutReq
	WorldID string
	Agents  map[string]*modelpkg.Agent
	Orgs    map[string]*modelpkg.Organization
	Trades  map[string]*modelpkg.Trade
}

type TransferOutHandleOutput struct {
	Err       string
	Transfer  AgentTransfer
	RemovedID string
}

func HandleTransferOut(input TransferOutHandleInput) TransferOutHandleOutput {
	out := TransferOutHandleOutput{}
	a := input.Agents[input.Req.AgentID]
	if a == nil {
		out.Err = "agent not found"
		return out
	}

	a.MoveTask = nil
	a.WorkTask = nil

	var orgTransfer *OrgTransfer
	if a.OrgID != "" {
		if org := input.Orgs[a.OrgID]; org != nil {
			orgTransfer = BuildOrgTransferFromOrganization(org)
		}
	}
	out.Transfer = BuildOutgoingAgent(a, input.WorldID, orgTransfer)

	delete(input.Agents, a.ID)
	for tid, tr := range input.Trades {
		if tr == nil {
			continue
		}
		if tr.From == a.ID || tr.To == a.ID {
			delete(input.Trades, tid)
		}
	}
	out.RemovedID = a.ID
	return out
}
