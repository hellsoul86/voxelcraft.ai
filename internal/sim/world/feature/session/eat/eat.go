package eat

type State struct {
	HP           int
	Hunger       int
	StaminaMilli int
}

func NormalizeConsumeCount(count int) int {
	if count <= 0 {
		return 1
	}
	return count
}

func IsFood(kind string, edibleHP int) bool {
	return kind == "FOOD" && edibleHP > 0
}

func ApplyFood(state State, edibleHP int, count int) State {
	if edibleHP <= 0 || count <= 0 {
		return state
	}
	next := state
	for i := 0; i < count; i++ {
		next.HP += edibleHP
		if next.HP > 20 {
			next.HP = 20
		}
		hungerGain := edibleHP * 2
		if hungerGain < 1 {
			hungerGain = 1
		}
		next.Hunger += hungerGain
		if next.Hunger > 20 {
			next.Hunger = 20
		}
		next.StaminaMilli += edibleHP * 50
		if next.StaminaMilli > 1000 {
			next.StaminaMilli = 1000
		}
	}
	return next
}
