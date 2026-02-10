package world

import "testing"

func setAir(w *World, pos Vec3i) {
	pos.Y = 0
	w.chunks.SetBlock(pos, w.chunks.gen.Air)
}

func setSolid(w *World, pos Vec3i, blockID uint16) {
	pos.Y = 0
	w.chunks.SetBlock(pos, blockID)
}

func clearBlueprintFootprint(t *testing.T, w *World, blueprintID string, anchor Vec3i, rotation int) {
	t.Helper()
	bp, ok := w.catalogs.Blueprints.ByID[blueprintID]
	if !ok {
		t.Fatalf("missing blueprint %q", blueprintID)
	}
	rot := normalizeRotation(rotation)
	for _, b := range bp.Blocks {
		off := rotateOffset(b.Pos, rot)
		p := Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
		setAir(w, p)
	}
}

