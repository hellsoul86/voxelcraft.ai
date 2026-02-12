package events

import "testing"

func TestBuildResp(t *testing.T) {
	resp := BuildResp(false, 10, nil, 10)
	if resp.Err == "" {
		t.Fatalf("expected missing agent error")
	}

	in := []CursorItem{{Cursor: 11, Event: map[string]any{"type": "X"}}}
	resp = BuildResp(true, 10, in, 11)
	if resp.Err != "" || resp.NextCursor != 11 || len(resp.Items) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	in[0].Cursor = 99
	if resp.Items[0].Cursor != 11 {
		t.Fatalf("response should copy input items")
	}
}
