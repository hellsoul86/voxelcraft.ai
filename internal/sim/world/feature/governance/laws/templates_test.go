package laws

import (
	"errors"
	"testing"
)

func TestNormalizeLawParams(t *testing.T) {
	itemExists := func(item string) bool { return item == "IRON_INGOT" }

	tests := []struct {
		name       string
		templateID string
		params     map[string]interface{}
		wantErr    bool
	}{
		{
			name:       "market tax",
			templateID: "MARKET_TAX",
			params:     map[string]interface{}{"market_tax": 0.31},
		},
		{
			name:       "curfew",
			templateID: "CURFEW_NO_BUILD",
			params:     map[string]interface{}{"start_time": 0.2, "end_time": 0.4},
		},
		{
			name:       "fine unknown item",
			templateID: "FINE_BREAK_PER_BLOCK",
			params:     map[string]interface{}{"fine_item": "WOOD", "fine_per_block": 1},
			wantErr:    true,
		},
		{
			name:       "access pass",
			templateID: "ACCESS_PASS_CORE",
			params:     map[string]interface{}{"ticket_item": "IRON_INGOT", "ticket_cost": 12},
		},
		{
			name:       "unsupported template",
			templateID: "NO_SUCH",
			params:     map[string]interface{}{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeLawParams(tt.templateID, tt.params, itemExists)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	_, err := NormalizeLawParams("NO_SUCH", map[string]interface{}{}, itemExists)
	if !errors.Is(err, ErrUnsupportedLawTemplate) {
		t.Fatalf("want ErrUnsupportedLawTemplate, got %v", err)
	}
}

func TestApplyLawTemplate(t *testing.T) {
	base := LandState{}
	out, err := ApplyLawTemplate("MARKET_TAX", map[string]string{"market_tax": "0.3"}, base)
	if err != nil {
		t.Fatalf("apply market tax: %v", err)
	}
	if out.MarketTax != 0.25 {
		t.Fatalf("market tax clamp failed: got %v", out.MarketTax)
	}

	out, err = ApplyLawTemplate("CURFEW_NO_BUILD", map[string]string{"start_time": "0.2", "end_time": "0.8"}, base)
	if err != nil {
		t.Fatalf("apply curfew: %v", err)
	}
	if !out.CurfewEnabled || out.CurfewStart != 0.2 || out.CurfewEnd != 0.8 {
		t.Fatalf("curfew fields mismatch: %+v", out)
	}

	out, err = ApplyLawTemplate("FINE_BREAK_PER_BLOCK", map[string]string{"fine_item": "IRON_INGOT", "fine_per_block": "9"}, base)
	if err != nil {
		t.Fatalf("apply fine break: %v", err)
	}
	if !out.FineBreakEnabled || out.FineBreakItem != "IRON_INGOT" || out.FineBreakPerBlock != 9 {
		t.Fatalf("fine break mismatch: %+v", out)
	}

	out, err = ApplyLawTemplate("ACCESS_PASS_CORE", map[string]string{"ticket_item": "IRON_INGOT", "ticket_cost": "11"}, base)
	if err != nil {
		t.Fatalf("apply access pass: %v", err)
	}
	if !out.AccessPassEnabled || out.AccessTicketItem != "IRON_INGOT" || out.AccessTicketCost != 11 {
		t.Fatalf("access pass mismatch: %+v", out)
	}

	_, err = ApplyLawTemplate("NO_SUCH", map[string]string{}, base)
	if !errors.Is(err, ErrUnsupportedLawTemplate) {
		t.Fatalf("want ErrUnsupportedLawTemplate, got %v", err)
	}
}
