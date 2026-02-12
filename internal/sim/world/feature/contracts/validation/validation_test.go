package validation

import "testing"

func TestValidateAccept(t *testing.T) {
	ok, _, _ := ValidateAccept(AcceptValidationInput{
		HasContract:      true,
		State:            "OPEN",
		TerminalMatch:    true,
		Distance:         1,
		NowTick:          10,
		DeadlineTick:     20,
		HasEnoughDeposit: true,
	})
	if !ok {
		t.Fatalf("expected valid accept")
	}
	ok, code, _ := ValidateAccept(AcceptValidationInput{
		HasContract:      true,
		State:            "OPEN",
		TerminalMatch:    true,
		Distance:         1,
		NowTick:          21,
		DeadlineTick:     20,
		HasEnoughDeposit: true,
	})
	if ok || code != "E_CONFLICT" {
		t.Fatalf("expected expired conflict, got ok=%v code=%s", ok, code)
	}
}

func TestValidateSubmit(t *testing.T) {
	ok, _, _ := ValidateSubmit(SubmitValidationInput{
		HasContract:   true,
		State:         "ACCEPTED",
		IsAcceptor:    true,
		TerminalMatch: true,
		Distance:      2,
		NowTick:       5,
		DeadlineTick:  10,
		CanSubmit:     true,
	})
	if !ok {
		t.Fatalf("expected valid submit")
	}
	ok, code, _ := ValidateSubmit(SubmitValidationInput{
		HasContract:   true,
		State:         "ACCEPTED",
		IsAcceptor:    true,
		TerminalMatch: true,
		Distance:      2,
		NowTick:       5,
		DeadlineTick:  10,
		CanSubmit:     false,
	})
	if ok || code != "E_BLOCKED" {
		t.Fatalf("expected blocked submit, got ok=%v code=%s", ok, code)
	}
}

func TestValidatePost(t *testing.T) {
	ok, _, _ := ValidatePost(PostValidationInput{
		TerminalID:      "T1",
		HasTerminal:     true,
		Distance:        1,
		Kind:            "BUILD",
		Requirements:    nil,
		Reward:          map[string]int{"PLANK": 2},
		IsBuild:         true,
		BlueprintID:     "bp1",
		HasEnoughReward: true,
	})
	if !ok {
		t.Fatalf("expected valid post")
	}
	ok, code, _ := ValidatePost(PostValidationInput{
		TerminalID:      "T1",
		HasTerminal:     true,
		Distance:        1,
		Kind:            "GATHER",
		Requirements:    map[string]int{"STONE": 2},
		Reward:          map[string]int{"PLANK": 2},
		IsBuild:         false,
		HasEnoughReward: false,
	})
	if ok || code != "E_NO_RESOURCE" {
		t.Fatalf("expected E_NO_RESOURCE, got ok=%v code=%s", ok, code)
	}
}

func TestPreparePost(t *testing.T) {
	out := PreparePost(PostPrepInput{
		TerminalID:      "CONTRACT_TERMINAL@1,0,1",
		TerminalType:    "CONTRACT_TERMINAL",
		Distance:        2,
		Kind:            "build",
		Requirements:    map[string]int{"PLANK": 4},
		Reward:          map[string]int{"IRON_INGOT": 2},
		BlueprintID:     "workshop_pad",
		HasEnoughReward: true,
		NowTick:         10,
		DurationTicks:   100,
		DayTicks:        6000,
	})
	if out.ResolvedKind != "BUILD" {
		t.Fatalf("expected normalized kind BUILD, got %q", out.ResolvedKind)
	}
	if !out.Validation.HasTerminal || !out.Validation.IsBuild {
		t.Fatalf("expected terminal/build flags on validation input")
	}
	if out.Deadline != 110 {
		t.Fatalf("unexpected deadline: %d", out.Deadline)
	}
}

func TestPrepareAccept(t *testing.T) {
	out := PrepareAccept(AcceptPrepInput{
		HasContract:     true,
		State:           "OPEN",
		TerminalType:    "CONTRACT_TERMINAL",
		TerminalMatches: true,
		Distance:        1,
		NowTick:         100,
		DeadlineTick:    200,
		BaseDeposit:     map[string]int{"IRON_INGOT": 2},
		DepositMult:     2,
		Inventory:       map[string]int{"IRON_INGOT": 3},
	})
	if out.Validation.HasEnoughDeposit {
		t.Fatalf("expected insufficient deposit after scaling")
	}
	if out.RequiredDeposit["IRON_INGOT"] != 4 {
		t.Fatalf("expected scaled deposit to 4")
	}
}

func TestPrepareSubmitValidation(t *testing.T) {
	in := SubmitPrepInput{
		HasContract:     true,
		State:           "ACCEPTED",
		IsAcceptor:      true,
		TerminalType:    "CONTRACT_TERMINAL",
		TerminalMatches: true,
		Distance:        1,
		NowTick:         100,
		DeadlineTick:    200,
		Kind:            "DELIVER",
		RequirementsOK:  true,
	}
	v := PrepareSubmitValidation(in)
	if !v.CanSubmit || !v.TerminalMatch {
		t.Fatalf("expected submit validation to allow completion")
	}
}
