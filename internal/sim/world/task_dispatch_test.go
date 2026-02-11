package world

import "testing"

func TestTaskReqDispatch_CoversSupportedTaskTypes(t *testing.T) {
	if len(taskReqDispatch) != len(supportedTaskReqTypes) {
		t.Fatalf("taskReqDispatch size mismatch: got=%d want=%d", len(taskReqDispatch), len(supportedTaskReqTypes))
	}
	allow := map[string]struct{}{}
	for _, k := range supportedTaskReqTypes {
		allow[k] = struct{}{}
		if taskReqDispatch[k] == nil {
			t.Fatalf("taskReqDispatch missing handler for %s", k)
		}
	}
	for k := range taskReqDispatch {
		if _, ok := allow[k]; !ok {
			t.Fatalf("taskReqDispatch has unexpected handler key %s", k)
		}
	}
}
