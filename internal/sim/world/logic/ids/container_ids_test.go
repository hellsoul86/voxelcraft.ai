package ids

import "testing"

func TestContainerIDRoundTrip(t *testing.T) {
	id := ContainerID("CHEST", 12, 0, -9)
	typ, x, y, z, ok := ParseContainerID(id)
	if !ok {
		t.Fatalf("ParseContainerID failed for %q", id)
	}
	if typ != "CHEST" || x != 12 || y != 0 || z != -9 {
		t.Fatalf("unexpected parse result: typ=%q x=%d y=%d z=%d", typ, x, y, z)
	}
}

func TestParseContainerIDRejectsInvalid(t *testing.T) {
	tests := []string{
		"",
		"CHEST",
		"CHEST@1,2",
		"CHEST@1,2,x",
	}
	for _, tc := range tests {
		if _, _, _, _, ok := ParseContainerID(tc); ok {
			t.Fatalf("expected parse failure for %q", tc)
		}
	}
}
