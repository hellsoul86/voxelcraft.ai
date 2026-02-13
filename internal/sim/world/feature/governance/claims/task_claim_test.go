package claims

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type stubClaimTaskEnv struct {
	inBounds bool
	canBuild bool
	claims   []*modelpkg.LandClaim
	block    uint16
	air      uint16
	totem    uint16
	hasTotem bool
	world    string
	dayTicks int
	put      *modelpkg.LandClaim
}

func (s *stubClaimTaskEnv) InBounds(pos modelpkg.Vec3i) bool                                             { return s.inBounds }
func (s *stubClaimTaskEnv) CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool          { return s.canBuild }
func (s *stubClaimTaskEnv) Claims() []*modelpkg.LandClaim                                                { return s.claims }
func (s *stubClaimTaskEnv) BlockAt(pos modelpkg.Vec3i) uint16                                            { return s.block }
func (s *stubClaimTaskEnv) AirBlockID() uint16                                                           { return s.air }
func (s *stubClaimTaskEnv) ClaimTotemBlockID() (uint16, bool)                                            { return s.totem, s.hasTotem }
func (s *stubClaimTaskEnv) SetBlock(pos modelpkg.Vec3i, blockID uint16)                                 { s.block = blockID }
func (s *stubClaimTaskEnv) AuditSetBlock(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string) {}
func (s *stubClaimTaskEnv) NewLandID(owner string) string                                                { return "LAND_X" }
func (s *stubClaimTaskEnv) WorldType() string                                                            { return s.world }
func (s *stubClaimTaskEnv) DayTicks() int                                                                 { return s.dayTicks }
func (s *stubClaimTaskEnv) PutClaim(c *modelpkg.LandClaim)                                               { s.put = c }

func arClaim(tick uint64, ref string, ok bool, code string, message string) protocol.Event {
	e := protocol.Event{"t": tick, "ref": ref, "ok": ok}
	if code != "" {
		e["code"] = code
	}
	return e
}

func TestHandleTaskClaimLandSuccess(t *testing.T) {
	env := &stubClaimTaskEnv{
		inBounds: true,
		canBuild: true,
		claims:   nil,
		block:    0,
		air:      0,
		totem:    42,
		hasTotem: true,
		world:    "OVERWORLD",
		dayTicks: 6000,
	}
	a := &modelpkg.Agent{
		ID:        "A1",
		Inventory: map[string]int{"BATTERY": 1, "CRYSTAL_SHARD": 1},
	}
	HandleTaskClaimLand(env, arClaim, a, protocol.TaskReq{
		ID:     "K1",
		Type:   "CLAIM_LAND",
		Anchor: [3]int{10, 0, 10},
		Radius: 32,
	}, 100, true)
	if env.put == nil {
		t.Fatalf("expected claim to be created")
	}
	if a.Inventory["BATTERY"] != 0 || a.Inventory["CRYSTAL_SHARD"] != 0 {
		t.Fatalf("expected claim cost to be consumed: %#v", a.Inventory)
	}
}

func TestHandleTaskClaimLandOverlapRejected(t *testing.T) {
	env := &stubClaimTaskEnv{
		inBounds: true,
		canBuild: true,
		claims: []*modelpkg.LandClaim{
			{LandID: "L1", Anchor: modelpkg.Vec3i{X: 10, Y: 0, Z: 10}, Radius: 32},
		},
		block:    0,
		air:      0,
		totem:    42,
		hasTotem: true,
		world:    "OVERWORLD",
	}
	a := &modelpkg.Agent{
		ID:        "A1",
		Inventory: map[string]int{"BATTERY": 1, "CRYSTAL_SHARD": 1},
	}
	HandleTaskClaimLand(env, arClaim, a, protocol.TaskReq{
		ID:     "K1",
		Type:   "CLAIM_LAND",
		Anchor: [3]int{20, 0, 20},
		Radius: 32,
	}, 100, true)
	if env.put != nil {
		t.Fatalf("expected no claim creation on overlap")
	}
	if len(a.Events) == 0 {
		t.Fatalf("expected failure event")
	}
	if code, _ := a.Events[len(a.Events)-1]["code"].(string); code != "E_CONFLICT" {
		t.Fatalf("expected E_CONFLICT, got %q", code)
	}
}
