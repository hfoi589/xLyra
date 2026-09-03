package oauthcostshare

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/store"
)

func TestSplitUsageRangeSeparatesFullHistoricalDaysFromDetailWindows(t *testing.T) {
	t.Parallel()

	timeZone := config.LoadTimeZone("UTC")
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	from := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)

	plan := splitUsageRange(from, to, now, timeZone)
	if !plan.SummaryFrom.Equal(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)) || !plan.SummaryTo.Equal(time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("summary range = %#v, want Aug 2 through Aug 5", plan)
	}
	if len(plan.DetailWindows) != 2 {
		t.Fatalf("detail windows = %#v, want two boundary windows", plan.DetailWindows)
	}
	if !plan.DetailWindows[0].From.Equal(from) || !plan.DetailWindows[0].To.Equal(plan.SummaryFrom) {
		t.Fatalf("first detail window = %#v, want [%s,%s)", plan.DetailWindows[0], from, plan.SummaryFrom)
	}
	if !plan.DetailWindows[1].From.Equal(plan.SummaryTo) || !plan.DetailWindows[1].To.Equal(to) {
		t.Fatalf("last detail window = %#v, want [%s,%s)", plan.DetailWindows[1], plan.SummaryTo, to)
	}
}

func TestUsageRowsFromSummariesFiltersToSuccessfulUSDAndFallsBackToKeyKey(t *testing.T) {
	t.Parallel()

	siteID := uuid.New()
	rows := usageRowsFromSummaries([]store.RequestUsageDailySummary{
		{SiteID: uuid.NullUUID{UUID: siteID, Valid: true}, Success: true, Currency: "USD", APIKeyKey: "key-a", APIKeyName: "Wilson", EstimatedCost: 2, RequestCount: 1},
		{SiteID: uuid.NullUUID{UUID: siteID, Valid: true}, Success: false, Currency: "USD", APIKeyKey: "key-b", EstimatedCost: 3, RequestCount: 1},
		{SiteID: uuid.NullUUID{UUID: siteID, Valid: true}, Success: true, Currency: "CNY", APIKeyKey: "key-c", EstimatedCost: 4, RequestCount: 1},
		{SiteID: uuid.NullUUID{UUID: siteID, Valid: true}, Success: true, Internal: true, Currency: "USD", APIKeyKey: "key-d", EstimatedCost: 5, RequestCount: 1},
		{SiteID: uuid.NullUUID{UUID: siteID, Valid: true}, Success: true, Currency: "", APIKeyKey: "key-e", EstimatedCost: 6, RequestCount: 1},
	}, siteID)

	if len(rows) != 2 {
		t.Fatalf("filtered usage rows = %#v, want two USD rows", rows)
	}
	if rows[1].APIKeyName != "key-e" || rows[1].Cost != 6 {
		t.Fatalf("blank key name fallback row = %#v, want APIKeyKey", rows[1])
	}
}
