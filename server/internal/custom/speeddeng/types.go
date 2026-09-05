package speeddeng

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	StatusInactive = "inactive"
	StatusActive   = "active"
	StatusStopped  = "stopped"

	StopReasonManual                = "manual"
	StopReasonQuotaRecovered        = "weekly_quota_recovered"
	StopReasonStartupQuotaRecovered = "startup_quota_recovered"
)

var (
	ErrNoActiveSession    = errSentinel("no active speed-deng session")
	ErrNoEligibleAccounts = errSentinel("no eligible codex oauth accounts")
	ErrServiceUnavailable = errSentinel("speed-deng service is unavailable")
)

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

type Session struct {
	ID               uuid.UUID  `json:"id"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	StoppedAt        *time.Time `json:"stopped_at,omitempty"`
	StopReason       string     `json:"stop_reason,omitempty"`
	LastQuotaCheckAt *time.Time `json:"last_quota_check_at,omitempty"`
	QuotaCheck       QuotaCheck `json:"quota_check"`
}

type QuotaCheck struct {
	EligibleCount int    `json:"eligible_count"`
	CheckedCount  int    `json:"checked_count"`
	SkippedCount  int    `json:"skipped_count"`
	Recovered     bool   `json:"recovered"`
	Warning       string `json:"warning,omitempty"`
}

type QuotaTarget struct {
	SiteID       uuid.UUID
	ConnectionID uuid.UUID
	SiteName     string
}

type QuotaSnapshot struct {
	HasWeekly              bool
	WeeklyRemainingPercent float64
}

type CaptureInput struct {
	SessionID          uuid.UUID
	SourceRequestLogID uuid.UUID
	SourceRequestID    string
	SiteID             uuid.UUID
	SiteName           string
	SiteType           string
	ModelKey           string
	APIKeyID           uuid.UUID
	APIKeyName         string
	PromptTokens       int
	CompletionTokens   int
	CachedTokens       int64
	TotalTokens        int
	EstimatedCostUSD   *float64
	Currency           string
	Success            bool
	Internal           bool
	Diagnostic         bool
	CreatedAt          time.Time
}

type Event struct {
	ID                 uuid.UUID
	SessionID          uuid.UUID
	SourceRequestLogID uuid.UUID
	SourceRequestID    string
	SiteID             uuid.UUID
	SiteName           string
	ModelKey           string
	APIKeyID           uuid.UUID
	APIKeyName         string
	PromptTokens       int
	CompletionTokens   int
	CachedTokens       int64
	TotalTokens        int
	EstimatedCostUSD   *float64
	Currency           string
	CreatedAt          time.Time
}

type Status struct {
	Active           bool       `json:"active"`
	State            string     `json:"state"`
	SessionID        *uuid.UUID `json:"session_id,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	StoppedAt        *time.Time `json:"stopped_at,omitempty"`
	StopReason       string     `json:"stop_reason,omitempty"`
	EventCount       int64      `json:"event_count"`
	LastQuotaCheckAt *time.Time `json:"last_quota_check_at,omitempty"`
	QuotaCheck       QuotaCheck `json:"quota_check"`
}

type Repository interface {
	Active(ctx context.Context) (Session, error)
	Start(ctx context.Context, now time.Time) (Session, error)
	Stop(ctx context.Context, sessionID uuid.UUID, reason string, now time.Time) (Session, error)
	UpdateQuotaCheck(ctx context.Context, sessionID uuid.UUID, check QuotaCheck, now time.Time) (Session, error)
	CountEvents(ctx context.Context, sessionID uuid.UUID) (int64, error)
	RecordEvent(ctx context.Context, event Event) error
	ListEvents(ctx context.Context, siteID uuid.UUID, from time.Time, to time.Time) ([]Event, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type EventSource interface {
	ListEvents(ctx context.Context, siteID uuid.UUID, from time.Time, to time.Time) ([]Event, error)
}

type QuotaProvider interface {
	ListEligibleCodexOAuth(ctx context.Context) ([]QuotaTarget, error)
	RefreshCodexQuota(ctx context.Context, target QuotaTarget) (QuotaSnapshot, error)
}

// OAuthSiteChecker lets the capture path distinguish a Codex OAuth site from
// a malformed/legacy site that merely carries the "codex" site type. It is
// optional so lightweight test providers and embedders can keep the existing
// quota-provider contract.
type OAuthSiteChecker interface {
	IsCodexOAuthSite(ctx context.Context, siteID uuid.UUID) (bool, error)
}

type sessionContextKey struct{}

func withSessionID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, id)
}

func sessionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	id, ok := ctx.Value(sessionContextKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
