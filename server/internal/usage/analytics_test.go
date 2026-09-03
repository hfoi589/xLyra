package usage

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/store"
)

func TestNormalizeAnalyticsQueryDefaultsToRecentSevenDaysAndSuccess(t *testing.T) {
	t.Parallel()

	service := NewService(nil, config.LoadTimeZone("UTC"))
	now := time.Date(2026, 8, 5, 12, 30, 0, 0, time.UTC)

	got, err := service.normalizeAnalyticsQuery(AnalyticsQuery{View: AnalyticsViewBar}, now)
	if err != nil {
		t.Fatalf("normalizeAnalyticsQuery returned error: %v", err)
	}
	if got.View != AnalyticsViewBar {
		t.Fatalf("view = %q, want %q", got.View, AnalyticsViewBar)
	}
	if got.Success == nil || !*got.Success {
		t.Fatalf("success = %#v, want true", got.Success)
	}
	wantFrom := now.Add(-7 * 24 * time.Hour)
	if got.CreatedFrom == nil || !got.CreatedFrom.Equal(wantFrom) {
		t.Fatalf("created_from = %v, want %v", got.CreatedFrom, wantFrom)
	}
	if got.CreatedTo == nil || !got.CreatedTo.Equal(now) {
		t.Fatalf("created_to = %v, want %v", got.CreatedTo, now)
	}
}

func TestNormalizeAnalyticsQueryRejectsRangeLongerThan365Days(t *testing.T) {
	t.Parallel()

	service := NewService(nil, config.LoadTimeZone("UTC"))
	now := time.Date(2026, 8, 5, 12, 30, 0, 0, time.UTC)
	from := now.Add(-366 * 24 * time.Hour)

	_, err := service.normalizeAnalyticsQuery(AnalyticsQuery{
		View:        AnalyticsViewScatter,
		CreatedFrom: &from,
		CreatedTo:   &now,
	}, now)
	if err == nil {
		t.Fatal("expected analytics range validation error")
	}
}

func TestNormalizeAnalyticsQueryBarAlwaysUsesSuccessAndOtherViewsSupportAllStatuses(t *testing.T) {
	t.Parallel()

	service := NewService(nil, config.LoadTimeZone("UTC"))
	now := time.Date(2026, 8, 5, 12, 30, 0, 0, time.UTC)
	failed := false

	bar, err := service.normalizeAnalyticsQuery(AnalyticsQuery{View: AnalyticsViewBar, Success: &failed}, now)
	if err != nil {
		t.Fatalf("bar normalization returned error: %v", err)
	}
	if bar.Success == nil || !*bar.Success {
		t.Fatalf("bar success = %#v, want true", bar.Success)
	}

	all, err := service.normalizeAnalyticsQuery(AnalyticsQuery{View: AnalyticsViewScatter, AllStatuses: true}, now)
	if err != nil {
		t.Fatalf("all-status normalization returned error: %v", err)
	}
	if all.Success != nil {
		t.Fatalf("all-status success = %#v, want nil", all.Success)
	}
}

func TestAnalyticsBucketUnitUsesDailyPrecisionWhenSummaryRowsAreIncluded(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	if got := analyticsBucketUnit(from, to, true); got != analyticsBucketDay {
		t.Fatalf("bucket unit with summary rows = %q, want %q", got, analyticsBucketDay)
	}
	if got := analyticsBucketUnit(from, to, false); got != analyticsBucketHour {
		t.Fatalf("detail-only bucket unit = %q, want %q", got, analyticsBucketHour)
	}
}

func TestBuildBarAnalyticsCollapsesNonTopModelsAndKeysWithoutLosingCost(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	rows := []store.RequestAnalyticsAggregate{
		{BucketStart: day, ModelKey: "model-a", ModelLabel: "Model A", APIKeyKey: "key-a", APIKeyLabel: "Key A", Cost: 7, HasCost: true, RequestCount: 1},
		{BucketStart: day, ModelKey: "model-a", ModelLabel: "Model A", APIKeyKey: "key-b", APIKeyLabel: "Key B", Cost: 3, HasCost: true, RequestCount: 1},
		{BucketStart: day, ModelKey: "model-b", ModelLabel: "Model B", APIKeyKey: "key-a", APIKeyLabel: "Key A", Cost: 2, HasCost: true, RequestCount: 1},
	}

	result := buildBarAnalytics(rows, config.LoadTimeZone("UTC"), "day")
	if len(result.Points) != 1 {
		t.Fatalf("points = %#v, want one bucket", result.Points)
	}
	if len(result.Points[0].Groups) != 2 {
		t.Fatalf("groups = %#v, want two models", result.Points[0].Groups)
	}
	if got := result.TotalCost; got != 12 {
		t.Fatalf("total cost = %v, want 12", got)
	}
}

func TestBuildSankeyAnalyticsPreservesTokensWhenDimensionsCollapse(t *testing.T) {
	t.Parallel()

	rows := make([]store.RequestAnalyticsAggregate, 0, 13)
	for index := 0; index < 13; index++ {
		rows = append(rows, store.RequestAnalyticsAggregate{
			SiteKey:      "site-a",
			SiteLabel:    "Site A",
			ModelKey:     uuid.NewString(),
			ModelLabel:   "Model",
			APIKeyKey:    "key-a",
			APIKeyLabel:  "Key A",
			TotalTokens:  100,
			RequestCount: 1,
		})
	}

	result := buildSankeyAnalytics(rows)
	if result.TotalTokens != 1300 {
		t.Fatalf("total tokens = %d, want 1300", result.TotalTokens)
	}
	var linkTokens int64
	for _, link := range result.Links {
		linkTokens += link.Value
	}
	if linkTokens != 2600 {
		t.Fatalf("two-stage link tokens = %d, want 2600", linkTokens)
	}
}

func TestBuildScatterAnalyticsUsesFifteenMinuteCostBuckets(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 8, 5, 10, 15, 0, 0, time.UTC)
	result := buildScatterAnalytics([]store.RequestAnalyticsScatterBucket{
		{
			BucketStart:      start,
			RequestCount:     3,
			TotalTokens:      4096,
			Cost:             1.25,
			MissingCostCount: 1,
			Currency:         "USD",
		},
	})

	if len(result.Points) != 1 {
		t.Fatalf("points = %#v, want one fifteen-minute bucket", result.Points)
	}
	point := result.Points[0]
	if point.BucketStart != start.Format(time.RFC3339Nano) {
		t.Fatalf("bucket start = %q, want %q", point.BucketStart, start.Format(time.RFC3339Nano))
	}
	if point.BucketEnd != start.Add(15*time.Minute).Format(time.RFC3339Nano) {
		t.Fatalf("bucket end = %q, want fifteen minutes after start", point.BucketEnd)
	}
	if point.RequestCount != 3 || point.TotalTokens != 4096 || point.TotalCost != 1.25 || point.Currency != "USD" {
		t.Fatalf("point = %#v, want aggregated request count, tokens and cost", point)
	}
}
