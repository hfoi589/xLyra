package gateway

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/auth"
	"xlyra/server/internal/config"
	"xlyra/server/internal/custom/speeddeng"
	routeengine "xlyra/server/internal/router"
	"xlyra/server/internal/store"
)

func TestRecordAttemptWritesSpeedDengEventAfterSourceCommit(t *testing.T) {
	dsn := os.Getenv("SPEED_DENG_TEST_DSN")
	if dsn == "" {
		t.Skip("set SPEED_DENG_TEST_DSN to run the gateway/Postgres speed-deng smoke test")
	}
	cfg := config.Config{PostgresDSN: dsn, DBConnectTimeout: 10 * time.Second}
	db, err := store.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := store.EnsureDatabaseInitialized(context.Background(), cfg); err != nil {
		t.Fatalf("ensure source schema: %v", err)
	}
	if err := speeddeng.EnsureSchema(db); err != nil {
		t.Fatalf("ensure speed-deng schema: %v", err)
	}

	siteID := uuid.New()
	apiKeyID := uuid.New()
	if err := db.DB().Create(&store.Site{ID: siteID, Name: "Codex Smoke", Slug: "codex-smoke-" + siteID.String()[:8], SiteType: "codex", BaseURL: "https://chatgpt.com/backend-api", Enabled: true, RoutingPriority: 1, Meta: store.JSON(`{}`)}).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	if err := db.DB().Create(&store.APIKey{ID: apiKeyID, Name: "Wilson", KeyPrefix: "smoke-" + apiKeyID.String()[:8], KeyHash: "hash-" + apiKeyID.String(), Status: "active", KeyKind: "generated", Scope: "gateway", ModelPolicy: "allow_all", SitePolicy: "allow_all", QuotaUnlimited: true, QuotaDailyUnlimited: true, QuotaWeeklyUnlimited: true}).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}

	provider := speeddeng.FuncQuotaProvider{ListFunc: func(context.Context) ([]speeddeng.QuotaTarget, error) {
		return []speeddeng.QuotaTarget{{SiteID: siteID}}, nil
	}, RefreshFunc: func(context.Context, speeddeng.QuotaTarget) (speeddeng.QuotaSnapshot, error) {
		return speeddeng.QuotaSnapshot{HasWeekly: true, WeeklyRemainingPercent: 50}, nil
	}}
	speedService := speeddeng.NewService(db, provider, config.LoadTimeZone("UTC"), nil)
	if _, err := speedService.Start(context.Background(), time.Now()); err != nil {
		t.Fatalf("start speed-deng: %v", err)
	}
	requestCtx := auth.WithAPIKey(context.Background(), store.APIKey{ID: apiKeyID, Name: "Wilson"})
	requestCtx, ok := speedService.BeginRequest(requestCtx)
	if !ok {
		t.Fatal("BeginRequest returned inactive")
	}

	handler := Handler{db: db, recorder: NewRecorder(db, nil, config.LoadTimeZone("UTC")), speedDengCapture: speedService, timeZone: config.LoadTimeZone("UTC")}
	cost := 0.5
	sourceID := handler.recordAttempt(requestCtx, "speed-smoke", apiKeyID, uuid.Nil, routeengine.Candidate{
		Site:  routeengine.CandidateSite{ID: siteID, Name: "Codex Smoke", SiteType: "codex"},
		Model: routeengine.CandidateModel{UpstreamName: "gpt-5"},
	}, gatewayAttemptResult{attempt: 1, statusCode: 200, success: true, promptTokens: 2, completionTokens: 3, estimatedCost: &cost, currency: "USD"}, nil)
	if sourceID == uuid.Nil {
		t.Fatal("recordAttempt did not create the source request log")
	}
	events, err := speeddeng.NewEventSource(db).ListEvents(context.Background(), siteID, time.Now().Add(-time.Minute), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("list speed-deng events: %v", err)
	}
	if len(events) != 1 || events[0].SourceRequestLogID != sourceID || events[0].APIKeyName != "Wilson" {
		t.Fatalf("events = %#v, want one event linked to source %s", events, sourceID)
	}

	failingHandler := handler
	failingHandler.speedDengCapture = failingSpeedDengCapture{}
	secondSourceID := failingHandler.recordAttempt(requestCtx, "speed-smoke-failing-custom", apiKeyID, uuid.Nil, routeengine.Candidate{
		Site:  routeengine.CandidateSite{ID: siteID, Name: "Codex Smoke", SiteType: "codex"},
		Model: routeengine.CandidateModel{UpstreamName: "gpt-5"},
	}, gatewayAttemptResult{attempt: 1, statusCode: 200, success: true, promptTokens: 1, completionTokens: 1, estimatedCost: &cost, currency: "USD"}, nil)
	if secondSourceID == uuid.Nil {
		t.Fatal("source request should remain recorded when custom event write fails")
	}
}
