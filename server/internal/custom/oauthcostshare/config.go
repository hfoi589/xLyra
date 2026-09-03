package oauthcostshare

import (
	"fmt"
	"math"
	"strings"

	"xlyra/server/internal/config"
)

const oauthCostShareConfigPath = "global.oauth_cost_share"

type PlanConfig struct {
	SingleQuota float64 `json:"single_quota"`
	ResetCount  int     `json:"reset_count"`
	AccountFee  float64 `json:"account_fee"`
}

type Config struct {
	Plus    PlanConfig `json:"plus"`
	ProLite PlanConfig `json:"pro_lite"`
	Pro     PlanConfig `json:"pro"`
}

func DefaultConfig() Config {
	return Config{}
}

func ReadConfig(confFile *config.ConfigFile) Config {
	if confFile == nil {
		return DefaultConfig()
	}
	raw, ok := confFile.Get(oauthCostShareConfigPath)
	if !ok {
		return DefaultConfig()
	}
	return ConfigFromRaw(raw)
}

func ConfigFromRaw(raw any) Config {
	defaults := DefaultConfig()
	root, ok := raw.(map[string]any)
	if !ok {
		return defaults
	}
	return Config{
		Plus:    planConfigFromRaw(root["plus"], defaults.Plus),
		ProLite: planConfigFromRaw(root["pro_lite"], defaults.ProLite),
		Pro:     planConfigFromRaw(root["pro"], defaults.Pro),
	}
}

func SaveConfig(confFile *config.ConfigFile, cfg Config) error {
	if confFile == nil {
		return fmt.Errorf("oauth cost share config persistence is not available")
	}
	if err := ValidateConfig(cfg); err != nil {
		return err
	}
	return confFile.Set(oauthCostShareConfigPath, configToMap(cfg))
}

func ValidateConfig(cfg Config) error {
	for name, plan := range map[string]PlanConfig{
		"plus":     cfg.Plus,
		"pro_lite": cfg.ProLite,
		"pro":      cfg.Pro,
	} {
		if math.IsNaN(plan.SingleQuota) || math.IsInf(plan.SingleQuota, 0) || plan.SingleQuota < 0 {
			return fmt.Errorf("%s.single_quota must be non-negative", name)
		}
		if plan.ResetCount < 0 {
			return fmt.Errorf("%s.reset_count must be non-negative", name)
		}
		if math.IsNaN(plan.AccountFee) || math.IsInf(plan.AccountFee, 0) || plan.AccountFee < 0 {
			return fmt.Errorf("%s.account_fee must be non-negative", name)
		}
	}
	return nil
}

func NormalizePlanType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.NewReplacer("_", " ", "-", " ").Replace(normalized)
	normalized = strings.Join(strings.Fields(normalized), " ")
	switch normalized {
	case "plus", "pro", "pro lite":
		return normalized
	default:
		return ""
	}
}

func configToMap(cfg Config) map[string]any {
	return map[string]any{
		"plus":     planConfigToMap(cfg.Plus),
		"pro_lite": planConfigToMap(cfg.ProLite),
		"pro":      planConfigToMap(cfg.Pro),
	}
}

func planConfigToMap(plan PlanConfig) map[string]any {
	return map[string]any{
		"single_quota": plan.SingleQuota,
		"reset_count":  plan.ResetCount,
		"account_fee":  plan.AccountFee,
	}
}

func planConfigFromRaw(raw any, fallback PlanConfig) PlanConfig {
	if plan, ok := raw.(PlanConfig); ok {
		return plan
	}
	root, ok := raw.(map[string]any)
	if !ok {
		return fallback
	}
	return PlanConfig{
		SingleQuota: floatFromAny(root["single_quota"], fallback.SingleQuota),
		ResetCount:  intFromAny(root["reset_count"], fallback.ResetCount),
		AccountFee:  floatFromAny(root["account_fee"], fallback.AccountFee),
	}
}

func floatFromAny(value any, fallback float64) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	default:
		return fallback
	}
}

func intFromAny(value any, fallback int) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		if number == float64(int(number)) {
			return int(number)
		}
		return fallback
	case float32:
		if number == float32(int(number)) {
			return int(number)
		}
		return fallback
	default:
		return fallback
	}
}
