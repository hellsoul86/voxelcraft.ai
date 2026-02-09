package encoding

import "testing"

func TestRLE_RoundTrip(t *testing.T) {
	in := make([]uint16, 0, 200)
	in = append(in, 1, 1, 1, 2, 2, 3)
	for i := 0; i < 50; i++ {
		in = append(in, 7)
	}
	in = append(in, 9, 10, 10, 10)

	enc := EncodeRLE(in)
	out, err := DecodeRLE(enc)
	if err != nil {
		t.Fatalf("DecodeRLE: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("len mismatch: got %d want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("mismatch at %d: got %d want %d", i, out[i], in[i])
		}
	}
}
