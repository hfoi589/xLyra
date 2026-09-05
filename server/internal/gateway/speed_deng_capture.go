package gateway

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/auth"
	"xlyra/server/internal/custom/speeddeng"
	routeengine "xlyra/server/internal/router"
)

func speedDengCaptureInput(
	ctx context.Context,
	sessionID uuid.UUID,
	requestID string,
	apiKeyID uuid.UUID,
	candidate routeengine.Candidate,
	result gatewayAttemptResult,
	sourceRequestLogID uuid.UUID,
) (speeddeng.CaptureInput, bool) {
	if sessionID == uuid.Nil || sourceRequestLogID == uuid.Nil {
		return speeddeng.CaptureInput{}, false
	}
	internal, _ := bridgeRecordingFromContext(ctx)
	if !result.success || internal || result.diagnostic || !strings.EqualFold(strings.TrimSpace(candidate.Site.SiteType), "codex") {
		return speeddeng.CaptureInput{}, false
	}
	totalTokens := result.promptTokens + result.completionTokens + result.audioOutputTokens
	if totalTokens <= 0 && result.estimatedCost == nil {
		return speeddeng.CaptureInput{}, false
	}
	apiKeyName := ""
	if key, ok := auth.APIKeyFromContext(ctx); ok {
		apiKeyName = key.Name
		if apiKeyID == uuid.Nil {
			apiKeyID = key.ID
		}
	}
	var cost *float64
	if result.estimatedCost != nil {
		value := *result.estimatedCost
		cost = &value
	}
	currency := strings.TrimSpace(result.currency)
	if currency == "" {
		currency = "USD"
	}
	return speeddeng.CaptureInput{
		SessionID:          sessionID,
		SourceRequestLogID: sourceRequestLogID,
		SourceRequestID:    strings.TrimSpace(requestID),
		SiteID:             candidate.Site.ID,
		SiteName:           candidate.Site.Name,
		SiteType:           candidate.Site.SiteType,
		ModelKey:           candidate.Model.UpstreamName,
		APIKeyID:           apiKeyID,
		APIKeyName:         apiKeyName,
		PromptTokens:       result.promptTokens,
		CompletionTokens:   result.completionTokens + result.audioOutputTokens,
		CachedTokens:       int64(result.cachedPromptTokens),
		TotalTokens:        totalTokens,
		EstimatedCostUSD:   cost,
		Currency:           currency,
		Success:            result.success,
		Internal:           internal,
		Diagnostic:         result.diagnostic,
		CreatedAt:          time.Now(),
	}, true
}
