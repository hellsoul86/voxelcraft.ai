package protocol

import "testing"

func TestIsKnownCode(t *testing.T) {
	cases := []string{
		"",
		ErrProtoBadRequest,
		ErrWorldBusy,
		ErrWorldNotFound,
		ErrWorldDenied,
		ErrWorldCooldown,
		ErrBadRequest,
		ErrNoPermission,
		ErrNoResource,
		ErrInvalidTarget,
		ErrRateLimit,
		ErrConflict,
		ErrBlocked,
		ErrStale,
		ErrInternal,
	}
	for _, c := range cases {
		if !IsKnownCode(c) {
			t.Fatalf("expected known code: %q", c)
		}
	}
	if IsKnownCode("E_NOT_DEFINED") {
		t.Fatalf("expected unknown code rejected")
	}
}
