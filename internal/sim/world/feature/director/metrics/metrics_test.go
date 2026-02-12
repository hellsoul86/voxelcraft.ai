package metrics

import "testing"

func TestComputeMetrics(t *testing.T) {
	got := ComputeMetrics(EvalInput{
		Agents:      2,
		WindowTicks: 100,
		Trades:      10,
		Denied:      2,
		Chunks:      8,
		Blueprints:  4,
		Wealth:      []float64{10, 30},
	})
	if got.Trade <= 0 || got.Exploration <= 0 || got.PublicInfra <= 0 {
		t.Fatalf("expected positive metrics, got %#v", got)
	}
	if got.Inequality <= 0 {
		t.Fatalf("expected positive inequality, got %#v", got)
	}
}
