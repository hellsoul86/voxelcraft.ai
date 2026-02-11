package contracts

type TickDecision string

const (
	DecisionNoop              TickDecision = "NOOP"
	DecisionFailMissingTarget TickDecision = "FAIL_MISSING_TERMINAL"
	DecisionExpireOpen        TickDecision = "EXPIRE_OPEN"
	DecisionTimeoutAccepted   TickDecision = "TIMEOUT_ACCEPTED"
	DecisionCompleteDeliver   TickDecision = "COMPLETE_DELIVER"
	DecisionCompleteBuild     TickDecision = "COMPLETE_BUILD"
)

type TickInput struct {
	State          string
	Kind           string
	NowTick        uint64
	DeadlineTick   uint64
	HasTerminal    bool
	RequirementsOK bool
	BuildPlaced    bool
	BuildStable    bool
}

func DecideTick(in TickInput) TickDecision {
	if in.State != "OPEN" && in.State != "ACCEPTED" {
		return DecisionNoop
	}
	if !in.HasTerminal {
		return DecisionFailMissingTarget
	}
	if in.State == "OPEN" {
		if in.NowTick > in.DeadlineTick {
			return DecisionExpireOpen
		}
		return DecisionNoop
	}
	// ACCEPTED
	if in.NowTick > in.DeadlineTick {
		return DecisionTimeoutAccepted
	}
	switch in.Kind {
	case "GATHER", "DELIVER":
		if in.RequirementsOK {
			return DecisionCompleteDeliver
		}
	case "BUILD":
		if in.BuildPlaced && in.BuildStable {
			return DecisionCompleteBuild
		}
	}
	return DecisionNoop
}
