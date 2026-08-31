package store

import (
	"context"
	"testing"
	"time"
)

func TestRequestAnalyticsAggregatesPostgresSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := devPostgresSmokeConfig()
	if err != nil {
		t.Skipf("dev PostgreSQL smoke disabled: %v", err)
	}
	db, err := Open(ctx, cfg)
	if err != nil {
		t.Skipf("dev PostgreSQL unavailable: %s", redactDatabaseOpenError(err, cfg))
	}
	defer db.Close()

	success := true
	query := RequestAnalyticsQuery{
		CreatedFrom:   time.Now().Add(-7 * 24 * time.Hour),
		CreatedTo:     time.Now(),
		Success:       &success,
		Currency:      "USD",
		ApplyCurrency: true,
	}
	repo := NewRequestAnalyticsRepository(db.DB())
	if _, err := repo.ListCurrencies(ctx, query); err != nil {
		t.Fatalf("ListCurrencies: %v", err)
	}
	if _, err := repo.ListSummaryAggregates(ctx, query, query.CreatedFrom, query.CreatedTo); err != nil {
		t.Fatalf("ListSummaryAggregates: %v", err)
	}
	if _, err := repo.ListDetailAggregates(ctx, query, query.CreatedFrom, query.CreatedTo, "day", "UTC"); err != nil {
		t.Fatalf("ListDetailAggregates: %v", err)
	}
	tokenQuery := query
	tokenQuery.ApplyCurrency = false
	if _, err := repo.ListDetailAggregates(ctx, tokenQuery, query.CreatedFrom, query.CreatedTo, "day", "UTC"); err != nil {
		t.Fatalf("ListDetailAggregates token mode: %v", err)
	}
	if _, _, _, err := repo.CountScatterBuckets(ctx, query, "UTC"); err != nil {
		t.Fatalf("CountScatterBuckets: %v", err)
	}
	if _, err := repo.ListScatterBuckets(ctx, query, "UTC"); err != nil {
		t.Fatalf("ListScatterBuckets: %v", err)
	}
}
