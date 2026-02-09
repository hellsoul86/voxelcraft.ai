package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestLandMembersPermissions(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		out := make(chan []byte, 1)
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: out, Resp: resp})
		jr := <-resp
		return w.agents[jr.Welcome.AgentID]
	}

	owner := join("owner")
	member := join("member")

	land := &LandClaim{
		LandID: "LAND_MEM",
		Owner:  owner.ID,
		Anchor: owner.Pos,
		Radius: 32,
		Flags:  ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
	}
	w.claims[land.LandID] = land

	// Member starts as visitor.
	_, perms := w.permissionsFor(member.ID, owner.Pos)
	if perms["can_build"] || perms["can_break"] || perms["can_trade"] {
		t.Fatalf("expected visitor permissions denied, got %+v", perms)
	}

	// Add member via instant.
	w.applyInstant(owner, protocol.InstantReq{
		ID:       "I_add",
		Type:     "ADD_MEMBER",
		LandID:   land.LandID,
		MemberID: member.ID,
	}, 0)

	_, perms = w.permissionsFor(member.ID, owner.Pos)
	if !perms["can_build"] || !perms["can_break"] || !perms["can_trade"] {
		t.Fatalf("expected member permissions allowed, got %+v", perms)
	}

	// Container withdrawal should be allowed for members.
	if !w.canWithdrawFromContainer(member.ID, owner.Pos) {
		t.Fatalf("expected member can withdraw from container")
	}

	// Remove member.
	w.applyInstant(owner, protocol.InstantReq{
		ID:       "I_remove",
		Type:     "REMOVE_MEMBER",
		LandID:   land.LandID,
		MemberID: member.ID,
	}, 1)

	_, perms = w.permissionsFor(member.ID, owner.Pos)
	if perms["can_build"] || perms["can_break"] || perms["can_trade"] {
		t.Fatalf("expected visitor permissions denied after removal, got %+v", perms)
	}
}
