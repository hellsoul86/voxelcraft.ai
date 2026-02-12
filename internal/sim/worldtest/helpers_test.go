package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
)

func hasTaskFail(obs protocol.ObsMsg, wantCode string) bool {
	for _, e := range obs.Events {
		if e["type"] != "TASK_FAIL" {
			continue
		}
		if wantCode == "" {
			return true
		}
		if code, _ := e["code"].(string); code == wantCode {
			return true
		}
	}
	return false
}

func actionResultCode(obs protocol.ObsMsg, ref string) string {
	for _, e := range obs.Events {
		if typ, _ := e["type"].(string); typ != "ACTION_RESULT" {
			continue
		}
		if got, _ := e["ref"].(string); got != ref {
			continue
		}
		if ok, _ := e["ok"].(bool); ok {
			return ""
		}
		if code, _ := e["code"].(string); code != "" {
			return code
		}
		return "E_INTERNAL"
	}
	return "E_INTERNAL"
}

func actionResultFieldString(obs protocol.ObsMsg, ref string, key string) string {
	for _, e := range obs.Events {
		if typ, _ := e["type"].(string); typ != "ACTION_RESULT" {
			continue
		}
		if got, _ := e["ref"].(string); got != ref {
			continue
		}
		if s, _ := e[key].(string); s != "" {
			return s
		}
	}
	return ""
}

func invCount(inv []protocol.ItemStack, item string) int {
	for _, it := range inv {
		if it.Item == item {
			return it.Count
		}
	}
	return 0
}

func findEntityIDAt(obs protocol.ObsMsg, typ string, pos [3]int) string {
	for _, e := range obs.Entities {
		if e.Type == typ && e.Pos == pos {
			return e.ID
		}
	}
	return ""
}

func stepUntilTick(t *testing.T, h *Harness, target uint64) {
	t.Helper()
	for i := 0; i < 100000; i++ {
		if h.LastObs().Tick >= target {
			return
		}
		h.StepNoop()
	}
	t.Fatalf("stepUntilTick: exceeded iteration limit; last=%d target=%d", h.LastObs().Tick, target)
}
