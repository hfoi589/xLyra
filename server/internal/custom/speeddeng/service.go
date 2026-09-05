package speeddeng

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/store"
)

const (
	defaultCaptureTimeout = 2 * time.Second
	stateCacheTTL         = time.Second
)

type Service struct {
	repo           Repository
	provider       QuotaProvider
	oauthSiteCheck OAuthSiteChecker
	timeZone       config.TimeZone
	logger         *slog.Logger
	checkMu        sync.Mutex
	stateMu        sync.RWMutex
	stateAt        time.Time
	state          Session
	stateErr       error
	stateOK        bool
}

func NewService(db *store.Store, provider QuotaProvider, timeZone config.TimeZone, logger *slog.Logger) *Service {
	return NewServiceWithDependencies(newRepository(db), provider, timeZone, logger)
}

func NewServiceWithDependencies(repo Repository, provider QuotaProvider, timeZone config.TimeZone, logger *slog.Logger) *Service {
	var oauthSiteCheck OAuthSiteChecker
	if checker, ok := provider.(OAuthSiteChecker); ok {
		oauthSiteCheck = checker
	}
	return &Service{
		repo:           repo,
		provider:       provider,
		oauthSiteCheck: oauthSiteCheck,
		timeZone:       config.TimeZoneOrDefault(timeZone),
		logger:         logger,
	}
}

func (s *Service) Start(ctx context.Context, now time.Time) (Session, error) {
	if s == nil || s.repo == nil || s.provider == nil {
		return Session{}, ErrServiceUnavailable
	}
	if now.IsZero() {
		now = time.Now()
	}
	if active, err := s.readSession(ctx); err == nil && active.Status == StatusActive {
		return active, nil
	} else if err != nil && !errors.Is(err, ErrNoActiveSession) {
		return Session{}, fmt.Errorf("read speed-deng state: %w", err)
	}
	targets, err := s.provider.ListEligibleCodexOAuth(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("list eligible codex oauth accounts: %w", err)
	}
	if len(targets) == 0 {
		return Session{}, ErrNoEligibleAccounts
	}
	session, err := s.repo.Start(ctx, now)
	if err != nil {
		// A second process may have won the active-session race. Re-read the
		// authoritative row and make start idempotent in that case.
		s.invalidateSessionCache()
		if active, readErr := s.readSession(ctx); readErr == nil && active.Status == StatusActive {
			return active, nil
		}
		return Session{}, fmt.Errorf("start speed-deng session: %w", err)
	}
	if session.Status == "" {
		session.Status = StatusActive
	}
	s.cacheSession(session)
	return session, nil
}

func (s *Service) Stop(ctx context.Context, now time.Time, reason string) (Session, error) {
	if s == nil || s.repo == nil {
		return Session{}, ErrServiceUnavailable
	}
	if now.IsZero() {
		now = time.Now()
	}
	if strings.TrimSpace(reason) == "" {
		reason = StopReasonManual
	}
	active, err := s.readSession(ctx)
	if err != nil {
		if errors.Is(err, ErrNoActiveSession) {
			s.cacheNoSession()
			return Session{Status: StatusInactive}, nil
		}
		return Session{}, fmt.Errorf("read speed-deng state: %w", err)
	}
	if active.Status != StatusActive {
		return active, nil
	}
	stopped, err := s.repo.Stop(ctx, active.ID, reason, now)
	if err != nil {
		return Session{}, fmt.Errorf("stop speed-deng session: %w", err)
	}
	s.cacheSession(stopped)
	return stopped, nil
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	if s == nil || s.repo == nil {
		return Status{}, ErrServiceUnavailable
	}
	active, err := s.readSession(ctx)
	if err != nil {
		if errors.Is(err, ErrNoActiveSession) {
			return Status{State: StatusInactive}, nil
		}
		return Status{}, fmt.Errorf("read speed-deng state: %w", err)
	}
	return s.statusForSession(ctx, active), nil
}

func (s *Service) BeginRequest(ctx context.Context) (context.Context, bool) {
	if s == nil || s.repo == nil {
		return ctx, false
	}
	active, err := s.readSession(ctx)
	if err != nil || active.Status != StatusActive {
		return ctx, false
	}
	return withSessionID(ctx, active.ID), true
}

func SessionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	return sessionIDFromContext(ctx)
}

func (s *Service) CaptureSuccess(ctx context.Context, input CaptureInput) error {
	if s == nil || s.repo == nil {
		return ErrServiceUnavailable
	}
	if input.SessionID == uuid.Nil {
		if id, ok := sessionIDFromContext(ctx); ok {
			input.SessionID = id
		}
	}
	if input.SessionID == uuid.Nil || input.SourceRequestLogID == uuid.Nil {
		return nil
	}
	if !input.Success || input.Internal || input.Diagnostic || !strings.EqualFold(strings.TrimSpace(input.SiteType), "codex") {
		return nil
	}
	if input.TotalTokens <= 0 && input.EstimatedCostUSD == nil {
		return nil
	}
	if s.oauthSiteCheck != nil {
		checkCtx, cancel := context.WithTimeout(context.Background(), defaultCaptureTimeout)
		defer cancel()
		isOAuth, err := s.oauthSiteCheck.IsCodexOAuthSite(checkCtx, input.SiteID)
		if err != nil {
			return fmt.Errorf("check codex oauth site: %w", err)
		}
		if !isOAuth {
			return nil
		}
	}
	if currency := strings.TrimSpace(input.Currency); currency != "" && !strings.EqualFold(currency, "USD") {
		return nil
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now()
	}
	input.APIKeyName = strings.TrimSpace(input.APIKeyName)
	if input.Currency == "" {
		input.Currency = "USD"
	}
	event := Event{
		ID:                 uuid.New(),
		SessionID:          input.SessionID,
		SourceRequestLogID: input.SourceRequestLogID,
		SourceRequestID:    strings.TrimSpace(input.SourceRequestID),
		SiteID:             input.SiteID,
		SiteName:           strings.TrimSpace(input.SiteName),
		ModelKey:           strings.TrimSpace(input.ModelKey),
		APIKeyID:           input.APIKeyID,
		APIKeyName:         input.APIKeyName,
		PromptTokens:       input.PromptTokens,
		CompletionTokens:   input.CompletionTokens,
		CachedTokens:       input.CachedTokens,
		TotalTokens:        input.TotalTokens,
		EstimatedCostUSD:   input.EstimatedCostUSD,
		Currency:           input.Currency,
		CreatedAt:          input.CreatedAt,
	}
	captureCtx, cancel := context.WithTimeout(context.Background(), defaultCaptureTimeout)
	defer cancel()
	if err := s.repo.RecordEvent(captureCtx, event); err != nil {
		return fmt.Errorf("record speed-deng event: %w", err)
	}
	return nil
}

func (s *Service) CheckAndAutoStop(ctx context.Context, now time.Time, startup bool) (Status, error) {
	if s == nil || s.repo == nil || s.provider == nil {
		return Status{}, ErrServiceUnavailable
	}
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	if now.IsZero() {
		now = time.Now()
	}
	active, err := s.readSession(ctx)
	if err != nil {
		if errors.Is(err, ErrNoActiveSession) {
			return Status{State: StatusInactive}, nil
		}
		return Status{}, fmt.Errorf("read speed-deng state: %w", err)
	}
	if active.Status != StatusActive {
		return s.statusForSession(ctx, active), nil
	}
	if !startup && active.FirstQuotaCheckAt != nil && now.Before(*active.FirstQuotaCheckAt) {
		return s.statusForSession(ctx, active), nil
	}
	targets, err := s.provider.ListEligibleCodexOAuth(ctx)
	if err != nil {
		check := QuotaCheck{Warning: err.Error()}
		if updated, updateErr := s.repo.UpdateQuotaCheck(ctx, active.ID, check, now); updateErr == nil {
			s.cacheSession(updated)
			return s.statusForSession(ctx, updated), nil
		}
		return s.statusForSession(ctx, active), nil
	}
	targets = sortQuotaTargets(targets)
	check := QuotaCheck{EligibleCount: len(targets)}
	warnings := make([]string, 0)
	for _, target := range targets {
		snapshot, refreshErr := s.provider.RefreshCodexQuota(ctx, target)
		if refreshErr != nil {
			check.SkippedCount++
			warnings = append(warnings, target.SiteName+": "+refreshErr.Error())
			continue
		}
		if !snapshot.HasWeekly {
			check.SkippedCount++
			warnings = append(warnings, target.SiteName+": weekly quota unavailable")
			continue
		}
		check.CheckedCount++
		if snapshot.WeeklyRemainingPercent > 99 {
			check.Recovered = true
			break
		}
	}
	if len(warnings) > 0 {
		check.Warning = strings.Join(warnings, "; ")
	}
	updated, updateErr := s.repo.UpdateQuotaCheck(ctx, active.ID, check, now)
	if updateErr != nil {
		return Status{}, fmt.Errorf("save speed-deng quota check: %w", updateErr)
	}
	active = updated
	s.cacheSession(updated)
	if check.Recovered {
		reason := StopReasonQuotaRecovered
		if startup {
			reason = StopReasonStartupQuotaRecovered
		}
		stopped, err := s.repo.Stop(ctx, active.ID, reason, now)
		if err != nil {
			return Status{}, fmt.Errorf("auto-stop speed-deng session: %w", err)
		}
		s.cacheSession(stopped)
	}
	return s.Status(ctx)
}

