package requests

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
