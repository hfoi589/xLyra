package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"xlyra/server/internal/auth"
	"xlyra/server/internal/custom/speeddeng"
	routeengine "xlyra/server/internal/router"
	"xlyra/server/internal/store"
)

func TestSpeedDengCaptureInputSnapshotsDownstreamAPIKeyAndUsage(t *testing.T) {
	apiKeyID := uuid.New()
	sessionID := uuid.New()
	cost := 0.75
	ctx := auth.WithAPIKey(context.Background(), store.APIKey{ID: apiKeyID, Name: "Wilson"})
	candidate := routeengine.Candidate{
		Site:  routeengine.CandidateSite{ID: uuid.New(), Name: "Codex OAuth", SiteType: "codex"},
		Model: routeengine.CandidateModel{UpstreamName: "gpt-5"},
	}
	result := gatewayAttemptResult{
		success:            true,
		promptTokens:       10,
		completionTokens:   5,
		cachedPromptTokens: 2,
		estimatedCost:      &cost,
		currency:           "USD",
	}

	input, ok := speedDengCaptureInput(ctx, sessionID, "req-1", apiKeyID, candidate, result, uuid.New())
	if !ok {
		t.Fatal("speedDengCaptureInput returned false for billable Codex attempt")
	}
	if input.SessionID != sessionID || input.APIKeyName != "Wilson" || input.ModelKey != "gpt-5" || input.TotalTokens != 15 || input.CachedTokens != 2 {
		t.Fatalf("capture input = %#v, want session/key/model/usage snapshot", input)
	}
	if input.EstimatedCostUSD == nil || *input.EstimatedCostUSD != cost {
		t.Fatalf("capture cost = %#v, want %v", input.EstimatedCostUSD, cost)
	}
}

func TestSpeedDengCaptureInputRejectsNonBillableAttempts(t *testing.T) {
	tests := []struct {
		name     string
		siteType string
		success  bool
		tokens   int
	}{
		{name: "non codex", siteType: "openai", success: true, tokens: 10},
		{name: "failure", siteType: "codex", success: false, tokens: 10},
		{name: "empty usage", siteType: "codex", success: true, tokens: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := auth.WithAPIKey(context.Background(), store.APIKey{Name: "Wilson"})
			candidate := routeengine.Candidate{Site: routeengine.CandidateSite{SiteType: tc.siteType}}
			result := gatewayAttemptResult{success: tc.success, promptTokens: tc.tokens}
			if _, ok := speedDengCaptureInput(ctx, uuid.New(), "req", uuid.New(), candidate, result, uuid.New()); ok {
				t.Fatal("speedDengCaptureInput returned true for non-billable attempt")
			}
		})
	}
}

var _ speeddeng.EventSource

type fakeSpeedDengCapture struct{}

func (fakeSpeedDengCapture) BeginRequest(ctx context.Context) (context.Context, bool) {
	return ctx, true
}
func (fakeSpeedDengCapture) CaptureSuccess(context.Context, speeddeng.CaptureInput) error { return nil }

func TestHandlerAcceptsOptionalSpeedDengCapture(t *testing.T) {
	capture := fakeSpeedDengCapture{}
	handler := (Handler{}).WithSpeedDengCapture(capture)
	if handler.speedDengCapture == nil {
		t.Fatal("speed-deng capture hook was not installed")
	}
}

type failingSpeedDengCapture struct{}

func (failingSpeedDengCapture) BeginRequest(ctx context.Context) (context.Context, bool) {
	return ctx, true
}
func (failingSpeedDengCapture) CaptureSuccess(context.Context, speeddeng.CaptureInput) error {
	return errors.New("custom event write failed")
}

func TestRecordSpeedDengEventDoesNotReturnCustomWriteFailure(t *testing.T) {
	handler := Handler{speedDengCapture: failingSpeedDengCapture{}}
	ctx := auth.WithAPIKey(context.Background(), store.APIKey{ID: uuid.New(), Name: "Wilson"})
	input, ok := speedDengCaptureInput(ctx, uuid.New(), "req", uuid.New(), routeengine.Candidate{Site: routeengine.CandidateSite{ID: uuid.New(), Name: "Codex", SiteType: "codex"}, Model: routeengine.CandidateModel{UpstreamName: "gpt-5"}}, gatewayAttemptResult{success: true, promptTokens: 1}, uuid.New())
	if !ok {
		t.Fatal("expected capture input")
	}
	if err := handler.recordSpeedDengEvent(ctx, input); err != nil {
		t.Fatalf("recordSpeedDengEvent returned custom error: %v", err)
	}
}
