package speeddeng

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
)

type fakeRepository struct {
	active        *Session
	startCalls    int
	stopCalls     int
	checks        []QuotaCheck
	events        []Event
	eventErr      error
	count         int64
	startErr      error
	activeErr     error
	stopErr       error
	checkErr      error
	deletedCutoff *time.Time
	activeCalls   int
}

func (f *fakeRepository) Active(context.Context) (Session, error) {
	f.activeCalls++
	if f.activeErr != nil {
		return Session{}, f.activeErr
	}
	if f.active == nil {
		return Session{}, ErrNoActiveSession
	}
	return *f.active, nil
}

func (f *fakeRepository) Start(_ context.Context, now time.Time) (Session, error) {
	f.startCalls++
	if f.startErr != nil {
		return Session{}, f.startErr
	}
	if f.active != nil && f.active.Status == StatusActive {
		return *f.active, nil
	}
	session := Session{ID: uuid.New(), Status: StatusActive, StartedAt: now}
	f.active = &session
	return session, nil
}

func (f *fakeRepository) Stop(_ context.Context, id uuid.UUID, reason string, now time.Time) (Session, error) {
	f.stopCalls++
	if f.stopErr != nil {
		return Session{}, f.stopErr
	}
	if f.active == nil || f.active.ID != id {
		return Session{}, ErrNoActiveSession
	}
	f.active.Status = StatusStopped
	f.active.StoppedAt = &now
	f.active.StopReason = reason
	return *f.active, nil
}

func (f *fakeRepository) UpdateQuotaCheck(_ context.Context, id uuid.UUID, check QuotaCheck, now time.Time) (Session, error) {
	if f.checkErr != nil {
		return Session{}, f.checkErr
	}
	if f.active == nil || f.active.ID != id {
		return Session{}, ErrNoActiveSession
	}
	f.checks = append(f.checks, check)
	f.active.LastQuotaCheckAt = &now
	f.active.QuotaCheck = check
	return *f.active, nil
}

func (f *fakeRepository) CountEvents(context.Context, uuid.UUID) (int64, error) {
	return f.count + int64(len(f.events)), nil
}

func (f *fakeRepository) RecordEvent(_ context.Context, event Event) error {
	if f.eventErr != nil {
		return f.eventErr
	}
	f.events = append(f.events, event)
	return nil
}

func (f *fakeRepository) ListEvents(context.Context, uuid.UUID, time.Time, time.Time) ([]Event, error) {
	return append([]Event(nil), f.events...), nil
}

func (f *fakeRepository) DeleteBefore(_ context.Context, cutoff time.Time) (int64, error) {
	f.deletedCutoff = &cutoff
	return 0, nil
}

type fakeQuotaProvider struct {
	targets   []QuotaTarget
	snapshots map[uuid.UUID]QuotaSnapshot
	errors    map[uuid.UUID]error
}

type checkingQuotaProvider struct {
	fakeQuotaProvider
	eligible map[uuid.UUID]bool
	err      error
}

func (p checkingQuotaProvider) IsCodexOAuthSite(_ context.Context, siteID uuid.UUID) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	return p.eligible != nil && p.eligible[siteID], nil
}

func (f fakeQuotaProvider) ListEligibleCodexOAuth(context.Context) ([]QuotaTarget, error) {
	return append([]QuotaTarget(nil), f.targets...), nil
}

func (f fakeQuotaProvider) RefreshCodexQuota(_ context.Context, target QuotaTarget) (QuotaSnapshot, error) {
	if err := f.errors[target.SiteID]; err != nil {
		return QuotaSnapshot{}, err
	}
	return f.snapshots[target.SiteID], nil
}

func newTestService(repo *fakeRepository, provider fakeQuotaProvider) *Service {
	return NewServiceWithDependencies(repo, provider, config.LoadTimeZone("UTC"), nil)
}

func TestStartRejectsWhenNoEligibleCodexOAuthAccountExists(t *testing.T) {
	repo := &fakeRepository{}
	service := newTestService(repo, fakeQuotaProvider{})

	_, err := service.Start(context.Background(), time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrNoEligibleAccounts) {
		t.Fatalf("Start error = %v, want ErrNoEligibleAccounts", err)
	}
	if repo.startCalls != 0 {
		t.Fatalf("start calls = %d, want 0", repo.startCalls)
	}
}

func TestStartAndStopCreateOneGlobalSession(t *testing.T) {
	repo := &fakeRepository{}
	provider := fakeQuotaProvider{targets: []QuotaTarget{{SiteID: uuid.New()}}}
	service := newTestService(repo, provider)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)

	first, err := service.Start(context.Background(), now)
	if err != nil {
		t.Fatalf("first Start error = %v", err)
	}
	second, err := service.Start(context.Background(), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("second Start error = %v", err)
	}
	if first.ID != second.ID || repo.startCalls != 1 {
		t.Fatalf("sessions = %s/%s, start calls = %d; want one id and one create", first.ID, second.ID, repo.startCalls)
	}

	stopped, err := service.Stop(context.Background(), now.Add(2*time.Minute), StopReasonManual)
	if err != nil {
		t.Fatalf("Stop error = %v", err)
	}
	if stopped.Status != StatusStopped || stopped.StopReason != StopReasonManual {
		t.Fatalf("stopped session = %#v", stopped)
	}
}

