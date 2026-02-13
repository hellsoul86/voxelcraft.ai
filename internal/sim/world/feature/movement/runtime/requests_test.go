package runtime

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type stubMoveReqEnv struct {
	nextID  string
	inBound bool
	follow  map[string]modelpkg.Vec3i
}

func (s stubMoveReqEnv) NewTaskID() string { return s.nextID }
func (s stubMoveReqEnv) InBounds(modelpkg.Vec3i) bool {
	return s.inBound
}
func (s stubMoveReqEnv) FollowTargetPos(targetID string) (modelpkg.Vec3i, bool) {
	v, ok := s.follow[targetID]
	return v, ok
}

func moveAR(tick uint64, ref string, ok bool, code string, message string) protocol.Event {
	ev := protocol.Event{"t": tick, "ref": ref, "ok": ok}
	if code != "" {
		ev["code"] = code
	}
	if message != "" {
		ev["message"] = message
	}
	return ev
}

func TestHandleTaskMoveToStartsTask(t *testing.T) {
	a := &modelpkg.Agent{ID: "A1", Pos: modelpkg.Vec3i{}, Inventory: map[string]int{}}
	env := stubMoveReqEnv{nextID: "T1", inBound: true}
	HandleTaskMoveTo(env, moveAR, a, protocol.TaskReq{
		ID:     "K1",
		Type:   string(tasks.KindMoveTo),
		Target: [3]int{5, 0, 6},
	}, 10)
	if a.MoveTask == nil {
		t.Fatalf("expected move task")
	}
	if a.MoveTask.TaskID != "T1" || a.MoveTask.Kind != tasks.KindMoveTo {
		t.Fatalf("unexpected move task: %#v", a.MoveTask)
	}
}

func TestHandleTaskFollowUnknownTarget(t *testing.T) {
	a := &modelpkg.Agent{ID: "A1", Pos: modelpkg.Vec3i{}, Inventory: map[string]int{}}
	env := stubMoveReqEnv{nextID: "T1", inBound: true, follow: map[string]modelpkg.Vec3i{}}
	HandleTaskFollow(env, moveAR, a, protocol.TaskReq{
		ID:       "K2",
		Type:     string(tasks.KindFollow),
		TargetID: "missing",
		Distance: 2,
	}, 12)
	if a.MoveTask != nil {
		t.Fatalf("expected no move task")
	}
	if len(a.Events) == 0 {
		t.Fatalf("expected action result event")
	}
	if got, _ := a.Events[len(a.Events)-1]["code"].(string); got != "E_INVALID_TARGET" {
		t.Fatalf("expected E_INVALID_TARGET, got %q", got)
	}
}
