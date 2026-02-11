package governance

import "testing"

func TestNormalizeVoteChoice(t *testing.T) {
	tests := map[string]string{
		"yes":     "YES",
		"Y":       "YES",
		"1":       "YES",
		"true":    "YES",
		"no":      "NO",
		"N":       "NO",
		"0":       "NO",
		"false":   "NO",
		"abstain": "ABSTAIN",
		" ??? ":   "",
	}
	for in, want := range tests {
		if got := NormalizeVoteChoice(in); got != want {
			t.Fatalf("NormalizeVoteChoice(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCountVotes(t *testing.T) {
	yes, no := CountVotes(map[string]string{
		"a": "YES",
		"b": "no",
		"c": "ABSTAIN",
		"d": "TRUE",
		"e": "invalid",
	})
	if yes != 2 || no != 1 {
		t.Fatalf("CountVotes mismatch: yes=%d no=%d", yes, no)
	}
}
