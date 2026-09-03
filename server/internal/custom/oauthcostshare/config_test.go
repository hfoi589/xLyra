package oauthcostshare

import (
	"strings"
	"testing"

	"xlyra/server/internal/config"
)

func TestDefaultConfigUsesZeroValuesForAllPlans(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	for name, plan := range map[string]PlanConfig{
		"plus":     cfg.Plus,
		"pro_lite": cfg.ProLite,
		"pro":      cfg.Pro,
	} {
		if plan.SingleQuota != 0 || plan.ResetCount != 0 || plan.AccountFee != 0 {
			t.Fatalf("default %s config = %#v, want zero values", name, plan)
		}
	}
}

func TestReadAndSaveConfigRoundTrip(t *testing.T) {
	t.Parallel()

	confFile, err := config.LoadConfigFile(t.TempDir())
	if err != nil {
		t.Fatalf("load config file: %v", err)
	}
	want := Config{
		Plus:    PlanConfig{SingleQuota: 100, ResetCount: 1, AccountFee: 20},
		ProLite: PlanConfig{SingleQuota: 200, ResetCount: 2, AccountFee: 30},
		Pro:     PlanConfig{SingleQuota: 300, ResetCount: 3, AccountFee: 40},
	}
	if err := SaveConfig(confFile, want); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if got := ReadConfig(confFile); got != want {
		t.Fatalf("round-trip config = %#v, want %#v", got, want)
	}
}

func TestValidateConfigRejectsNegativeValuesAndNonIntegerResetCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "negative quota", cfg: Config{Plus: PlanConfig{SingleQuota: -1}}, want: "single_quota"},
		{name: "negative reset", cfg: Config{Plus: PlanConfig{ResetCount: -1}}, want: "reset_count"},
		{name: "negative fee", cfg: Config{Plus: PlanConfig{AccountFee: -1}}, want: "account_fee"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig(tc.cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateConfig error = %v, want field %q", err, tc.want)
			}
		})
	}
}

func TestNormalizePlanTypeSupportsOAuthPlanVariants(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		" PLUS ":   "plus",
		"pro_lite": "pro lite",
		"PRO-LITE": "pro lite",
		" Pro ":    "pro",
		"team":     "",
	}
	for input, want := range tests {
		if got := NormalizePlanType(input); got != want {
			t.Fatalf("NormalizePlanType(%q) = %q, want %q", input, got, want)
		}
	}
}
