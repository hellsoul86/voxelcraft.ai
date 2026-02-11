package world

import (
	"math"
	"strconv"
)

func (w *World) funDecay(a *Agent, key string, base int, nowTick uint64) int {
	if a == nil || base <= 0 {
		return 0
	}
	dw := a.funDecay[key]
	if dw == nil {
		dw = &funDecayWindow{StartTick: nowTick}
		a.funDecay[key] = dw
	}
	window := uint64(w.cfg.FunDecayWindowTicks)
	if window == 0 {
		window = 3000
	}
	if nowTick-dw.StartTick >= window {
		dw.StartTick = nowTick
		dw.Count = 0
	}
	dw.Count++
	baseMult := w.cfg.FunDecayBase
	if baseMult <= 0 || baseMult > 1.0 {
		baseMult = 0.70
	}
	mult := math.Pow(baseMult, float64(dw.Count-1))
	delta := int(math.Round(float64(base) * mult))
	if delta <= 0 {
		return 0
	}
	return delta
}

func socialFunFactor(a *Agent) float64 {
	// Map reputation 0..1000 to 0.5..1.0 multiplier (default rep=500 -> 1.0).
	rep := a.RepTrade
	if rep >= 500 {
		return 1.0
	}
	if rep <= 0 {
		return 0.5
	}
	return 0.5 + 0.5*(float64(rep)/500.0)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func itoaU64(v uint64) string { return strconv.FormatUint(v, 10) }
func itoaI(v int) string      { return strconv.Itoa(v) }
