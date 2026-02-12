package runtime

func HungerAfterTick(hunger int, inBlight bool) int {
	if hunger <= 0 {
		return 0
	}
	next := hunger - 1
	if inBlight {
		next--
	}
	if next < 0 {
		next = 0
	}
	return next
}

func IsNight(timeOfDay float64) bool {
	return timeOfDay < 0.25 || timeOfDay > 0.75
}

func StaminaRecovery(weather string, hunger int, eventID string, inEventRadius bool) int {
	rec := 2
	if weather == "STORM" || weather == "COLD" {
		rec = 1
	}
	if hunger == 0 {
		rec = 0
	} else if hunger < 5 && rec > 1 {
		rec = 1
	}
	if inEventRadius {
		switch eventID {
		case "BLIGHT_ZONE":
			rec = 0
		case "FLOOD_WARNING":
			if rec > 1 {
				rec = 1
			}
		}
	}
	return rec
}
