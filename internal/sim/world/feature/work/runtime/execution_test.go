package runtime

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type stubCraftExecEnv struct {
	recipes map[string]catalogs.RecipeDef
	smelts  map[string]catalogs.RecipeDef
	near    bool
}

func (s stubCraftExecEnv) GetRecipe(recipeID string) (catalogs.RecipeDef, bool) {
	v, ok := s.recipes[recipeID]
	return v, ok
}

func (s stubCraftExecEnv) GetSmeltRecipeByInput(itemID string) (catalogs.RecipeDef, bool) {
	v, ok := s.smelts[itemID]
	return v, ok
}

func (s stubCraftExecEnv) NearBlock(_ modelpkg.Vec3i, _ string, _ int) bool { return s.near }

func (s stubCraftExecEnv) OnRecipe(_ *modelpkg.Agent, _ string, _ int, _ uint64) {}

func TestTickCraftConsumesAndProduces(t *testing.T) {
	env := stubCraftExecEnv{
		recipes: map[string]catalogs.RecipeDef{
			"stick_from_plank": {
				RecipeID:  "stick_from_plank",
				Station:   "HAND",
				TimeTicks: 1,
				Inputs:    []catalogs.ItemCount{{Item: "PLANK", Count: 2}},
				Outputs:   []catalogs.ItemCount{{Item: "STICK", Count: 4}},
			},
		},
		near: true,
	}
	a := &modelpkg.Agent{Inventory: map[string]int{"PLANK": 2}}
	wt := &tasks.WorkTask{TaskID: "T1", Kind: tasks.KindCraft, RecipeID: "stick_from_plank", Count: 1}
	a.WorkTask = wt

	TickCraft(env, a, wt, 10)

	if a.WorkTask != nil {
		t.Fatalf("expected work task to finish")
	}
	if got := a.Inventory["PLANK"]; got != 0 {
		t.Fatalf("expected PLANK consumed, got %d", got)
	}
	if got := a.Inventory["STICK"]; got != 4 {
		t.Fatalf("expected STICK output=4, got %d", got)
	}
}

type stubInteractExecEnv struct {
	containers map[string]*modelpkg.Container
	withdrawOK bool
}

func (s stubInteractExecEnv) GetContainerByID(id string) *modelpkg.Container { return s.containers[id] }
func (s stubInteractExecEnv) ParseContainerID(string) (string, modelpkg.Vec3i, bool) {
	return "", modelpkg.Vec3i{}, false
}
func (s stubInteractExecEnv) BlockNameAt(modelpkg.Vec3i) string { return "" }
func (s stubInteractExecEnv) EnsureBoard(modelpkg.Vec3i) *modelpkg.Board {
	return &modelpkg.Board{}
}
func (s stubInteractExecEnv) BoardIDAt(modelpkg.Vec3i) string { return "" }
func (s stubInteractExecEnv) GetSign(modelpkg.Vec3i) *modelpkg.Sign {
	return nil
}
func (s stubInteractExecEnv) SignIDAt(modelpkg.Vec3i) string { return "" }
func (s stubInteractExecEnv) ContractSummariesForTerminal(modelpkg.Vec3i) []map[string]interface{} {
	return nil
}
func (s stubInteractExecEnv) OnContainerOpenedDuringEvent(*modelpkg.Agent, *modelpkg.Container, uint64) {
}
func (s stubInteractExecEnv) CanWithdrawFromContainer(string, modelpkg.Vec3i) bool {
	return s.withdrawOK
}
func (s stubInteractExecEnv) AuditTransfer(uint64, string, modelpkg.Vec3i, string, string, string, int) {
}

func TestTickTransferMovesItems(t *testing.T) {
	src := &modelpkg.Container{
		Type:      "CHEST",
		Pos:       modelpkg.Vec3i{X: 0, Y: 0, Z: 0},
		Inventory: map[string]int{"PLANK": 10},
	}
	dst := &modelpkg.Container{
		Type:      "CHEST",
		Pos:       modelpkg.Vec3i{X: 1, Y: 0, Z: 0},
		Inventory: map[string]int{},
	}
	env := stubInteractExecEnv{
		containers: map[string]*modelpkg.Container{
			"SRC": src,
			"DST": dst,
		},
		withdrawOK: true,
	}
	a := &modelpkg.Agent{
		ID:        "A1",
		Pos:       modelpkg.Vec3i{X: 0, Y: 0, Z: 0},
		Inventory: map[string]int{},
	}
	wt := &tasks.WorkTask{
		TaskID:       "T1",
		Kind:         tasks.KindTransfer,
		SrcContainer: "SRC",
		DstContainer: "DST",
		ItemID:       "PLANK",
		Count:        4,
	}
	a.WorkTask = wt

	TickTransfer(env, a, wt, 20)

	if a.WorkTask != nil {
		t.Fatalf("expected transfer task to finish")
	}
	if got := src.Inventory["PLANK"]; got != 6 {
		t.Fatalf("expected src=6, got %d", got)
	}
	if got := dst.Inventory["PLANK"]; got != 4 {
		t.Fatalf("expected dst=4, got %d", got)
	}
}

