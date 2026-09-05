package speeddeng

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type FuncQuotaProvider struct {
	ListFunc    func(context.Context) ([]QuotaTarget, error)
	RefreshFunc func(context.Context, QuotaTarget) (QuotaSnapshot, error)
}

func (p FuncQuotaProvider) ListEligibleCodexOAuth(ctx context.Context) ([]QuotaTarget, error) {
	if p.ListFunc == nil {
		return nil, fmt.Errorf("quota provider list function is not configured")
	}
	return p.ListFunc(ctx)
}

func (p FuncQuotaProvider) RefreshCodexQuota(ctx context.Context, target QuotaTarget) (QuotaSnapshot, error) {
	if p.RefreshFunc == nil {
		return QuotaSnapshot{}, fmt.Errorf("quota provider refresh function is not configured")
	}
	return p.RefreshFunc(ctx, target)
}

func quotaSnapshotFromRaw(raw map[string]any) QuotaSnapshot {
	if raw == nil {
		return QuotaSnapshot{}
	}
	weekly, ok := raw["weekly"].(map[string]any)
	if !ok {
		return QuotaSnapshot{}
	}
	value, ok := numberFromAny(weekly["remaining_percent"])
	if !ok {
		return QuotaSnapshot{}
	}
	return QuotaSnapshot{HasWeekly: true, WeeklyRemainingPercent: value}
}

func numberFromAny(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
