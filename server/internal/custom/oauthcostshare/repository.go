package oauthcostshare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"xlyra/server/internal/config"
	"xlyra/server/internal/custom/speeddeng"
	"xlyra/server/internal/store"
)

type repository struct {
	db       *store.Store
	timeZone config.TimeZone
}

type speedRepository struct {
	source speeddeng.EventSource
}

func newSpeedRepository(db *store.Store) SpeedUsageSource {
	source := speeddeng.NewEventSource(db)
	if source == nil {
		return nil
	}
	return &speedRepository{source: source}
}

func (r *speedRepository) Usage(ctx context.Context, siteID uuid.UUID, from time.Time, to time.Time) ([]UsageRow, error) {
	if r == nil || r.source == nil {
		return nil, fmt.Errorf("speed-deng event store is not initialized")
	}
	events, err := r.source.ListEvents(ctx, siteID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list speed-deng events: %w", err)
	}
	rows := make([]UsageRow, 0, len(events))
	for _, event := range events {
		if currency := strings.TrimSpace(event.Currency); currency != "" && !strings.EqualFold(currency, "USD") {
			continue
		}
		cost := float64(0)
		if event.EstimatedCostUSD != nil {
			cost = *event.EstimatedCostUSD
		}
		apiKeyKey := ""
		if event.APIKeyID != uuid.Nil {
			apiKeyKey = event.APIKeyID.String()
		}
		rows = append(rows, UsageRow{
			ModelKey:     event.ModelKey,
			APIKeyKey:    apiKeyKey,
			APIKeyName:   event.APIKeyName,
			Cost:         cost,
			RequestCount: 1,
			SpeedDeng:    true,
		})
	}
	return rows, nil
}

func newRepository(db *store.Store, timeZone config.TimeZone) UsageSource {
	return &repository{db: db, timeZone: config.TimeZoneOrDefault(timeZone)}
}

func (r *repository) Site(ctx context.Context, siteID uuid.UUID) (OAuthSite, error) {
	if r == nil || r.db == nil || r.db.DB() == nil {
		return OAuthSite{}, fmt.Errorf("oauth cost share store is not initialized")
	}
	site, err := store.NewSiteRepository(r.db.DB()).GetByID(ctx, siteID)
	if err != nil {
		return OAuthSite{}, err
	}
	result := OAuthSite{ID: site.ID, Name: site.Name}
	connection, err := store.NewOAuthConnectionRepository(r.db.DB()).GetBySiteID(ctx, siteID)
	if err == nil {
		result.IsOAuth = true
		result.PlanType = oauthPlanType(connection.Metadata)
		if result.PlanType == "" {
			result.PlanType = oauthPlanType(site.Meta)
		}
		return result, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return OAuthSite{}, err
	}
	result.PlanType = oauthPlanType(site.Meta)
	return result, nil
}

func (r *repository) Usage(ctx context.Context, siteID uuid.UUID, from time.Time, to time.Time) ([]UsageRow, error) {
	if r == nil || r.db == nil || r.db.DB() == nil {
		return nil, fmt.Errorf("oauth cost share store is not initialized")
	}
	if !to.After(from) {
		return []UsageRow{}, nil
	}
	timeZone := config.TimeZoneOrDefault(r.timeZone)
	plan := splitUsageRange(from, to, time.Now().In(timeZone.Location), timeZone)
	summaryRepo := store.NewRequestUsageSummaryRepository(r.db.DB())
	rows := make([]store.RequestUsageDailySummary, 0)
	if plan.SummaryFrom.Before(plan.SummaryTo) {
		summaryRows, err := summaryRepo.List(ctx, store.RequestUsageSummaryQuery{
			TimeZone: timeZone.Name,
			From:     timePtr(plan.SummaryFrom),
			To:       timePtr(plan.SummaryTo),
			SiteID:   uuidPtr(siteID),
		})
		if err != nil {
			return nil, fmt.Errorf("list oauth cost share summaries: %w", err)
		}
		rows = append(rows, summaryRows...)
	}
	for _, window := range plan.DetailWindows {
		detailRows, err := summaryRepo.ListFromDetails(ctx, window.From, window.To, timeZone)
		if err != nil {
			return nil, fmt.Errorf("list oauth cost share details: %w", err)
		}
		rows = append(rows, detailRows...)
	}
	return usageRowsFromSummaries(rows, siteID), nil
}

type usageWindow struct {
	From time.Time
	To   time.Time
}

type usageRangePlan struct {
	SummaryFrom   time.Time
	SummaryTo     time.Time
	DetailWindows []usageWindow
}

func splitUsageRange(from time.Time, to time.Time, now time.Time, timeZone config.TimeZone) usageRangePlan {
	timeZone = config.TimeZoneOrDefault(timeZone)
	now = timeZone.In(now)
	from = timeZone.In(from)
	to = timeZone.In(to)
	if to.After(now) {
		to = now
	}
	plan := usageRangePlan{DetailWindows: []usageWindow{}}
	if !to.After(from) {
		return plan
	}

	todayStart := timeZone.StartOfDay(now)
	summaryFrom := timeZone.StartOfDay(from)
	if from.After(summaryFrom) {
		summaryFrom = summaryFrom.AddDate(0, 0, 1)
	}
	summaryTo := timeZone.StartOfDay(to)
	if summaryTo.After(todayStart) {
		summaryTo = todayStart
	}
	plan.SummaryFrom = summaryFrom
	plan.SummaryTo = summaryTo

	if summaryFrom.Before(summaryTo) {
		if from.Before(summaryFrom) {
			plan.DetailWindows = append(plan.DetailWindows, usageWindow{From: from, To: minTime(summaryFrom, to)})
		}
		if summaryTo.Before(to) {
			plan.DetailWindows = append(plan.DetailWindows, usageWindow{From: maxTime(summaryTo, from), To: to})
		}
		return plan
	}
	plan.DetailWindows = append(plan.DetailWindows, usageWindow{From: from, To: to})
	return plan
}

func usageRowsFromSummaries(rows []store.RequestUsageDailySummary, siteID uuid.UUID) []UsageRow {
	result := make([]UsageRow, 0, len(rows))
	for _, row := range rows {
		if !row.SiteID.Valid || row.SiteID.UUID != siteID || !row.Success || row.Internal {
			continue
		}
		currency := strings.TrimSpace(row.Currency)
		if currency == "" {
			currency = "USD"
		}
		if !strings.EqualFold(currency, "USD") {
			continue
		}
		apiKeyName := strings.TrimSpace(row.APIKeyName)
		if apiKeyName == "" {
			apiKeyName = strings.TrimSpace(row.APIKeyKey)
		}
		result = append(result, UsageRow{
			ModelKey:     row.CanonicalModelKey,
			APIKeyKey:    row.APIKeyKey,
			APIKeyName:   apiKeyName,
			Cost:         row.EstimatedCost,
			RequestCount: row.RequestCount,
		})
	}
	return result
}

func oauthPlanType(raw store.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return ""
	}
	if value, ok := root["plan_type"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	if quota, ok := root["quota"].(map[string]any); ok {
		if value, ok := quota["plan_type"].(string); ok {
			return value
		}
	}
	if value, ok := root["oauth_plan_type"].(string); ok {
		return value
	}
	return ""
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	return &value
}

func minTime(left time.Time, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

func maxTime(left time.Time, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}
