package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
)

func TestActionResult_UnknownCodeIsSanitized(t *testing.T) {
	ev := actionResult(1, "X", false, "E_UNKNOWN_CODE", "")
	got, _ := ev["code"].(string)
	if got != protocol.ErrInternal {
		t.Fatalf("code=%q want %q", got, protocol.ErrInternal)
	}
}
