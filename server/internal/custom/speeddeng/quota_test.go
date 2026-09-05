package speeddeng

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestFuncQuotaProviderDelegatesListAndRefresh(t *testing.T) {
	siteID := uuid.New()
	provider := FuncQuotaProvider{
		ListFunc: func(context.Context) ([]QuotaTarget, error) {
			return []QuotaTarget{{SiteID: siteID, SiteName: "Codex"}}, nil
		},
		RefreshFunc: func(_ context.Context, target QuotaTarget) (QuotaSnapshot, error) {
			if target.SiteID != siteID {
				t.Fatalf("target site = %s, want %s", target.SiteID, siteID)
			}
			return QuotaSnapshot{HasWeekly: true, WeeklyRemainingPercent: 100}, nil
		},
	}
	targets, err := provider.ListEligibleCodexOAuth(context.Background())
	if err != nil || len(targets) != 1 {
		t.Fatalf("ListEligibleCodexOAuth = %#v/%v", targets, err)
	}
	snapshot, err := provider.RefreshCodexQuota(context.Background(), targets[0])
	if err != nil || !snapshot.HasWeekly || snapshot.WeeklyRemainingPercent != 100 {
		t.Fatalf("RefreshCodexQuota = %#v/%v", snapshot, err)
	}
}

func TestQuotaSnapshotFromRawReadsWeeklyRemainingPercent(t *testing.T) {
	tests := []struct {
		name      string
		raw       map[string]any
		hasWeekly bool
		remaining float64
	}{
		{name: "integer", raw: map[string]any{"weekly": map[string]any{"remaining_percent": 100}}, hasWeekly: true, remaining: 100},
		{name: "float", raw: map[string]any{"weekly": map[string]any{"remaining_percent": 99.5}}, hasWeekly: true, remaining: 99.5},
		{name: "missing", raw: map[string]any{}, hasWeekly: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quotaSnapshotFromRaw(tc.raw)
			if got.HasWeekly != tc.hasWeekly || got.WeeklyRemainingPercent != tc.remaining {
				t.Fatalf("quotaSnapshotFromRaw = %#v, want weekly=%v remaining=%v", got, tc.hasWeekly, tc.remaining)
			}
		})
	}
}
