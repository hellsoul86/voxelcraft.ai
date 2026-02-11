package world

func (w *World) auditSetBlock(tick uint64, actor string, pos Vec3i, from, to uint16, reason string) {
	entry := AuditEntry{
		Tick:   tick,
		Actor:  actor,
		Action: "SET_BLOCK",
		Pos:    pos.ToArray(),
		From:   from,
		To:     to,
		Reason: reason,
	}
	if w.auditLogger != nil {
		_ = w.auditLogger.WriteAudit(entry)
	}
	if len(w.observers) > 0 {
		w.obsAuditsThisTick = append(w.obsAuditsThisTick, entry)
	}
}

func (w *World) auditEvent(tick uint64, actor string, action string, pos Vec3i, reason string, details map[string]any) {
	if w.auditLogger == nil {
		return
	}
	_ = w.auditLogger.WriteAudit(AuditEntry{
		Tick:    tick,
		Actor:   actor,
		Action:  action,
		Pos:     pos.ToArray(),
		Reason:  reason,
		Details: details,
	})
}
