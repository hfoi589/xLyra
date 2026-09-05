package speeddeng

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"xlyra/server/internal/store"
)

type fakeAccountSource struct {
	connections []store.OAuthConnection
	sites       map[uuid.UUID]store.Site
}

func (f fakeAccountSource) ListConnections(context.Context) ([]store.OAuthConnection, error) {
	return append([]store.OAuthConnection(nil), f.connections...), nil
}

func (f fakeAccountSource) Site(_ context.Context, id uuid.UUID) (store.Site, error) {
	site, ok := f.sites[id]
	if !ok {
		return store.Site{}, errors.New("missing site")
	}
	return site, nil
}

type fakeUsageRefresher struct {
	quota map[string]any
}

func (f fakeUsageRefresher) RefreshCodexUsage(context.Context, uuid.UUID) (map[string]any, error) {
	return f.quota, nil
}

func TestDatabaseQuotaProviderFiltersConnectedEnabledCodexAccounts(t *testing.T) {
	eligibleSite := uuid.New()
	disabledSite := uuid.New()
	otherSite := uuid.New()
	provider := newDatabaseQuotaProviderWithDependencies(fakeAccountSource{
		connections: []store.OAuthConnection{
			{ID: uuid.New(), Provider: "codex", Status: "connected", SiteID: &eligibleSite},
			{ID: uuid.New(), Provider: "codex", Status: "connected", SiteID: &disabledSite},
			{ID: uuid.New(), Provider: "claude_code", Status: "connected", SiteID: &otherSite},
			{ID: uuid.New(), Provider: "codex", Status: "reconnect_required", SiteID: &otherSite},
		},
		sites: map[uuid.UUID]store.Site{
			eligibleSite: {ID: eligibleSite, Name: "Eligible", SiteType: "codex", Enabled: true},
			disabledSite: {ID: disabledSite, Name: "Disabled", SiteType: "codex", Enabled: false},
			otherSite:    {ID: otherSite, Name: "Other", SiteType: "claude_code", Enabled: true},
		},
	}, fakeUsageRefresher{})

	targets, err := provider.ListEligibleCodexOAuth(context.Background())
	if err != nil {
		t.Fatalf("ListEligibleCodexOAuth error = %v", err)
	}
	if len(targets) != 1 || targets[0].SiteID != eligibleSite || targets[0].SiteName != "Eligible" {
		t.Fatalf("targets = %#v, want only eligible Codex site", targets)
	}
}

func TestDatabaseQuotaProviderRefreshesWeeklySnapshot(t *testing.T) {
	connectionID := uuid.New()
	provider := newDatabaseQuotaProviderWithDependencies(fakeAccountSource{}, fakeUsageRefresher{quota: map[string]any{
		"weekly": map[string]any{"remaining_percent": 100.0},
	}})

	snapshot, err := provider.RefreshCodexQuota(context.Background(), QuotaTarget{ConnectionID: connectionID})
	if err != nil {
		t.Fatalf("RefreshCodexQuota error = %v", err)
	}
	if !snapshot.HasWeekly || snapshot.WeeklyRemainingPercent != 100 {
		t.Fatalf("snapshot = %#v, want weekly 100%%", snapshot)
	}
}
