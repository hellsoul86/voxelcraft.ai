package requests

import (
	"context"
	"errors"
)

type SnapshotReq struct {
	Resp chan SnapshotResp
}

type SnapshotResp struct {
	Tick uint64
	Err  string
}

type ResetReq struct {
	Resp chan ResetResp
}

type ResetResp struct {
	Tick uint64
	Err  string
}

func RequestSnapshot(ctx context.Context, ch chan<- SnapshotReq) (tick uint64, err error) {
	if ch == nil {
		return 0, errors.New("admin snapshot not available")
	}
	resp := make(chan SnapshotResp, 1)
	req := SnapshotReq{Resp: resp}
	select {
	case ch <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	select {
	case r := <-resp:
		if r.Err != "" {
			return r.Tick, errors.New(r.Err)
		}
		return r.Tick, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func RequestReset(ctx context.Context, ch chan<- ResetReq) (tick uint64, err error) {
	if ch == nil {
		return 0, errors.New("admin reset not available")
	}
	resp := make(chan ResetResp, 1)
	req := ResetReq{Resp: resp}
	select {
	case ch <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	select {
	case r := <-resp:
		if r.Err != "" {
			return r.Tick, errors.New(r.Err)
		}
		return r.Tick, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
