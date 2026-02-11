package blueprint

import "testing"

func TestNormalizeRotation_AcceptsDegreesAndQuarterTurns(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{in: 0, want: 0},
		{in: 1, want: 1},
		{in: 2, want: 2},
		{in: 3, want: 3},
		{in: 4, want: 0},
		{in: -1, want: 3},
		{in: 90, want: 1},
		{in: 180, want: 2},
		{in: 270, want: 3},
		{in: 360, want: 0},
		{in: -90, want: 3},
	}
	for _, c := range cases {
		if got := NormalizeRotation(c.in); got != c.want {
			t.Fatalf("NormalizeRotation(%d)=%d want %d", c.in, got, c.want)
		}
	}
}
