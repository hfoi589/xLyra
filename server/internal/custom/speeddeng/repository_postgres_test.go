package speeddeng

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/store"
)

func TestEnsureSchemaAndRepositoryRoundTrip(t *testing.T) {
	dsn := os.Getenv("SPEED_DENG_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://xlyra:postgres_password@localhost:5432/xlyra?sslmode=disable"
	}
	db, err := store.Open(context.Background(), config.Config{PostgresDSN: dsn, DBConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Skipf("postgres test database unavailable: %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema error = %v", err)
	}

	repo := newRepository(db)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	session, err := repo.Start(context.Background(), now)
	if err != nil {
		t.Fatalf("Start error = %v", err)
	}
	cost := 0.5
	event := Event{ID: uuid.New(), SessionID: session.ID, SourceRequestLogID: uuid.New(), SiteID: uuid.New(), APIKeyName: "Wilson", TotalTokens: 10, EstimatedCostUSD: &cost, Currency: "USD", CreatedAt: now}
	if err := repo.RecordEvent(context.Background(), event); err != nil {
		t.Fatalf("RecordEvent error = %v", err)
	}
	if err := repo.RecordEvent(context.Background(), event); err != nil {
		t.Fatalf("duplicate RecordEvent should be idempotent, got %v", err)
	}
	items, err := repo.ListEvents(context.Background(), event.SiteID, now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ListEvents error = %v", err)
	}
	if len(items) != 1 || items[0].SourceRequestLogID != event.SourceRequestLogID {
		t.Fatalf("events = %#v, want one round-tripped event", items)
	}
}

func TestDeleteBeforeKeepsEventsBelongingToActiveSession(t *testing.T) {
	dsn := os.Getenv("SPEED_DENG_TEST_DSN")
	if dsn == "" {
		t.Skip("set SPEED_DENG_TEST_DSN to run the gateway/Postgres speed-deng smoke test")
	}
	db, err := store.Open(context.Background(), config.Config{PostgresDSN: dsn, DBConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Skipf("postgres test database unavailable: %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema error = %v", err)
	}
	repo := newRepository(db)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	session, err := repo.Start(context.Background(), now)
	if err != nil {
		t.Fatalf("Start error = %v", err)
	}
	event := Event{ID: uuid.New(), SessionID: session.ID, SourceRequestLogID: uuid.New(), SiteID: uuid.New(), TotalTokens: 1, CreatedAt: now.Add(-48 * time.Hour)}
	if err := repo.RecordEvent(context.Background(), event); err != nil {
		t.Fatalf("RecordEvent error = %v", err)
	}
	if _, err := repo.DeleteBefore(context.Background(), now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("DeleteBefore error = %v", err)
	}
	items, err := repo.ListEvents(context.Background(), event.SiteID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ListEvents error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("active-session events = %#v, want preserved event", items)
	}
}
