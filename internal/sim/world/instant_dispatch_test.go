package world

import "testing"

func TestInstantDispatch_CoversSupportedInstantTypes(t *testing.T) {
	if len(instantDispatch) != len(supportedInstantTypes) {
		t.Fatalf("instantDispatch size mismatch: got=%d want=%d", len(instantDispatch), len(supportedInstantTypes))
	}
	allow := map[string]struct{}{}
	for _, k := range supportedInstantTypes {
		allow[k] = struct{}{}
		if instantDispatch[k] == nil {
			t.Fatalf("instantDispatch missing handler for %s", k)
		}
	}
	for k := range instantDispatch {
		if _, ok := allow[k]; !ok {
			t.Fatalf("instantDispatch has unexpected handler key %s", k)
		}
	}
}
