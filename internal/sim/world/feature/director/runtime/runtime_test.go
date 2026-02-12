package runtime

import "testing"

func TestExpire(t *testing.T) {
	out, res := Expire(State{
		ActiveEventID:   "MARKET_WEEK",
		ActiveEventEnds: 10,
		Weather:         "STORM",
		WeatherUntil:    10,
	}, 10)
	if out.ActiveEventID != "" || out.Weather != "CLEAR" {
		t.Fatalf("unexpected expire output: %+v", out)
	}
	if !res.ActiveCleared || !res.WeatherCleared {
		t.Fatalf("expected both clear flags, got %+v", res)
	}
}

func TestScriptedEvent(t *testing.T) {
	if got := ScriptedEvent(1); got != "MARKET_WEEK" {
		t.Fatalf("unexpected day1 event: %s", got)
	}
	if got := ScriptedEvent(8); got != "" {
		t.Fatalf("expected empty after schedule, got: %s", got)
	}
}

func TestShouldEvaluate(t *testing.T) {
	if ShouldEvaluate(0, 3000) {
		t.Fatalf("tick zero should not evaluate")
	}
	if !ShouldEvaluate(3000, 3000) {
		t.Fatalf("tick 3000 should evaluate")
	}
	if ShouldEvaluate(3001, 3000) {
		t.Fatalf("tick 3001 should not evaluate")
	}
}
