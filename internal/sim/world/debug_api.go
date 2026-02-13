package world

import (
	"errors"
	"fmt"

	admindebugpkg "voxelcraft.ai/internal/sim/world/feature/admin/debug"
)

// ---- Debug/Test Helpers ----
//
// These helpers exist to allow black-box tests in sibling packages (e.g. internal/sim/worldtest)
// to set up deterministic preconditions without reaching into world internals.
//
// They are NOT safe to call concurrently with Run(). Prefer using them only in tests that drive
// the world via StepOnce(), from a single goroutine.

func (w *World) DebugSetAgentPos(agentID string, pos Vec3i) bool {
	if w == nil || agentID == "" {
		return false
	}
	return admindebugpkg.SetAgentPos(w.agents[agentID], pos)
}

func (w *World) DebugClearAgentEvents(agentID string) bool {
	if w == nil || agentID == "" {
		return false
	}
	return admindebugpkg.ClearAgentEvents(w.agents[agentID])
}

func (w *World) DebugSetAgentVitals(agentID string, hp, hunger, staminaMilli int) bool {
	if w == nil || agentID == "" {
		return false
	}
	return admindebugpkg.SetAgentVitals(w.agents[agentID], hp, hunger, staminaMilli)
}

func (w *World) DebugSetAgentReputation(agentID string, repTrade, repBuild, repSocial, repLaw int) bool {
	if w == nil || agentID == "" {
		return false
	}
	return admindebugpkg.SetAgentReputation(w.agents[agentID], repTrade, repBuild, repSocial, repLaw)
}

func (w *World) DebugAddInventory(agentID string, item string, delta int) bool {
	if w == nil || agentID == "" {
		return false
	}
	if w.catalogs == nil {
		return false
	}
	return admindebugpkg.AddInventory(w.agents[agentID], item, delta, func(id string) bool {
		_, ok := w.catalogs.Items.Defs[id]
		return ok
	})
}

// DebugSetBlock sets a single world tile directly (2D: y must be 0).
// It does not write audit entries and does not update derived runtime meta (containers/signs/etc).
func (w *World) DebugSetBlock(pos Vec3i, blockName string) error {
	if w == nil {
		return errors.New("nil world")
	}
	if pos.Y != 0 {
		return fmt.Errorf("2D world requires y==0: %+v", pos)
	}
	bid, ok := w.catalogs.Blocks.Index[blockName]
	if !ok {
		return fmt.Errorf("unknown block: %q", blockName)
	}
	if !w.chunks.inBounds(pos) {
		return fmt.Errorf("out of bounds: %+v", pos)
	}
	w.chunks.SetBlock(pos, bid)
	return nil
}

func (w *World) DebugGetBlock(pos Vec3i) (uint16, error) {
	if w == nil {
		return 0, errors.New("nil world")
	}
	if pos.Y != 0 {
		return 0, fmt.Errorf("2D world requires y==0: %+v", pos)
	}
	if !w.chunks.inBounds(pos) {
		return 0, fmt.Errorf("out of bounds: %+v", pos)
	}
	return w.chunks.GetBlock(pos), nil
}

// DebugStateDigest returns the current world digest for the given tick label.
// This is intended for black-box determinism tests in sibling packages.
func (w *World) DebugStateDigest(nowTick uint64) string {
	if w == nil {
		return ""
	}
	return w.stateDigest(nowTick)
}

// CheckBlueprintPlaced is a stable helper for tests/automation.
// It reports whether the blueprint blocks match the world at the given anchor/rotation.
func (w *World) CheckBlueprintPlaced(blueprintID string, anchor [3]int, rotation int) bool {
	if w == nil {
		return false
	}
	return w.checkBlueprintPlaced(blueprintID, Vec3i{X: anchor[0], Y: anchor[1], Z: anchor[2]}, rotation)
}
