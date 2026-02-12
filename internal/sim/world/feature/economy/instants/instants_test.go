package instants

import (
	"errors"
	"testing"
)

func TestValidateOfferTradeInput(t *testing.T) {
	ok, code, _ := ValidateOfferTradeInput(false, "A2")
	if ok || code != "E_NO_PERMISSION" {
		t.Fatalf("expected disabled trade rejection")
	}
	ok, code, _ = ValidateOfferTradeInput(true, "")
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected missing recipient rejection")
	}
	ok, code, _ = ValidateOfferTradeInput(true, "A2")
	if !ok || code != "" {
		t.Fatalf("expected valid offer input")
	}
}

func TestValidateTradeOfferPairs(t *testing.T) {
	ok, code, _ := ValidateTradeOfferPairs(nil, errors.New("bad"), map[string]int{"IRON_INGOT": 1}, nil)
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected bad offer rejection")
	}
	ok, code, _ = ValidateTradeOfferPairs(map[string]int{"PLANK": 1}, nil, nil, errors.New("bad"))
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected bad request rejection")
	}
	ok, code, _ = ValidateTradeOfferPairs(map[string]int{"PLANK": 1}, nil, map[string]int{"IRON_INGOT": 1}, nil)
	if !ok || code != "" {
		t.Fatalf("expected valid pair payload")
	}
}
