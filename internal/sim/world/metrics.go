package world

// WorldMetrics is a thread-safe read-only view of key world runtime signals.
// It is updated from the world loop goroutine and read from HTTP handlers/tests.
type WorldMetrics struct {
	Tick uint64 `json:"tick"`

	Agents       int    `json:"agents"`
	Clients      int    `json:"clients"`
	LoadedChunks int    `json:"loaded_chunks"`
	ResetTotal   uint64 `json:"reset_total"`

	QueueDepths QueueDepths `json:"queue_depths"`

	StepMS float64 `json:"step_ms"`

	StatsWindowTicks uint64      `json:"stats_window_ticks"`
	StatsWindow      StatsBucket `json:"stats_window"`

	Director DirectorMetrics `json:"director_metrics"`

	ResourceDensity map[string]float64 `json:"resource_density,omitempty"`

	Weather          string `json:"weather"`
	WeatherUntilTick uint64 `json:"weather_until_tick"`

	ActiveEventID     string `json:"active_event_id"`
	ActiveEventStart  uint64 `json:"active_event_start_tick"`
	ActiveEventEnds   uint64 `json:"active_event_ends_tick"`
	ActiveEventCenter [3]int `json:"active_event_center"`
	ActiveEventRadius int    `json:"active_event_radius"`
}

type QueueDepths struct {
	Inbox  int `json:"inbox"`
	Join   int `json:"join"`
	Leave  int `json:"leave"`
	Attach int `json:"attach"`
}

type DirectorMetrics struct {
	Trade       float64 `json:"trade"`
	Conflict    float64 `json:"conflict"`
	Exploration float64 `json:"exploration"`
	Inequality  float64 `json:"inequality"`
	PublicInfra float64 `json:"public_infra"`
}

func (w *World) Metrics() WorldMetrics {
	if w == nil {
		return WorldMetrics{}
	}
	v := w.metrics.Load()
	if v == nil {
		return WorldMetrics{}
	}
	m, ok := v.(WorldMetrics)
	if !ok {
		return WorldMetrics{}
	}
	return m
}
