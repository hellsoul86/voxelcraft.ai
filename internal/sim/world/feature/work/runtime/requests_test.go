package runtime

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type stubWorkReqEnv struct {
	nextID         string
	itemEntity     map[string]bool
	recipes        map[string]bool
	smelts         map[string]bool
	blueprints     map[string]bool
}

func (s stubWorkReqEnv) NewTaskID() string                     { return s.nextID }
func (s stubWorkReqEnv) ItemEntityExists(id string) bool       { return s.itemEntity[id] }
func (s stubWorkReqEnv) RecipeExists(id string) bool           { return s.recipes[id] }
func (s stubWorkReqEnv) SmeltExists(id string) bool            { return s.smelts[id] }
func (s stubWorkReqEnv) BlueprintExists(id string) bool        { return s.blueprints[id] }

func ar(tick uint64, ref string, ok bool, code string, message string) protocol.Event {
	e := protocol.Event{"t": tick, "ref": ref, "ok": ok}
	if code != "" {
		e["code"] = code
	}
	if message != "" {
		e["message"] = message
	}
	return e
}

func TestHandleTaskMineStartsWorkTask(t *testing.T) {
	a := &modelpkg.Agent{ID: "A1", Inventory: map[string]int{}}
	env := stubWorkReqEnv{nextID: "T1"}
	HandleTaskMine(env, ar, a, protocol.TaskReq{
		ID:       "K1",
		Type:     string(tasks.KindMine),
		BlockPos: [3]int{1, 0, 2},
	}, 10, true)
	if a.WorkTask == nil {
		t.Fatalf("expected work task")
	}
	if a.WorkTask.TaskID != "T1" || a.WorkTask.Kind != tasks.KindMine {
		t.Fatalf("unexpected task: %#v", a.WorkTask)
	}
}

func TestHandleTaskBuildBlueprintUnknownBlueprint(t *testing.T) {
	a := &modelpkg.Agent{ID: "A1", Inventory: map[string]int{}}
	env := stubWorkReqEnv{nextID: "T1", blueprints: map[string]bool{}}
	HandleTaskBuildBlueprint(env, ar, a, protocol.TaskReq{
		ID:          "K1",
		Type:        string(tasks.KindBuildBlueprint),
		BlueprintID: "missing_bp",
		Anchor:      [3]int{1, 0, 2},
	}, 10, true)
	if a.WorkTask != nil {
		t.Fatalf("expected no work task")
	}
	if len(a.Events) == 0 {
		t.Fatalf("expected action result event")
	}
	if code, _ := a.Events[len(a.Events)-1]["code"].(string); code != "E_INVALID_TARGET" {
		t.Fatalf("expected E_INVALID_TARGET, got %q", code)
	}
}
