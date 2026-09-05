package speeddeng

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"xlyra/server/internal/store"
)

type sessionRecord struct {
	ID                 uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Status             string    `gorm:"index:custom_speed_deng_sessions_status_idx"`
	StartedAt          time.Time `gorm:"index:custom_speed_deng_sessions_started_at_idx"`
	FirstQuotaCheckAt  *time.Time
	StoppedAt          *time.Time
	StopReason         string
	LastQuotaCheckAt   *time.Time
	QuotaEligibleCount int
	QuotaCheckedCount  int
	QuotaSkippedCount  int
	QuotaRecovered     bool
	QuotaWarning       string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (sessionRecord) TableName() string { return "custom_speed_deng_sessions" }

type eventRecord struct {
	ID                 uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	SessionID          uuid.UUID `gorm:"type:uuid;index:custom_speed_deng_events_session_created_idx,priority:1"`
	SourceRequestLogID uuid.UUID `gorm:"type:uuid;uniqueIndex:custom_speed_deng_events_source_request_idx"`
	SourceRequestID    string
	SiteID             uuid.UUID `gorm:"type:uuid;index:custom_speed_deng_events_site_created_idx,priority:1"`
	SiteName           string
	ModelKey           string
	APIKeyID           *uuid.UUID `gorm:"type:uuid"`
	APIKeyName         string
	PromptTokens       int
	CompletionTokens   int
	CachedTokens       int64
	TotalTokens        int
	EstimatedCostUSD   *float64 `gorm:"type:numeric(18,8)"`
	Currency           string
	CreatedAt          time.Time `gorm:"index:custom_speed_deng_events_site_created_idx,priority:2;index:custom_speed_deng_events_session_created_idx,priority:2"`
}

func (eventRecord) TableName() string { return "custom_speed_deng_events" }

type repository struct {
	db *gorm.DB
}

func newRepository(db *store.Store) Repository {
	if db == nil || db.DB() == nil {
		return &repository{}
	}
	return &repository{db: db.DB()}
}

func NewEventSource(db *store.Store) EventSource {
	if db == nil || db.DB() == nil {
		return nil
	}
	return &repository{db: db.DB()}
}

func EnsureSchema(db *store.Store) error {
	if db == nil || db.DB() == nil {
		return nil
	}
	orm := db.DB()
	if err := orm.AutoMigrate(&sessionRecord{}, &eventRecord{}); err != nil {
		return fmt.Errorf("ensure speed-deng tables: %w", err)
	}
	statements := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS custom_speed_deng_active_session_idx ON custom_speed_deng_sessions (status) WHERE status = 'active'",
		"CREATE UNIQUE INDEX IF NOT EXISTS custom_speed_deng_events_source_request_idx ON custom_speed_deng_events (source_request_log_id)",
		"CREATE INDEX IF NOT EXISTS custom_speed_deng_events_site_created_idx ON custom_speed_deng_events (site_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS custom_speed_deng_events_session_created_idx ON custom_speed_deng_events (session_id, created_at DESC)",
	}
	for _, statement := range statements {
		if err := orm.Exec(statement).Error; err != nil {
			return fmt.Errorf("ensure speed-deng index: %w", err)
		}
	}
	return nil
}

func (r *repository) Active(ctx context.Context) (Session, error) {
	if r == nil || r.db == nil {
		return Session{}, ErrServiceUnavailable
	}
	var row sessionRecord
	err := r.db.WithContext(ctx).Where("status = ?", StatusActive).Order("started_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = r.db.WithContext(ctx).Order("started_at DESC").First(&row).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Session{}, ErrNoActiveSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("read speed-deng session: %w", err)
	}
	return sessionFromRecord(row), nil
}

func (r *repository) Start(ctx context.Context, now time.Time) (Session, error) {
	if r == nil || r.db == nil {
		return Session{}, ErrServiceUnavailable
	}
	if now.IsZero() {
		now = time.Now()
	}
	var result Session
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing sessionRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ?", StatusActive).Order("started_at DESC").First(&existing).Error
		if err == nil {
			result = sessionFromRecord(existing)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row := newSessionRecord(now)
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		result = sessionFromRecord(row)
		return nil
	})
	if err != nil {
		return Session{}, fmt.Errorf("start speed-deng session: %w", err)
	}
	return result, nil
}

