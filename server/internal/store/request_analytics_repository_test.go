package store

import (
	"testing"
	"time"
)

func TestRequestAnalyticsAggregateDefaultsMissingDimensions(t *testing.T) {
	t.Parallel()

	row := normalizeRequestAnalyticsAggregate(RequestAnalyticsAggregate{
		BucketStart: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
		Cost:        1.25,
	})
	if row.SiteKey != requestAnalyticsUnknownKey || row.SiteLabel != requestAnalyticsUnknownLabel {
		t.Fatalf("site fallback = %#v, want unknown", row)
	}
	if row.ModelKey != requestAnalyticsUnknownKey || row.APIKeyKey != requestAnalyticsUnknownKey {
		t.Fatalf("dimension fallbacks = %#v, want unknown", row)
	}
}

func TestRequestAnalyticsAggregateDoesNotTreatMissingCostAsZeroCost(t *testing.T) {
	t.Parallel()

	row := normalizeRequestAnalyticsAggregate(RequestAnalyticsAggregate{
		RequestCount:     1,
		Cost:             0,
		HasCost:          false,
		MissingCostCount: 1,
	})
	if row.HasCost {
		t.Fatal("missing cost must remain unavailable")
	}
}

func TestDetailAggregateGroupsProjectedCurrencyForTokenAggregates(t *testing.T) {
	t.Parallel()

	if got := detailAggregateGroupCurrency(false); got != ", currency" {
		t.Fatalf("group currency expression = %q, want projected CTE currency", got)
	}
}
