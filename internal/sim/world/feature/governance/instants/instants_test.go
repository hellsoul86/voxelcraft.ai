package instants

import "testing"

func TestValidateLandAdmin(t *testing.T) {
	if ok, _, _ := ValidateLandAdmin(true, true); !ok {
		t.Fatalf("expected pass")
	}
	if ok, code, _ := ValidateLandAdmin(false, true); ok || code != "E_INVALID_TARGET" {
		t.Fatalf("expected invalid target")
	}
}
