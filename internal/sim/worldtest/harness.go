package worldtest

import (
	"encoding/json"
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

// Harness is a small black-box test helper for driving a world via exported APIs:
// - Join() issues JoinRequest via StepOnce()
// - Step()/StepFor() issues ACT via StepOnce()
// - Per-agent Out channels carry OBS JSON
// - ExportSnapshot/Debug* helpers provide deterministic preconditions
//
// It intentionally avoids touching world internals so tests can live outside the world package.
type Harness struct {
	T    *testing.T
	Cats *catalogs.Catalogs
	W    *world.World

	DefaultAgentID string

	sessions map[string]*session
}

func NewHarness(t *testing.T, cfg world.WorldConfig, cats *catalogs.Catalogs, agentName string) *Harness {
	t.Helper()

	w, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world.New: %v", err)
	}

	h := &Harness{
		T:        t,
		Cats:     cats,
		W:        w,
		sessions: map[string]*session{},
	}
	h.DefaultAgentID = h.Join(agentName)
	return h
}

// NewHarnessWithWorld is like NewHarness, but uses an already-constructed world instance.
// This is useful for snapshot round-trip tests where the snapshot is imported before join.
func NewHarnessWithWorld(t *testing.T, w *world.World, cats *catalogs.Catalogs, agentName string) *Harness {
	t.Helper()
	if w == nil {
		t.Fatalf("NewHarnessWithWorld: nil world")
	}

	h := &Harness{
		T:        t,
		Cats:     cats,
		W:        w,
		sessions: map[string]*session{},
	}
	h.DefaultAgentID = h.Join(agentName)
	return h
}

type session struct {
	AgentID string
	Out     chan []byte
	lastObs protocol.ObsMsg
}

func (h *Harness) Join(agentName string) string {
	h.T.Helper()

	out := make(chan []byte, 16)
	resp := make(chan world.JoinResponse, 1)
	_, _ = h.W.StepOnce([]world.JoinRequest{{
		Name:        agentName,
		DeltaVoxels: false,
		Out:         out,
		Resp:        resp,
	}}, nil, nil)
	jr := <-resp
	if jr.Welcome.AgentID == "" {
		h.T.Fatalf("join returned empty agent id")
	}
	s := &session{AgentID: jr.Welcome.AgentID, Out: out}
	h.sessions[s.AgentID] = s
	h.drainAllObs()
	return s.AgentID
}

func (h *Harness) LastObs() protocol.ObsMsg {
	return h.LastObsFor(h.DefaultAgentID)
}

func (h *Harness) LastObsFor(agentID string) protocol.ObsMsg {
	h.T.Helper()
	s := h.sessions[agentID]
	if s == nil {
		h.T.Fatalf("unknown agent id: %q", agentID)
	}
	return s.lastObs
}

func (h *Harness) Step(instants []protocol.InstantReq, tasks []protocol.TaskReq, cancel []string) protocol.ObsMsg {
	return h.StepFor(h.DefaultAgentID, instants, tasks, cancel)
}

func (h *Harness) StepFor(agentID string, instants []protocol.InstantReq, tasks []protocol.TaskReq, cancel []string) protocol.ObsMsg {
	h.T.Helper()
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            h.W.CurrentTick(),
		AgentID:         agentID,
		Instants:        instants,
		Tasks:           tasks,
		Cancel:          cancel,
	}
	_, _ = h.W.StepOnce(nil, nil, []world.ActionEnvelope{{
		AgentID: agentID,
		Act:     act,
	}})
	h.drainAllObs()
	return h.LastObsFor(agentID)
}

func (h *Harness) StepMulti(actions []world.ActionEnvelope) {
	h.T.Helper()
	_, _ = h.W.StepOnce(nil, nil, actions)
	h.drainAllObs()
}

func (h *Harness) StepNoop() protocol.ObsMsg {
	h.T.Helper()
	_, _ = h.W.StepOnce(nil, nil, nil)
	h.drainAllObs()
	return h.LastObs()
}

func (h *Harness) Snapshot() (tick uint64, snap snapshot.SnapshotV1) {
	h.T.Helper()
	// Keep tick stable: export at currentTick-1 then import would restore to currentTick.
	cur := h.W.CurrentTick()
	if cur == 0 {
		return 0, h.W.ExportSnapshot(0)
	}
	tick = cur - 1
	return tick, h.W.ExportSnapshot(tick)
}

func (h *Harness) SetAgentPos(pos world.Vec3i) {
	h.SetAgentPosFor(h.DefaultAgentID, pos)
}

func (h *Harness) SetAgentPosFor(agentID string, pos world.Vec3i) {
	h.T.Helper()
	if ok := h.W.DebugSetAgentPos(agentID, pos); !ok {
		h.T.Fatalf("DebugSetAgentPos returned false")
	}
}

func (h *Harness) ClearAgentEventsFor(agentID string) {
	h.T.Helper()
	if ok := h.W.DebugClearAgentEvents(agentID); !ok {
		h.T.Fatalf("DebugClearAgentEvents returned false")
	}
}

func (h *Harness) ClearAgentEvents() {
	h.ClearAgentEventsFor(h.DefaultAgentID)
}

func (h *Harness) AddInventoryFor(agentID string, item string, delta int) {
	h.T.Helper()
	if ok := h.W.DebugAddInventory(agentID, item, delta); !ok {
		h.T.Fatalf("DebugAddInventory returned false")
	}
}

func (h *Harness) AddInventory(item string, delta int) {
	h.AddInventoryFor(h.DefaultAgentID, item, delta)
}

func (h *Harness) SetBlock(pos world.Vec3i, blockName string) {
	h.T.Helper()
	if err := h.W.DebugSetBlock(pos, blockName); err != nil {
		h.T.Fatalf("DebugSetBlock: %v", err)
	}
}

func (h *Harness) drainAllObs() {
	h.T.Helper()
	for _, s := range h.sessions {
		h.drainOneObs(s)
	}
}

func (h *Harness) drainOneObs(s *session) {
	h.T.Helper()
	var last []byte
	for {
		select {
		case b := <-s.Out:
			last = b
			continue
		default:
		}
		break
	}
	if len(last) == 0 {
		return
	}
	var obs protocol.ObsMsg
	if err := json.Unmarshal(last, &obs); err != nil {
		h.T.Fatalf("unmarshal OBS: %v", err)
	}
	s.lastObs = obs
}
