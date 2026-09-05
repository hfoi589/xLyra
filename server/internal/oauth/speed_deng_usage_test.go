package oauth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"xlyra/server/internal/store"
)

func TestRefreshCodexUsageRejectsUnsupportedProvider(t *testing.T) {
	connectionID := uuid.New()
	service := oauthServiceWithQueryUpdate(t, func(tx *gorm.DB) {
		connection, ok := tx.Statement.Dest.(*store.OAuthConnection)
		if !ok {
			tx.AddError(errors.New("unexpected speed-deng usage query destination"))
			return
		}
		*connection = store.OAuthConnection{ID: connectionID, Provider: "claude_code"}
		tx.Statement.RowsAffected = 1
	}, func(tx *gorm.DB) {
		tx.AddError(errors.New("unsupported provider must not save"))
	})

	_, err := service.RefreshCodexUsage(context.Background(), connectionID)
	if err == nil || !strings.Contains(err.Error(), "does not support codex usage refresh") {
		t.Fatalf("RefreshCodexUsage error = %v, want provider guard", err)
	}
}

func TestRefreshCodexUsageAcceptsCaseAndWhitespaceAndPersistsSummary(t *testing.T) {
	service := NewService(nil, "master-key")
	encryptedAccess, _, err := service.credentials.Encrypt("access-token")
	if err != nil {
		t.Fatalf("encrypt access token: %v", err)
	}
	connectionID := uuid.New()
	connection := store.OAuthConnection{
		ID:                   connectionID,
		Provider:             " CoDeX ",
		Status:               "connected",
		AccountID:            "acct-1",
		Email:                "user@example.com",
		EncryptedAccessToken: encryptedAccess,
		Metadata:             store.JSON(`{"plan_type":"plus"}`),
		ExpiresAt:            sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true},
	}
	var saved store.OAuthConnection
	db := oauthGormWithQueryUpdate(t, func(tx *gorm.DB) {
		item, ok := tx.Statement.Dest.(*store.OAuthConnection)
		if !ok {
			tx.AddError(errors.New("unexpected usage connection query destination"))
			return
		}
		*item = connection
		tx.Statement.RowsAffected = 1
	}, func(tx *gorm.DB) {
		item, ok := tx.Statement.Dest.(*store.OAuthConnection)
		if !ok {
			tx.AddError(errors.New("unexpected usage connection save destination"))
			return
		}
		saved = *item
		tx.Statement.RowsAffected = 1
	})
	service.db = oauthStoreWithGorm(t, db)
	service.httpClient = &http.Client{Transport: oauthRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/backend-api/wham/usage":
			return oauthHTTPResponse(http.StatusOK, `{"rate_limit":{"secondary_window":{"limit_window_seconds":604800,"remaining_percent":100}}}`), nil
		case "/backend-api/codex/models":
			return oauthHTTPResponse(http.StatusOK, `{"data":[{"id":"gpt-5"}]}`), nil
		default:
			return oauthHTTPResponse(http.StatusNotFound, `{"error":"not found"}`), nil
		}
	})}

	quota, err := service.RefreshCodexUsage(context.Background(), connectionID)
	if err != nil {
		t.Fatalf("RefreshCodexUsage error = %v", err)
	}
	weekly, ok := quota["weekly"].(map[string]any)
	if !ok || weekly["remaining_percent"] != 100 {
		t.Fatalf("quota = %#v, want weekly remaining 100", quota)
	}
	if !strings.Contains(string(saved.Metadata), `"quota"`) || !saved.LastSyncAt.Valid {
		t.Fatalf("saved connection metadata = %s, last_sync=%v; want quota and sync time", saved.Metadata, saved.LastSyncAt.Valid)
	}
}