func (s *Service) StartupCheck(ctx context.Context, now time.Time) (Status, error) {
	return s.CheckAndAutoStop(ctx, now, true)
}

func (s *Service) Cleanup(ctx context.Context, cutoff time.Time) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, ErrServiceUnavailable
	}
	return s.repo.DeleteBefore(ctx, cutoff)
}

func (s *Service) CleanupWithRetention(ctx context.Context, now time.Time, retentionDays int) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, ErrServiceUnavailable
	}
	if now.IsZero() {
		now = time.Now()
	}
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := s.timeZone.StartOfDay(now).AddDate(0, 0, -retentionDays)
	return s.Cleanup(ctx, cutoff)
}

func (s *Service) readSession(ctx context.Context) (Session, error) {
	if s == nil || s.repo == nil {
		return Session{}, ErrServiceUnavailable
	}
	s.stateMu.RLock()
	if s.stateOK && time.Since(s.stateAt) < stateCacheTTL {
		session, err := s.state, s.stateErr
		s.stateMu.RUnlock()
		return session, err
	}
	s.stateMu.RUnlock()

	session, err := s.repo.Active(ctx)
	if err == nil {
		s.cacheSession(session)
	} else if errors.Is(err, ErrNoActiveSession) {
		s.cacheNoSession()
	}
	return session, err
}

func (s *Service) cacheSession(session Session) {
	if s == nil {
		return
	}
	s.stateMu.Lock()
	s.state = session
	s.stateErr = nil
	s.stateAt = time.Now()
	s.stateOK = true
	s.stateMu.Unlock()
}

func (s *Service) cacheNoSession() {
	if s == nil {
		return
	}
	s.stateMu.Lock()
	s.state = Session{Status: StatusInactive}
	s.stateErr = ErrNoActiveSession
	s.stateAt = time.Now()
	s.stateOK = true
	s.stateMu.Unlock()
}

func (s *Service) invalidateSessionCache() {
	if s == nil {
		return
	}
	s.stateMu.Lock()
	s.stateOK = false
	s.stateMu.Unlock()
}

func (s *Service) statusForSession(ctx context.Context, session Session) Status {
	status := Status{
		Active:           session.Status == StatusActive,
		State:            session.Status,
		SessionID:        uuidPtr(session.ID),
		StartedAt:         timePtr(session.StartedAt),
		FirstQuotaCheckAt: session.FirstQuotaCheckAt,
		StoppedAt:         session.StoppedAt,
		StopReason:        session.StopReason,
		LastQuotaCheckAt:  session.LastQuotaCheckAt,
		QuotaCheck:        session.QuotaCheck,
	}
	if status.State == "" {
		if status.Active {
			status.State = StatusActive
		} else {
			status.State = StatusStopped
		}
	}
	if count, err := s.repo.CountEvents(ctx, session.ID); err == nil {
		status.EventCount = count
	}
	return status
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	return &value
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func sortQuotaTargets(targets []QuotaTarget) []QuotaTarget {
	result := append([]QuotaTarget(nil), targets...)
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].SiteName) < strings.ToLower(result[j].SiteName)
	})
	return result
}
