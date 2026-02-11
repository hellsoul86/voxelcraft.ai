package world

func (w *World) ID() string {
	if w == nil {
		return ""
	}
	return w.cfg.ID
}

func (w *World) TickRateHz() int {
	if w == nil {
		return 0
	}
	return w.cfg.TickRateHz
}