type stubGatherPlaceEnv struct {
	items      map[string]*modelpkg.ItemEntity
	canPickup  bool
	denied     int
	blocks     map[modelpkg.Vec3i]uint16
	air        uint16
	canBuild   bool
	placeAs    map[string]string
	blockIndex map[string]uint16
}

func (s *stubGatherPlaceEnv) GetItemEntity(id string) *modelpkg.ItemEntity { return s.items[id] }
func (s *stubGatherPlaceEnv) CanPickupItemEntity(string, modelpkg.Vec3i) bool {
	return s.canPickup
}
func (s *stubGatherPlaceEnv) RemoveItemEntity(_ uint64, _ string, id string, _ string) {
	delete(s.items, id)
}
func (s *stubGatherPlaceEnv) InBounds(modelpkg.Vec3i) bool { return true }
func (s *stubGatherPlaceEnv) CanBuildAt(string, modelpkg.Vec3i, uint64) bool {
	return s.canBuild
}
func (s *stubGatherPlaceEnv) RecordDenied(uint64)               { s.denied++ }
func (s *stubGatherPlaceEnv) BumpRepLaw(string, int)            {}
func (s *stubGatherPlaceEnv) BlockAt(pos modelpkg.Vec3i) uint16 { return s.blocks[pos] }
func (s *stubGatherPlaceEnv) AirBlockID() uint16                { return s.air }
func (s *stubGatherPlaceEnv) ItemPlaceAs(itemID string) (string, bool) {
	v, ok := s.placeAs[itemID]
	return v, ok
}
func (s *stubGatherPlaceEnv) BlockIDByName(blockName string) (uint16, bool) {
	v, ok := s.blockIndex[blockName]
	return v, ok
}
func (s *stubGatherPlaceEnv) SetBlock(pos modelpkg.Vec3i, blockID uint16) { s.blocks[pos] = blockID }
func (s *stubGatherPlaceEnv) AuditSetBlock(uint64, string, modelpkg.Vec3i, uint16, uint16, string) {
}
func (s *stubGatherPlaceEnv) EnsureContainerForPlacedBlock(modelpkg.Vec3i, string) {}
func (s *stubGatherPlaceEnv) EnsureConveyorFromYaw(modelpkg.Vec3i, int)            {}

func TestTickGatherCollectsItemEntity(t *testing.T) {
	pos := modelpkg.Vec3i{X: 1, Y: 0, Z: 1}
	env := &stubGatherPlaceEnv{
		items: map[string]*modelpkg.ItemEntity{
			"I1": {EntityID: "I1", Pos: pos, Item: "COAL", Count: 2},
		},
		canPickup: true,
		blocks:    map[modelpkg.Vec3i]uint16{},
	}
	a := &modelpkg.Agent{ID: "A1", Pos: pos, Inventory: map[string]int{}}
	wt := &tasks.WorkTask{TaskID: "T1", Kind: tasks.KindGather, TargetID: "I1"}
	a.WorkTask = wt

	TickGather(env, a, wt, 30)

	if a.WorkTask != nil {
		t.Fatalf("expected gather task to finish")
	}
	if got := a.Inventory["COAL"]; got != 2 {
		t.Fatalf("expected coal=2, got %d", got)
	}
	if env.items["I1"] != nil {
		t.Fatalf("expected item entity removed")
	}
}

