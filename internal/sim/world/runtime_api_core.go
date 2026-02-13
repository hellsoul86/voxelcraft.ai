package world

import "voxelcraft.ai/internal/persistence/snapshot"

func (w *World) SetTickLogger(l TickLogger)                    { w.tickLogger = l }
func (w *World) SetAuditLogger(l AuditLogger)                  { w.auditLogger = l }
func (w *World) SetSnapshotSink(ch chan<- snapshot.SnapshotV1) { w.snapshotSink = ch }

func (w *World) ExportSnapshot(nowTick uint64) snapshot.SnapshotV1 {
	return w.exportSnapshot(nowTick)
}

// ImportSnapshot replaces the current in-memory world state with the snapshot.
// It sets the world's tick to snapshotTick+1 (the next tick to simulate).
//
// This must be called only when the world is stopped or from the world loop goroutine.
func (w *World) ImportSnapshot(s snapshot.SnapshotV1) error {
	return w.importSnapshotV1(s)
}

func (w *World) Inbox() chan<- ActionEnvelope { return w.inbox }
func (w *World) Join() chan<- JoinRequest     { return w.join }
func (w *World) Attach() chan<- AttachRequest { return w.attach }
func (w *World) Leave() chan<- string         { return w.leave }

func (w *World) ObserverJoin() chan<- ObserverJoinRequest           { return w.observerJoin }
func (w *World) ObserverSubscribe() chan<- ObserverSubscribeRequest { return w.observerSub }
func (w *World) ObserverLeave() chan<- string                       { return w.observerLeave }

func (w *World) CurrentTick() uint64 { return w.tick.Load() }

func (w *World) systemMovement(nowTick uint64) { w.systemMovementImpl(nowTick) }
func (w *World) systemWork(nowTick uint64)     { w.systemWorkImpl(nowTick) }