func TestBeginRequestReusesShortLivedSessionStateCache(t *testing.T) {
	session := Session{ID: uuid.New(), Status: StatusActive, StartedAt: time.Now().Add(-time.Minute)}
	repo := &fakeRepository{active: &session}
	service := newTestService(repo, fakeQuotaProvider{})
	if _, ok := service.BeginRequest(context.Background()); !ok {
		t.Fatal("first BeginRequest returned inactive")
	}
	if _, ok := service.BeginRequest(context.Background()); !ok {
		t.Fatal("second BeginRequest returned inactive")
	}
	if repo.activeCalls != 1 {
		t.Fatalf("active repository calls = %d, want one cached read", repo.activeCalls)
	}
}

func TestCaptureUsesSessionCapturedBeforeStop(t *testing.T) {
	session := Session{ID: uuid.New(), Status: StatusActive, StartedAt: time.Now().Add(-time.Minute)}
	repo := &fakeRepository{active: &session}
	service := newTestService(repo, fakeQuotaProvider{})

	ctx, ok := service.BeginRequest(context.Background())
	if !ok {
		t.Fatal("BeginRequest returned inactive")
	}
	if _, err := service.Stop(context.Background(), time.Now(), StopReasonManual); err != nil {
		t.Fatalf("Stop error = %v", err)
	}

	err := service.CaptureSuccess(ctx, CaptureInput{
		SourceRequestLogID: uuid.New(),
		SourceRequestID:    "req-1",
		SiteID:             uuid.New(),
		SiteType:           "codex",
		APIKeyName:         "Wilson",
		PromptTokens:       10,
		CompletionTokens:   5,
		TotalTokens:        15,
		EstimatedCostUSD:   floatPtr(0.25),
		Success:            true,
		CreatedAt:          time.Now(),
	})
	if err != nil {
		t.Fatalf("CaptureSuccess error = %v", err)
	}
	if len(repo.events) != 1 || repo.events[0].SessionID != session.ID {
		t.Fatalf("events = %#v, want one event tied to original session", repo.events)
	}
}

func TestCaptureFiltersNonCodexFailedAndEmptyUsage(t *testing.T) {
	session := Session{ID: uuid.New(), Status: StatusActive, StartedAt: time.Now().Add(-time.Minute)}
	repo := &fakeRepository{active: &session}
	service := newTestService(repo, fakeQuotaProvider{})
	ctx, ok := service.BeginRequest(context.Background())
	if !ok {
		t.Fatal("BeginRequest returned inactive")
	}

	inputs := []CaptureInput{
		{SourceRequestLogID: uuid.New(), SiteType: "anthropic", Success: true, TotalTokens: 10},
		{SourceRequestLogID: uuid.New(), SiteType: "codex", Success: false, TotalTokens: 10},
		{SourceRequestLogID: uuid.New(), SiteType: "codex", Success: true},
		{SourceRequestLogID: uuid.New(), SiteType: "codex", Success: true, TotalTokens: 10},
	}
	for _, input := range inputs {
		if err := service.CaptureSuccess(ctx, input); err != nil {
			t.Fatalf("CaptureSuccess(%#v) error = %v", input, err)
		}
	}
	if len(repo.events) != 1 || repo.events[0].TotalTokens != 10 {
		t.Fatalf("events = %#v, want one billable Codex event", repo.events)
	}
}

