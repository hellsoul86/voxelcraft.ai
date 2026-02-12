package agent

import "testing"

func TestBuildWorldSwitchEvent(t *testing.T) {
	ev, ok := BuildWorldSwitchEvent(7, "OVERWORLD", "MINE_L1", "A1", "gate_a", "gate_b")
	if !ok {
		t.Fatalf("expected event")
	}
	if ev["type"] != "WORLD_SWITCH" || ev["from"] != "OVERWORLD" || ev["to"] != "MINE_L1" {
		t.Fatalf("unexpected event: %#v", ev)
	}
	if ev["from_entry_id"] != "gate_a" || ev["to_entry_id"] != "gate_b" {
		t.Fatalf("missing entry ids: %#v", ev)
	}
}

func TestBuildWorldSwitchEventEmptyFrom(t *testing.T) {
	ev, ok := BuildWorldSwitchEvent(7, "", "MINE_L1", "A1", "", "")
	if ok || ev != nil {
		t.Fatalf("expected no event, got ok=%v ev=%#v", ok, ev)
	}
}