func (r *repository) Stop(ctx context.Context, sessionID uuid.UUID, reason string, now time.Time) (Session, error) {
	if r == nil || r.db == nil {
		return Session{}, ErrServiceUnavailable
	}
	if now.IsZero() {
		now = time.Now()
	}
	result := r.db.WithContext(ctx).Model(&sessionRecord{}).
		Where("id = ? AND status = ?", sessionID, StatusActive).
		Updates(map[string]any{"status": StatusStopped, "stopped_at": now, "stop_reason": reason, "updated_at": now})
	if result.Error != nil {
		return Session{}, fmt.Errorf("stop speed-deng session: %w", result.Error)
	}
	var row sessionRecord
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&row).Error; err != nil {
		return Session{}, fmt.Errorf("read stopped speed-deng session: %w", err)
	}
	return sessionFromRecord(row), nil
}

func (r *repository) UpdateQuotaCheck(ctx context.Context, sessionID uuid.UUID, check QuotaCheck, now time.Time) (Session, error) {
	if r == nil || r.db == nil {
		return Session{}, ErrServiceUnavailable
	}
	if now.IsZero() {
		now = time.Now()
	}
	result := r.db.WithContext(ctx).Model(&sessionRecord{}).Where("id = ?", sessionID).Updates(map[string]any{
		"last_quota_check_at":  now,
		"quota_eligible_count": check.EligibleCount,
		"quota_checked_count":  check.CheckedCount,
		"quota_skipped_count":  check.SkippedCount,
		"quota_recovered":      check.Recovered,
		"quota_warning":        check.Warning,
		"updated_at":           now,
	})
	if result.Error != nil {
		return Session{}, fmt.Errorf("update speed-deng quota check: %w", result.Error)
	}
	var row sessionRecord
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&row).Error; err != nil {
		return Session{}, fmt.Errorf("read speed-deng quota check: %w", err)
	}
	return sessionFromRecord(row), nil
}

func (r *repository) CountEvents(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	if r == nil || r.db == nil {
		return 0, ErrServiceUnavailable
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&eventRecord{}).Where("session_id = ?", sessionID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count speed-deng events: %w", err)
	}
	return count, nil
}

func (r *repository) RecordEvent(ctx context.Context, event Event) error {
	if r == nil || r.db == nil {
		return ErrServiceUnavailable
	}
	row := eventRecordFromEvent(event)
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_request_log_id"}},
		DoNothing: true,
	}).Create(&row).Error; err != nil {
		return fmt.Errorf("create speed-deng event: %w", err)
	}
	return nil
}

func (r *repository) ListEvents(ctx context.Context, siteID uuid.UUID, from time.Time, to time.Time) ([]Event, error) {
	if r == nil || r.db == nil {
		return nil, ErrServiceUnavailable
	}
	var rows []eventRecord
	query := r.db.WithContext(ctx).Where("site_id = ?", siteID)
	if !from.IsZero() {
		query = query.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("created_at < ?", to)
	}
	if err := query.Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list speed-deng events: %w", err)
	}
	result := make([]Event, 0, len(rows))
	for _, row := range rows {
		result = append(result, eventFromRecord(row))
	}
	return result, nil
}

func (r *repository) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if r == nil || r.db == nil {
		return 0, ErrServiceUnavailable
	}
	if cutoff.IsZero() {
		return 0, nil
	}
	var deleted int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("created_at < ? AND session_id NOT IN (SELECT id FROM custom_speed_deng_sessions WHERE status = ?)", cutoff, StatusActive).Delete(&eventRecord{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected
		return tx.Where("status <> ? AND stopped_at IS NOT NULL AND stopped_at < ?", StatusActive, cutoff).Delete(&sessionRecord{}).Error
	})
	if err != nil {
		return deleted, fmt.Errorf("delete speed-deng records before cutoff: %w", err)
	}
	return deleted, nil
}