func TestCaptureIgnoresNonUSDCostEvents(t *testing.T) {
	session := Session{ID: uuid.New(), Status: StatusActive, StartedAt: time.Now().Add(-time.Minute)}
	repo := &fakeRepository{active: &session}
	service := newTestService(repo, fakeQuotaProvider{})
	ctx, ok := service.BeginRequest(context.Background())
	if !ok {
		t.Fatal("BeginRequest returned inactive")
	}
	cost := 1.0
	if err := service.CaptureSuccess(ctx, CaptureInput{SessionID: session.ID, SourceRequestLogID: uuid.New(), SiteType: "codex", Success: true, TotalTokens: 10, EstimatedCostUSD: &cost, Currency: "EUR"}); err != nil {
		t.Fatalf("CaptureSuccess error = %v", err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("events = %#v, want no non-USD event", repo.events)
	}
}

func TestCaptureSkipsCodexSiteWithoutOAuthConnection(t *testing.T) {
	siteID := uuid.New()
	session := Session{ID: uuid.New(), Status: StatusActive, StartedAt: time.Now().Add(-time.Minute)}
	repo := &fakeRepository{active: &session}
	provider := checkingQuotaProvider{fakeQuotaProvider: fakeQuotaProvider{}, eligible: map[uuid.UUID]bool{siteID: false}}
	service := NewServiceWithDependencies(repo, provider, config.LoadTimeZone("UTC"), nil)
	ctx, ok := service.BeginRequest(context.Background())
	if !ok {
		t.Fatal("BeginRequest returned inactive")
	}
	cost := 0.5
	if err := service.CaptureSuccess(ctx, CaptureInput{
		SourceRequestLogID: uuid.New(),
		SiteID:             siteID,
		SiteType:           "codex",
		Success:            true,
		TotalTokens:        10,
		EstimatedCostUSD:   &cost,
	}); err != nil {
		t.Fatalf("CaptureSuccess error = %v", err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("events = %#v, want no event for a non-OAuth Codex site", repo.events)
	}
}

func TestAutoCheckStopsWhenAnyEligibleAccountRecoversAbove99Percent(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	repo := &fakeRepository{active: &Session{ID: uuid.New(), Status: StatusActive}}
	provider := fakeQuotaProvider{
		targets: []QuotaTarget{{SiteID: first}, {SiteID: second}},
		snapshots: map[uuid.UUID]QuotaSnapshot{
			first:  {HasWeekly: true, WeeklyRemainingPercent: 50},
			second: {HasWeekly: true, WeeklyRemainingPercent: 100},
		},
	}
	service := newTestService(repo, provider)

	status, err := service.CheckAndAutoStop(context.Background(), time.Now(), false)
	if err != nil {
		t.Fatalf("CheckAndAutoStop error = %v", err)
	}
	if status.Active || repo.active.StopReason != StopReasonQuotaRecovered {
		t.Fatalf("status/session = %#v/%#v, want stopped quota recovery", status, repo.active)
	}
}

func TestStartupCheckUsesStartupQuotaRecoveryReason(t *testing.T) {
	siteID := uuid.New()
	repo := &fakeRepository{active: &Session{ID: uuid.New(), Status: StatusActive}}
	service := newTestService(repo, fakeQuotaProvider{
		targets:   []QuotaTarget{{SiteID: siteID}},
		snapshots: map[uuid.UUID]QuotaSnapshot{siteID: {HasWeekly: true, WeeklyRemainingPercent: 100}},
	})

	if _, err := service.StartupCheck(context.Background(), time.Now()); err != nil {
		t.Fatalf("StartupCheck error = %v", err)
	}
	if repo.active.StopReason != StopReasonStartupQuotaRecovered {
		t.Fatalf("stop reason = %q, want %q", repo.active.StopReason, StopReasonStartupQuotaRecovered)
	}
}

func TestAutoCheckSkipsQuotaErrorsAndKeepsSessionActive(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	repo := &fakeRepository{active: &Session{ID: uuid.New(), Status: StatusActive}}
	provider := fakeQuotaProvider{
		targets: []QuotaTarget{{SiteID: first}, {SiteID: second}},
		snapshots: map[uuid.UUID]QuotaSnapshot{
			second: {HasWeekly: true, WeeklyRemainingPercent: 50},
		},
		errors: map[uuid.UUID]error{first: errors.New("temporary upstream error")},
	}
	service := newTestService(repo, provider)

	status, err := service.CheckAndAutoStop(context.Background(), time.Now(), false)
	if err != nil {
		t.Fatalf("CheckAndAutoStop error = %v", err)
	}
	if !status.Active || status.QuotaCheck.CheckedCount != 1 || status.QuotaCheck.SkippedCount != 1 {
		t.Fatalf("status = %#v, want active with one checked and one skipped", status)
	}
}

func TestAutoCheckDoesNotStopAtExactly99Percent(t *testing.T) {
	siteID := uuid.New()
	repo := &fakeRepository{active: &Session{ID: uuid.New(), Status: StatusActive}}
	service := newTestService(repo, fakeQuotaProvider{
		targets:   []QuotaTarget{{SiteID: siteID}},
		snapshots: map[uuid.UUID]QuotaSnapshot{siteID: {HasWeekly: true, WeeklyRemainingPercent: 99}},
	})
	status, err := service.CheckAndAutoStop(context.Background(), time.Now(), false)
	if err != nil {
		t.Fatalf("CheckAndAutoStop error = %v", err)
	}
	if !status.Active {
		t.Fatalf("status = %#v, want active at exactly 99%%", status)
	}
}

func TestCleanupWithRetentionUsesServiceTimezoneDayBoundary(t *testing.T) {
	repo := &fakeRepository{}
	service := newTestService(repo, fakeQuotaProvider{})
	now := time.Date(2026, 9, 4, 15, 0, 0, 0, time.UTC)
	if _, err := service.CleanupWithRetention(context.Background(), now, 7); err != nil {
		t.Fatalf("CleanupWithRetention error = %v", err)
	}
	if repo.deletedCutoff == nil || !repo.deletedCutoff.Equal(time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("cutoff = %v, want 2026-08-28 UTC midnight", repo.deletedCutoff)
	}
}

func floatPtr(value float64) *float64 { return &value }
