package requests

import "testing"

func TestRequestTypesCompile(t *testing.T) {
	req := SnapshotReq{Resp: make(chan SnapshotResp, 1)}
	req.Resp <- SnapshotResp{Tick: 1}
	got := <-req.Resp
	if got.Tick != 1 {
		t.Fatalf("unexpected snapshot resp: %#v", got)
	}
}
