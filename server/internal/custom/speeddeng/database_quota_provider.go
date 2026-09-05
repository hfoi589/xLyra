package speeddeng

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	oauthsvc "xlyra/server/internal/oauth"
	"xlyra/server/internal/store"
)

type quotaAccountSource interface {
	ListConnections(ctx context.Context) ([]store.OAuthConnection, error)
	Site(ctx context.Context, siteID uuid.UUID) (store.Site, error)
}

type quotaUsageRefresher interface {
	RefreshCodexUsage(ctx context.Context, connectionID uuid.UUID) (map[string]any, error)
}

type databaseQuotaProvider struct {
	accounts     quotaAccountSource
	refresher    quotaUsageRefresher
	oauthSitesMu sync.RWMutex
	oauthSitesAt time.Time
	oauthSiteIDs map[uuid.UUID]struct{}
}

const oauthSiteCacheTTL = 15 * time.Second

func NewDatabaseQuotaProvider(db *store.Store, oauth *oauthsvc.Service) QuotaProvider {
	if db == nil || db.DB() == nil || oauth == nil {
		return nil
	}
	return newDatabaseQuotaProviderWithDependencies(databaseQuotaAccountSource{db: db}, oauth)
}

func newDatabaseQuotaProviderWithDependencies(accounts quotaAccountSource, refresher quotaUsageRefresher) QuotaProvider {
	return &databaseQuotaProvider{accounts: accounts, refresher: refresher}
}

func (p *databaseQuotaProvider) ListEligibleCodexOAuth(ctx context.Context) ([]QuotaTarget, error) {
	if p == nil || p.accounts == nil {
		return nil, fmt.Errorf("quota account source is not configured")
	}
	connections, err := p.accounts.ListConnections(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]QuotaTarget, 0, len(connections))
	oauthSiteIDs := make(map[uuid.UUID]struct{})
	for _, connection := range connections {
		if !strings.EqualFold(strings.TrimSpace(connection.Provider), "codex") || connection.SiteID == nil || *connection.SiteID == uuid.Nil {
			continue
		}
		site, siteErr := p.accounts.Site(ctx, *connection.SiteID)
		if siteErr != nil || !strings.EqualFold(strings.TrimSpace(site.SiteType), "codex") {
			continue
		}
		oauthSiteIDs[site.ID] = struct{}{}
		if !strings.EqualFold(strings.TrimSpace(connection.Status), "connected") || !site.Enabled {
			continue
		}
		targets = append(targets, QuotaTarget{SiteID: site.ID, ConnectionID: connection.ID, SiteName: site.Name})
	}
	p.cacheOAuthSites(oauthSiteIDs, time.Now())
	sort.SliceStable(targets, func(i, j int) bool {
		return strings.ToLower(targets[i].SiteName) < strings.ToLower(targets[j].SiteName)
	})
	return targets, nil
}

func (p *databaseQuotaProvider) IsCodexOAuthSite(ctx context.Context, siteID uuid.UUID) (bool, error) {
	// Keep the identity snapshot broader than the start/monitor eligibility
	// filter: a request that entered while the connection was valid must still
	// be attributed if the site is disabled or disconnected before its response
	// is recorded.
	if p == nil || p.accounts == nil {
		return false, fmt.Errorf("quota account source is not configured")
	}
	if siteID == uuid.Nil {
		return false, nil
	}
	p.oauthSitesMu.RLock()
	if !p.oauthSitesAt.IsZero() && time.Since(p.oauthSitesAt) < oauthSiteCacheTTL {
		_, ok := p.oauthSiteIDs[siteID]
		p.oauthSitesMu.RUnlock()
		return ok, nil
	}
	p.oauthSitesMu.RUnlock()

	connections, err := p.accounts.ListConnections(ctx)
	if err != nil {
		return false, err
	}
	oauthSiteIDs := make(map[uuid.UUID]struct{})
	for _, connection := range connections {
		if !strings.EqualFold(strings.TrimSpace(connection.Provider), "codex") || connection.SiteID == nil || *connection.SiteID == uuid.Nil {
			continue
		}
		site, siteErr := p.accounts.Site(ctx, *connection.SiteID)
		if siteErr != nil || !strings.EqualFold(strings.TrimSpace(site.SiteType), "codex") {
			continue
		}
		oauthSiteIDs[site.ID] = struct{}{}
	}
	p.cacheOAuthSites(oauthSiteIDs, time.Now())
	_, ok := oauthSiteIDs[siteID]
	return ok, nil
}

func (p *databaseQuotaProvider) cacheOAuthSites(siteIDs map[uuid.UUID]struct{}, at time.Time) {
	if p == nil {
		return
	}
	p.oauthSitesMu.Lock()
	p.oauthSiteIDs = siteIDs
	p.oauthSitesAt = at
	p.oauthSitesMu.Unlock()
}

func (p *databaseQuotaProvider) RefreshCodexQuota(ctx context.Context, target QuotaTarget) (QuotaSnapshot, error) {
	if p == nil || p.refresher == nil {
		return QuotaSnapshot{}, fmt.Errorf("quota usage refresher is not configured")
	}
	if target.ConnectionID == uuid.Nil {
		return QuotaSnapshot{}, fmt.Errorf("quota target connection id is required")
	}
	raw, err := p.refresher.RefreshCodexUsage(ctx, target.ConnectionID)
	if err != nil {
		return QuotaSnapshot{}, err
	}
	return quotaSnapshotFromRaw(raw), nil
}

type databaseQuotaAccountSource struct {
	db *store.Store
}

func (s databaseQuotaAccountSource) ListConnections(ctx context.Context) ([]store.OAuthConnection, error) {
	if s.db == nil || s.db.DB() == nil {
		return nil, fmt.Errorf("quota database is not initialized")
	}
	return store.NewOAuthConnectionRepository(s.db.DB()).List(ctx)
}

func (s databaseQuotaAccountSource) Site(ctx context.Context, siteID uuid.UUID) (store.Site, error) {
	if s.db == nil || s.db.DB() == nil {
		return store.Site{}, fmt.Errorf("quota database is not initialized")
	}
	return store.NewSiteRepository(s.db.DB()).GetByID(ctx, siteID)
}
