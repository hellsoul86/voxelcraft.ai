package validation

import (
	"strings"

	"voxelcraft.ai/internal/sim/world/feature/contracts/core"
	lc "voxelcraft.ai/internal/sim/world/feature/contracts/lifecycle"
)

type PostValidationInput struct {
	TerminalID      string
	HasTerminal     bool
	Distance        int
	Kind            string
	Requirements    map[string]int
	Reward          map[string]int
	IsBuild         bool
	BlueprintID     string
	HasEnoughReward bool
}

func ValidatePost(in PostValidationInput) (ok bool, code string, msg string) {
	if in.TerminalID == "" {
		return false, "E_BAD_REQUEST", "missing terminal_id"
	}
	if !in.HasTerminal {
		return false, "E_INVALID_TARGET", "contract terminal not found"
	}
	if in.Distance > 3 {
		return false, "E_BLOCKED", "too far"
	}
	if in.Kind == "" {
		return false, "E_BAD_REQUEST", "bad contract_kind"
	}
	if ok, code, msg := lc.ValidatePostInput(in.Kind, in.Requirements, in.Reward); !ok {
		return false, code, msg
	}
	if in.IsBuild && in.BlueprintID == "" {
		return false, "E_BAD_REQUEST", "missing blueprint_id"
	}
	if !in.HasEnoughReward {
		return false, "E_NO_RESOURCE", "insufficient reward items"
	}
	return true, "", ""
}

type AcceptValidationInput struct {
	HasContract      bool
	State            string
	TerminalMatch    bool
	Distance         int
	NowTick          uint64
	DeadlineTick     uint64
	HasEnoughDeposit bool
}

func ValidateAccept(in AcceptValidationInput) (ok bool, code string, msg string) {
	if !in.HasContract {
		return false, "E_INVALID_TARGET", "contract not found"
	}
	if in.State != "OPEN" {
		return false, "E_CONFLICT", "contract not open"
	}
	if !in.TerminalMatch {
		return false, "E_INVALID_TARGET", "terminal mismatch"
	}
	if in.Distance > 3 {
		return false, "E_BLOCKED", "too far"
	}
	if in.NowTick > in.DeadlineTick {
		return false, "E_CONFLICT", "contract expired"
	}
	if !in.HasEnoughDeposit {
		return false, "E_NO_RESOURCE", "insufficient deposit"
	}
	return true, "", ""
}

type SubmitValidationInput struct {
	HasContract   bool
	State         string
	IsAcceptor    bool
	TerminalMatch bool
	Distance      int
	NowTick       uint64
	DeadlineTick  uint64
	CanSubmit     bool
}

func ValidateSubmit(in SubmitValidationInput) (ok bool, code string, msg string) {
	if !in.HasContract {
		return false, "E_INVALID_TARGET", "contract not found"
	}
	if in.State != "ACCEPTED" || !in.IsAcceptor {
		return false, "E_NO_PERMISSION", "not acceptor"
	}
	if !in.TerminalMatch {
		return false, "E_INVALID_TARGET", "terminal mismatch"
	}
	if in.Distance > 3 {
		return false, "E_BLOCKED", "too far"
	}
	if in.NowTick > in.DeadlineTick {
		return false, "E_CONFLICT", "contract expired"
	}
	if !in.CanSubmit {
		return false, "E_BLOCKED", "requirements not met"
	}
	return true, "", ""
}

type PostPrepInput struct {
	TerminalID      string
	TerminalType    string
	Distance        int
	Kind            string
	Requirements    map[string]int
	Reward          map[string]int
	BlueprintID     string
	HasEnoughReward bool
	NowTick         uint64
	DeadlineTick    uint64
	DurationTicks   int
	DayTicks        int
}

type PostPrepResult struct {
	Validation   PostValidationInput
	ResolvedKind string
	Deadline     uint64
}

func PreparePost(in PostPrepInput) PostPrepResult {
	kind := core.NormalizeKind(in.Kind)
	return PostPrepResult{
		Validation: PostValidationInput{
			TerminalID:      in.TerminalID,
			HasTerminal:     in.TerminalType == "CONTRACT_TERMINAL",
			Distance:        in.Distance,
			Kind:            kind,
			Requirements:    in.Requirements,
			Reward:          in.Reward,
			IsBuild:         kind == "BUILD",
			BlueprintID:     in.BlueprintID,
			HasEnoughReward: in.HasEnoughReward,
		},
		ResolvedKind: kind,
		Deadline:     lc.BuildDeadline(in.NowTick, in.DeadlineTick, in.DurationTicks, in.DayTicks),
	}
}

func ValidateLifecycleIDs(contractID, terminalID string) (ok bool, code string, msg string) {
	if strings.TrimSpace(contractID) == "" || strings.TrimSpace(terminalID) == "" {
		return false, "E_BAD_REQUEST", "missing contract_id/terminal_id"
	}
	return true, "", ""
}

type AcceptPrepInput struct {
	HasContract     bool
	State           string
	TerminalType    string
	TerminalMatches bool
	Distance        int
	NowTick         uint64
	DeadlineTick    uint64
	BaseDeposit     map[string]int
	DepositMult     int
	Inventory       map[string]int
}

type AcceptPrepResult struct {
	Validation      AcceptValidationInput
	RequiredDeposit map[string]int
}

func PrepareAccept(in AcceptPrepInput) AcceptPrepResult {
	required := map[string]int(nil)
	hasEnough := true
	if in.HasContract {
		required = lc.ScaleDeposit(in.BaseDeposit, in.DepositMult)
		for item, n := range required {
			if in.Inventory[item] < n {
				hasEnough = false
				break
			}
		}
	}
	return AcceptPrepResult{
		Validation: AcceptValidationInput{
			HasContract:      in.HasContract,
			State:            in.State,
			TerminalMatch:    in.TerminalType == "CONTRACT_TERMINAL" && in.TerminalMatches,
			Distance:         in.Distance,
			NowTick:          in.NowTick,
			DeadlineTick:     in.DeadlineTick,
			HasEnoughDeposit: hasEnough,
		},
		RequiredDeposit: required,
	}
}

type SubmitPrepInput struct {
	HasContract     bool
	State           string
	IsAcceptor      bool
	TerminalType    string
	TerminalMatches bool
	Distance        int
	NowTick         uint64
	DeadlineTick    uint64
	Kind            string
	RequirementsOK  bool
	BuildOK         bool
}

func PrepareSubmitValidation(in SubmitPrepInput) SubmitValidationInput {
	return SubmitValidationInput{
		HasContract:   in.HasContract,
		State:         in.State,
		IsAcceptor:    in.IsAcceptor,
		TerminalMatch: in.TerminalType == "CONTRACT_TERMINAL" && in.TerminalMatches,
		Distance:      in.Distance,
		NowTick:       in.NowTick,
		DeadlineTick:  in.DeadlineTick,
		CanSubmit:     lc.CanSubmit(in.Kind, in.RequirementsOK, in.BuildOK),
	}
}
