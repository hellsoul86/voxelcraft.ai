package directorcenter

import (
	"math"
	"sort"
)

func Min01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 || math.IsNaN(x) {
		return 1
	}
	return x
}

func MapValue(inv map[string]int, valueOf func(string) float64) float64 {
	if len(inv) == 0 {
		return 0
	}
	var v float64
	for item, n := range inv {
		if n <= 0 {
			continue
		}
		v += float64(n) * valueOf(item)
	}
	return v
}

func Gini(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	sum := 0.0
	valid := make([]float64, 0, len(values))
	for _, x := range values {
		if x <= 0 {
			continue
		}
		valid = append(valid, x)
		sum += x
	}
	if len(valid) <= 1 || sum <= 0 {
		return 0
	}
	sort.Float64s(valid)
	// Gini coefficient:
	// (2*sum_i i*x_i)/(n*sum x) - (n+1)/n, with i=1..n.
	n := float64(len(valid))
	var weighted float64
	for i, x := range valid {
		weighted += float64(i+1) * x
	}
	g := (2.0*weighted)/(n*sum) - (n+1.0)/n
	return Min01(g)
}
