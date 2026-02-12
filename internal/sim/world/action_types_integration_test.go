package world

import "testing"

func TestValidateDispatchMap_OK(t *testing.T) {
	handlers := map[string]int{"A": 1, "B": 2}
	supported := []string{"A", "B"}
	if err := validateDispatchMap("test", handlers, supported); err != nil {
		t.Fatalf("validateDispatchMap returned error: %v", err)
	}
}

func TestValidateDispatchMap_DetectsMismatch(t *testing.T) {
	handlers := map[string]int{"A": 1, "C": 3}
	supported := []string{"A", "B"}
	if err := validateDispatchMap("test", handlers, supported); err == nil {
		t.Fatalf("expected mismatch error, got nil")
	}
}

func TestValidateActionDispatchMaps_CurrentMapsValid(t *testing.T) {
	if err := validateActionDispatchMaps(); err != nil {
		t.Fatalf("current dispatch maps invalid: %v", err)
	}
}
