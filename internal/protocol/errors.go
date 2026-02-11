package protocol

const (
	// Protocol/transport validation.
	ErrProtoBadRequest = "E_PROTO_BAD_REQUEST"

	// World routing/state.
	ErrWorldBusy     = "E_WORLD_BUSY"
	ErrWorldNotFound = "E_WORLD_NOT_FOUND"
	ErrWorldDenied   = "E_WORLD_DENIED"
	ErrWorldCooldown = "E_WORLD_COOLDOWN"

	// Rule/action layer.
	ErrBadRequest    = "E_BAD_REQUEST"
	ErrNoPermission  = "E_NO_PERMISSION"
	ErrNoResource    = "E_NO_RESOURCE"
	ErrInvalidTarget = "E_INVALID_TARGET"
	ErrRateLimit     = "E_RATE_LIMIT"
	ErrConflict      = "E_CONFLICT"
	ErrBlocked       = "E_BLOCKED"
	ErrStale         = "E_STALE"
	ErrInternal      = "E_INTERNAL"
)

var knownCodes = map[string]struct{}{
	ErrProtoBadRequest: {},
	ErrWorldBusy:       {},
	ErrWorldNotFound:   {},
	ErrWorldDenied:     {},
	ErrWorldCooldown:   {},
	ErrBadRequest:      {},
	ErrNoPermission:    {},
	ErrNoResource:      {},
	ErrInvalidTarget:   {},
	ErrRateLimit:       {},
	ErrConflict:        {},
	ErrBlocked:         {},
	ErrStale:           {},
	ErrInternal:        {},
}

func IsKnownCode(code string) bool {
	if code == "" {
		return true
	}
	_, ok := knownCodes[code]
	return ok
}
