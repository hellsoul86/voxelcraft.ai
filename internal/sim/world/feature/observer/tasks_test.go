package observer

import "testing"

func TestBuildTasks_Move(t *testing.T) {
	tasks := BuildTasks(BuildTasksInput{
		SelfPos: TaskVec3{X: 1, Y: 0, Z: 0},
		Move: &MoveTaskInput{
			TaskID:    "T1",
			Kind:      "MOVE_TO",
			Target:    TaskVec3{X: 4, Y: 0, Z: 0},
			StartPos:  TaskVec3{X: 0, Y: 0, Z: 0},
			Tolerance: 1,
		},
	}, nil)
	if len(tasks) != 1 || tasks[0].TaskID != "T1" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestBuildTasks_FollowResolve(t *testing.T) {
	tasks := BuildTasks(BuildTasksInput{
		SelfPos: TaskVec3{X: 0, Y: 0, Z: 0},
		Move: &MoveTaskInput{
			TaskID:   "T2",
			Kind:     "FOLLOW",
			TargetID: "A2",
			Distance: 2,
			Target:   TaskVec3{X: 100, Y: 0, Z: 100},
		},
	}, func(id string) (TaskVec3, bool) {
		if id == "A2" {
			return TaskVec3{X: 1, Y: 0, Z: 1}, true
		}
		return TaskVec3{}, false
	})
	if len(tasks) != 1 || tasks[0].Target[0] != 1 || tasks[0].Target[2] != 1 {
		t.Fatalf("expected resolved follow target, got %#v", tasks)
	}
}