func sessionFromRecord(row sessionRecord) Session {
	return Session{
		ID:                row.ID,
		Status:            row.Status,
		StartedAt:         row.StartedAt,
		FirstQuotaCheckAt: row.FirstQuotaCheckAt,
		StoppedAt:         row.StoppedAt,
		StopReason:        row.StopReason,
		LastQuotaCheckAt:  row.LastQuotaCheckAt,
		QuotaCheck:        QuotaCheck{
			EligibleCount: row.QuotaEligibleCount,
			CheckedCount:  row.QuotaCheckedCount,
			SkippedCount:  row.QuotaSkippedCount,
			Recovered:     row.QuotaRecovered,
			Warning:       row.QuotaWarning,
		},
	}
}

func newSessionRecord(now time.Time) sessionRecord {
	if now.IsZero() {
		now = time.Now()
	}
	firstQuotaCheckAt := now.Add(firstQuotaCheckDelay)
	return sessionRecord{
		ID:                uuid.New(),
		Status:            StatusActive,
		StartedAt:         now,
		FirstQuotaCheckAt: &firstQuotaCheckAt,
	}
}

func eventRecordFromInput(input CaptureInput) eventRecord {
	return eventRecordFromEvent(Event{
		ID:                 uuid.New(),
		SessionID:          input.SessionID,
		SourceRequestLogID: input.SourceRequestLogID,
		SourceRequestID:    input.SourceRequestID,
		SiteID:             input.SiteID,
		SiteName:           input.SiteName,
		ModelKey:           input.ModelKey,
		APIKeyID:           input.APIKeyID,
		APIKeyName:         input.APIKeyName,
		PromptTokens:       input.PromptTokens,
		CompletionTokens:   input.CompletionTokens,
		CachedTokens:       input.CachedTokens,
		TotalTokens:        input.TotalTokens,
		EstimatedCostUSD:   input.EstimatedCostUSD,
		Currency:           input.Currency,
		CreatedAt:          input.CreatedAt,
	})
}

func eventRecordFromEvent(event Event) eventRecord {
	var apiKeyID *uuid.UUID
	if event.APIKeyID != uuid.Nil {
		id := event.APIKeyID
		apiKeyID = &id
	}
	return eventRecord{
		ID:                 event.ID,
		SessionID:          event.SessionID,
		SourceRequestLogID: event.SourceRequestLogID,
		SourceRequestID:    event.SourceRequestID,
		SiteID:             event.SiteID,
		SiteName:           event.SiteName,
		ModelKey:           event.ModelKey,
		APIKeyID:           apiKeyID,
		APIKeyName:         event.APIKeyName,
		PromptTokens:       event.PromptTokens,
		CompletionTokens:   event.CompletionTokens,
		CachedTokens:       event.CachedTokens,
		TotalTokens:        event.TotalTokens,
		EstimatedCostUSD:   event.EstimatedCostUSD,
		Currency:           event.Currency,
		CreatedAt:          event.CreatedAt,
	}
}

func eventFromRecord(row eventRecord) Event {
	apiKeyID := uuid.Nil
	if row.APIKeyID != nil {
		apiKeyID = *row.APIKeyID
	}
	return Event{
		ID:                 row.ID,
		SessionID:          row.SessionID,
		SourceRequestLogID: row.SourceRequestLogID,
		SourceRequestID:    row.SourceRequestID,
		SiteID:             row.SiteID,
		SiteName:           row.SiteName,
		ModelKey:           row.ModelKey,
		APIKeyID:           apiKeyID,
		APIKeyName:         row.APIKeyName,
		PromptTokens:       row.PromptTokens,
		CompletionTokens:   row.CompletionTokens,
		CachedTokens:       row.CachedTokens,
		TotalTokens:        row.TotalTokens,
		EstimatedCostUSD:   row.EstimatedCostUSD,
		Currency:           row.Currency,
		CreatedAt:          row.CreatedAt,
	}
}
