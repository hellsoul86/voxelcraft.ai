package runtime

import (
	"context"
	"errors"

	orgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
)

type TransferOutReq struct {
	AgentID string
	Resp    chan TransferOutResp
}

type TransferOutResp struct {
	Transfer AgentTransfer
	Err      string
}

type TransferInReq struct {
	Transfer    AgentTransfer
	Out         chan []byte
	DeltaVoxels bool
	Resp        chan TransferInResp
}

type TransferInResp struct {
	Err string
}

type AgentPosReq struct {
	AgentID string
	Resp    chan AgentPosResp
}

type AgentPosResp struct {
	Pos [3]int
	Err string
}

type OrgMetaReq struct {
	Resp chan OrgMetaResp
}

type OrgMetaResp struct {
	Orgs []orgpkg.State
	Err  string
}

type OrgMetaUpsertReq struct {
	Orgs []orgpkg.State
	Resp chan OrgMetaUpsertResp
}

type OrgMetaUpsertResp struct {
	Err string
}

func RequestTransferOut(ctx context.Context, ch chan<- TransferOutReq, agentID string) (AgentTransfer, error) {
	if ch == nil {
		return AgentTransfer{}, errors.New("transfer out not available")
	}
	req := TransferOutReq{
		AgentID: agentID,
		Resp:    make(chan TransferOutResp, 1),
	}
	select {
	case ch <- req:
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

func RequestTransferIn(ctx context.Context, ch chan<- TransferInReq, t AgentTransfer, out chan []byte, delta bool) error {
	if ch == nil {
		return errors.New("transfer in not available")
	}
	req := TransferInReq{
		Transfer:    t,
		Out:         out,
		DeltaVoxels: delta,
		Resp:        make(chan TransferInResp, 1),
	}
	select {
	case ch <- req:
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

func RequestAgentPos(ctx context.Context, ch chan<- AgentPosReq, agentID string) ([3]int, error) {
	if ch == nil {
		return [3]int{}, errors.New("agent position query not available")
	}
	req := AgentPosReq{
		AgentID: agentID,
		Resp:    make(chan AgentPosResp, 1),
	}
	select {
	case ch <- req:
	case <-ctx.Done():
		return [3]int{}, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return [3]int{}, errors.New(resp.Err)
		}
		return resp.Pos, nil
	case <-ctx.Done():
		return [3]int{}, ctx.Err()
	}
}

func RequestOrgMetaSnapshot(ctx context.Context, ch chan<- OrgMetaReq) ([]orgpkg.State, error) {
	if ch == nil {
		return nil, errors.New("org metadata query not available")
	}
	req := OrgMetaReq{Resp: make(chan OrgMetaResp, 1)}
	select {
	case ch <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return nil, errors.New(resp.Err)
		}
		return resp.Orgs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func RequestOrgMetaUpsert(ctx context.Context, ch chan<- OrgMetaUpsertReq, orgs []orgpkg.State) error {
	if ch == nil {
		return errors.New("org metadata upsert not available")
	}
	req := OrgMetaUpsertReq{
		Orgs: orgs,
		Resp: make(chan OrgMetaUpsertResp, 1),
	}
	select {
	case ch <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return errors.New(resp.Err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
