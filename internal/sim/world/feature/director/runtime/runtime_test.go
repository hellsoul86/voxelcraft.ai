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

func TestSeasonHelpers(t *testing.T) {
	if got := SeasonLengthTicks(0, 42000); got != 42000 {
		t.Fatalf("expected fallback season length, got %d", got)
	}
	if got := SeasonIndex(0, 42000, 0); got != 1 {
		t.Fatalf("expected season index 1 at tick 0, got %d", got)
	}
	if got := SeasonDay(6000, 6000, 42000, 0); got != 2 {
		t.Fatalf("expected season day 2, got %d", got)
	}
}

func TestShouldWorldResetNotice(t *testing.T) {
	ok, resetTick := ShouldWorldResetNotice(1000, 42000, 300)
	if ok || resetTick != 0 {
		t.Fatalf("unexpected notice outside window")
	}
	ok, resetTick = ShouldWorldResetNotice(41700, 42000, 300)
	if !ok || resetTick != 42000 {
		t.Fatalf("expected reset notice at cycle-notice, got ok=%v resetTick=%d", ok, resetTick)
	}
}
