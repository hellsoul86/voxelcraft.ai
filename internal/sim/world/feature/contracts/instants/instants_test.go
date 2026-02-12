package instants

import "testing"

func TestBuildTerminalContext(t *testing.T) {
	ctx := BuildTerminalContext(true, "CONTRACT_TERMINAL", Vec3{X: 2, Y: 0, Z: 2}, Vec3{X: 2, Y: 0, Z: 2}, Vec3{X: 1, Y: 0, Z: 1})
	if ctx.Type != "CONTRACT_TERMINAL" || !ctx.Matches || ctx.Distance != 2 {
		t.Fatalf("unexpected ctx: %+v", ctx)
	}
}
