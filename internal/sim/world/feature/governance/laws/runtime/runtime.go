package runtime

type TransitionInput struct {
	Status         string
	NowTick        uint64
	NoticeEndsTick uint64
	VoteEndsTick   uint64
}

type Transition struct {
	ShouldTransition bool
	NextStatus       string
	EventKind        string
}

func NextTransition(in TransitionInput) Transition {
	switch in.Status {
	case "NOTICE":
		if in.NowTick >= in.NoticeEndsTick {
			return Transition{ShouldTransition: true, NextStatus: "VOTING", EventKind: "VOTING"}
		}
	case "VOTING":
		if in.NowTick >= in.VoteEndsTick {
			return Transition{ShouldTransition: true}
		}
	}
	return Transition{}
}

func VotePassed(yes int, no int) bool {
	return yes > no
}