func TestTickPlaceSetsBlockAndConsumesItem(t *testing.T) {
	pos := modelpkg.Vec3i{X: 2, Y: 0, Z: 2}
	env := &stubGatherPlaceEnv{
		canBuild: true,
		blocks:   map[modelpkg.Vec3i]uint16{},
		air:      0,
		placeAs: map[string]string{
			"CRAFTING_BENCH": "CRAFTING_BENCH",
		},
		blockIndex: map[string]uint16{
			"CRAFTING_BENCH": 7,
		},
	}
	a := &modelpkg.Agent{
		ID:        "A1",
		Pos:       pos,
		Inventory: map[string]int{"CRAFTING_BENCH": 1},
	}
	wt := &tasks.WorkTask{
		TaskID:   "T2",
		Kind:     tasks.KindPlace,
		ItemID:   "CRAFTING_BENCH",
		BlockPos: tasks.Vec3i{X: pos.X, Y: pos.Y, Z: pos.Z},
	}
	a.WorkTask = wt

	TickPlace(env, a, wt, 40)

	if a.WorkTask != nil {
		t.Fatalf("expected place task to finish")
	}
	if got := a.Inventory["CRAFTING_BENCH"]; got != 0 {
		t.Fatalf("expected item consumed, got %d", got)
	}
	if got := env.blocks[pos]; got != 7 {
		t.Fatalf("expected block id 7, got %d", got)
	}
}

type stubMineEnv struct {
	allowBreak bool
	blocks     map[modelpkg.Vec3i]uint16
	air        uint16
	names      map[uint16]string
	drops      map[uint16]string
}

func (s *stubMineEnv) CanBreakAt(string, modelpkg.Vec3i, uint64) bool { return s.allowBreak }
func (s *stubMineEnv) PermissionsFor(string, modelpkg.Vec3i) (*modelpkg.LandClaim, map[string]bool) {
	return nil, nil
}
func (s *stubMineEnv) IsLandMember(string, *modelpkg.LandClaim) bool { return false }
func (s *stubMineEnv) TransferToLandOwner(string, string, int)       {}
func (s *stubMineEnv) BumpRepLaw(string, int)                        {}
func (s *stubMineEnv) RecordDenied(uint64)                           {}
func (s *stubMineEnv) BlockAt(pos modelpkg.Vec3i) uint16             { return s.blocks[pos] }
func (s *stubMineEnv) AirBlockID() uint16                            { return s.air }
func (s *stubMineEnv) BlockName(blockID uint16) string               { return s.names[blockID] }
func (s *stubMineEnv) SetBlock(pos modelpkg.Vec3i, blockID uint16)   { s.blocks[pos] = blockID }
func (s *stubMineEnv) AuditSetBlock(uint64, string, modelpkg.Vec3i, uint16, uint16, string) {
}
func (s *stubMineEnv) BlockIDToItem(blockID uint16) string { return s.drops[blockID] }
func (s *stubMineEnv) SpawnItemEntity(uint64, string, modelpkg.Vec3i, string, int, string) string {
	return ""
}
func (s *stubMineEnv) GetContainerAt(modelpkg.Vec3i) *modelpkg.Container { return nil }
func (s *stubMineEnv) RemoveContainer(modelpkg.Vec3i)                    {}
func (s *stubMineEnv) RemoveBoard(modelpkg.Vec3i)                        {}
func (s *stubMineEnv) RemoveSign(uint64, string, modelpkg.Vec3i, string) {}
func (s *stubMineEnv) RemoveConveyor(uint64, string, modelpkg.Vec3i, string) {
}
func (s *stubMineEnv) RemoveSwitch(uint64, string, modelpkg.Vec3i, string) {}
func (s *stubMineEnv) RemoveClaimByAnchor(uint64, string, modelpkg.Vec3i, string) {
}
func (s *stubMineEnv) OnMinedBlockDuringEvent(*modelpkg.Agent, modelpkg.Vec3i, string, uint64) {}

func TestTickMineBreaksBlockAfterWorkTicks(t *testing.T) {
	pos := modelpkg.Vec3i{X: 1, Y: 0, Z: 0}
	env := &stubMineEnv{
		allowBreak: true,
		blocks:     map[modelpkg.Vec3i]uint16{pos: 5},
		air:        0,
		names:      map[uint16]string{5: "STONE"},
		drops:      map[uint16]string{},
	}
	a := &modelpkg.Agent{
		ID:           "A1",
		Pos:          modelpkg.Vec3i{X: 0, Y: 0, Z: 0},
		Inventory:    map[string]int{},
		StaminaMilli: 1000,
	}
	wt := &tasks.WorkTask{
		TaskID:   "T3",
		Kind:     tasks.KindMine,
		BlockPos: tasks.Vec3i{X: pos.X, Y: pos.Y, Z: pos.Z},
	}
	a.WorkTask = wt

	for i := 0; i < 10; i++ {
		TickMine(env, a, wt, uint64(50+i))
		if a.WorkTask == nil {
			break
		}
	}

	if a.WorkTask != nil {
		t.Fatalf("expected mine task to finish")
	}
	if got := env.blocks[pos]; got != env.air {
		t.Fatalf("expected mined block to become air, got %d", got)
	}
}
