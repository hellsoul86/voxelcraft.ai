package targets

import "testing"

func TestValidatePhysicalBoardTarget(t *testing.T) {
	ok, code, _ := ValidatePhysicalBoardTarget("CHEST", "BULLETIN_BOARD", 1, true)
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected board type guard")
	}
	ok, code, _ = ValidatePhysicalBoardTarget("BULLETIN_BOARD", "BULLETIN_BOARD", 4, true)
	if ok || code != "E_BLOCKED" {
		t.Fatalf("expected distance guard")
	}
	ok, code, _ = ValidatePhysicalBoardTarget("BULLETIN_BOARD", "BULLETIN_BOARD", 1, true)
	if !ok || code != "" {
		t.Fatalf("expected valid board target")
	}
}

func TestValidateSetSignTarget(t *testing.T) {
	ok, code, _ := ValidateSetSignTarget("CHEST", "SIGN", 1, 3)
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected sign target guard")
	}
	ok, code, _ = ValidateSetSignTarget("SIGN", "SIGN", 1, 250)
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected sign text size guard")
	}
	ok, code, _ = ValidateSetSignTarget("SIGN", "SIGN", 1, 50)
	if !ok || code != "" {
		t.Fatalf("expected valid sign target")
	}
}
