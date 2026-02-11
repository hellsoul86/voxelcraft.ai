package world

import (
	"fmt"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
)

func (w *World) followTargetPos(targetID string) (Vec3i, bool) {
	if targetID == "" {
		return Vec3i{}, false
	}
	if a := w.agents[targetID]; a != nil {
		return a.Pos, true
	}
	if c := w.getContainerByID(targetID); c != nil {
		return c.Pos, true
	}
	return Vec3i{}, false
}

func (w *World) newTaskID() string {
	n := w.nextTaskNum.Add(1)
	return fmt.Sprintf("T%06d", n)
}

func sendLatest(ch chan []byte, b []byte) {
	select {
	case ch <- b:
		return
	default:
	}
	// Drop one.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- b:
	default:
	}
}

func (w *World) broadcastChat(tick uint64, from *Agent, channel string, text string) {
	for _, a := range w.agents {
		switch channel {
		case "LOCAL":
			if Manhattan(a.Pos, from.Pos) > 32 {
				continue
			}
		case "CITY":
			if from.OrgID == "" || !w.isOrgMember(a.ID, from.OrgID) {
				continue
			}
		}
		a.AddEvent(protocol.Event{
			"t":       tick,
			"type":    "CHAT",
			"from":    from.ID,
			"channel": channel,
			"text":    text,
		})
	}
}

func v3FromTask(v tasks.Vec3i) Vec3i {
	return Vec3i{X: v.X, Y: v.Y, Z: v.Z}
}

func v3ToTask(v Vec3i) tasks.Vec3i {
	return tasks.Vec3i{X: v.X, Y: v.Y, Z: v.Z}
}
