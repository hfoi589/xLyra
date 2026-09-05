package oauthcostshare

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/custom/speeddeng"
	"xlyra/server/internal/store"
)

type fakeSpeedEventSource struct {
	events []speeddeng.Event
	err    error
}

func (f fakeSpeedEventSource) ListEvents(context.Context, uuid.UUID, time.Time, time.Time) ([]speeddeng.Event, error) {
	return f.events, f.err
}

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

func TestSpeedRepositoryMapsEventsToIndependentUsageRows(t *testing.T) {
	cost := 0.75
	source := fakeSpeedEventSource{events: []speeddeng.Event{{
		SourceRequestLogID: uuid.New(),
		APIKeyID:           uuid.New(),
		APIKeyName:         "Wilson",
		ModelKey:           "gpt-5",
		TotalTokens:        20,
		EstimatedCostUSD:   &cost,
	}}}
	rows, err := (&speedRepository{source: source}).Usage(context.Background(), uuid.New(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("speed repository Usage error = %v", err)
	}
	if len(rows) != 1 || !rows[0].SpeedDeng || rows[0].APIKeyName != "Wilson" || rows[0].Cost != cost || rows[0].RequestCount != 1 {
		t.Fatalf("rows = %#v, want one independent speed row", rows)
	}
}

func TestSpeedRepositoryPropagatesEventSourceFailure(t *testing.T) {
	wantErr := errors.New("speed table unavailable")
	_, err := (&speedRepository{source: fakeSpeedEventSource{err: wantErr}}).Usage(context.Background(), uuid.New(), time.Time{}, time.Time{})
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("Usage error = %v, want wrapped source error", err)
	}
}

func TestSpeedRepositoryIgnoresNonUSDEvents(t *testing.T) {
	cost := 1.0
	rows, err := (&speedRepository{source: fakeSpeedEventSource{events: []speeddeng.Event{{
		SourceRequestLogID: uuid.New(),
		Currency:           "EUR",
		TotalTokens:        5,
		EstimatedCostUSD:   &cost,
	}}}}).Usage(context.Background(), uuid.New(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Usage error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %#v, want non-USD event ignored", rows)
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
