package parser_test

import (
	"testing"

	"github.com/tamnd/gopapy/parser"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in      string
		minor   int
		wantErr bool
	}{
		{"3.8", 8, false},
		{"3.9", 9, false},
		{"3.10", 10, false},
		{"3.14", 14, false},
		{"3.0", 0, false},
		{" 3.12 ", 12, false},
		{"2.7", 0, true},
		{"3", 0, true},
		{"3.x", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		minor, err := parser.ParseVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q) = %d, want error", c.in, minor)
			}
		} else {
			if err != nil {
				t.Errorf("ParseVersion(%q): unexpected error: %v", c.in, err)
			} else if minor != c.minor {
				t.Errorf("ParseVersion(%q) = %d, want %d", c.in, minor, c.minor)
			}
		}
	}
}

func TestFeaturesFor(t *testing.T) {
	// 3.8 has walrus and pos-only, not match/case
	f38 := parser.FeaturesFor(8)
	if !f38.Has(parser.FeatWalrus) {
		t.Error("3.8 should have FeatWalrus")
	}
	if !f38.Has(parser.FeatPosOnlyParams) {
		t.Error("3.8 should have FeatPosOnlyParams")
	}
	if f38.Has(parser.FeatMatchCase) {
		t.Error("3.8 should NOT have FeatMatchCase")
	}
	if f38.Has(parser.FeatExceptStar) {
		t.Error("3.8 should NOT have FeatExceptStar")
	}

	// 3.10 has match/case
	f310 := parser.FeaturesFor(10)
	if !f310.Has(parser.FeatMatchCase) {
		t.Error("3.10 should have FeatMatchCase")
	}
	if f310.Has(parser.FeatExceptStar) {
		t.Error("3.10 should NOT have FeatExceptStar")
	}

	// 3.12 has type params
	f312 := parser.FeaturesFor(12)
	if !f312.Has(parser.FeatTypeParams) {
		t.Error("3.12 should have FeatTypeParams")
	}
	if f312.Has(parser.FeatTypeParamDefaults) {
		t.Error("3.12 should NOT have FeatTypeParamDefaults")
	}

	// 3.14 has t-strings
	f314 := parser.FeaturesFor(14)
	if !f314.Has(parser.FeatTStrings) {
		t.Error("3.14 should have FeatTStrings")
	}

	// Unknown future version falls back to latest
	fFuture := parser.FeaturesFor(99)
	if !fFuture.Has(parser.FeatTStrings) {
		t.Error("future version should have latest features")
	}

	// Features are cumulative: 3.13 includes everything up to 3.12
	f313 := parser.FeaturesFor(13)
	if !f313.Has(parser.FeatTypeParams) {
		t.Error("3.13 should include 3.12 features (FeatTypeParams)")
	}
	if !f313.Has(parser.FeatTypeParamDefaults) {
		t.Error("3.13 should have FeatTypeParamDefaults")
	}
	if f313.Has(parser.FeatTStrings) {
		t.Error("3.13 should NOT have FeatTStrings")
	}
}
