package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	requestAnalyticsUnknownKey   = "__unknown__"
	requestAnalyticsUnknownLabel = "未识别"
)

// RequestAnalyticsAggregate is a database-side aggregate row. The repository
// deliberately returns grouped rows instead of request details for bar and
// sankey views so the usage service never needs to materialize a large range
// of logs in Go memory.
type RequestAnalyticsAggregate struct {
	BucketStart      time.Time
	SiteKey          string
	SiteLabel        string
	ModelKey         string
	ModelLabel       string
	APIKeyKey        string
	APIKeyLabel      string
	RequestCount     int64
	TotalTokens      int64
	Cost             float64
	HasCost          bool
	MissingCostCount int64
	Currency         string
}

// RequestAnalyticsScatterBucket is a database-side fifteen-minute cost
// aggregate. It intentionally contains no request-level dimensions because
// the scatter view plots one point for the selected time bucket.
type RequestAnalyticsScatterBucket struct {
	BucketStart      time.Time
	RequestCount     int64
	TotalTokens      int64
	Cost             float64
	HasCost          bool
	MissingCostCount int64
	Currency         string
}

type RequestAnalyticsQuery struct {
	CreatedFrom   time.Time
	CreatedTo     time.Time
	ModelKeys     []string
	SiteIDs       []uuid.UUID
	APIKeyIDs     []uuid.UUID
	Success       *bool
	Currency      string
	ApplyCurrency bool
}

func normalizeRequestAnalyticsAggregate(row RequestAnalyticsAggregate) RequestAnalyticsAggregate {
	row.SiteKey, row.SiteLabel = analyticsDimensionDefaults(row.SiteKey, row.SiteLabel)
	row.ModelKey, row.ModelLabel = analyticsDimensionDefaults(row.ModelKey, row.ModelLabel)
	row.APIKeyKey, row.APIKeyLabel = analyticsDimensionDefaults(row.APIKeyKey, row.APIKeyLabel)
	return row
}

func analyticsDimensionDefaults(key string, label string) (string, string) {
	key = strings.TrimSpace(key)
	label = strings.TrimSpace(label)
	if key == "" || strings.EqualFold(key, requestUsageSummaryNoneKey) {
		key = requestAnalyticsUnknownKey
	}
	if label == "" || strings.EqualFold(label, requestUsageSummaryNoneKey) {
		label = requestAnalyticsUnknownLabel
	}
	return key, label
}

type RequestAnalyticsRepository struct {
	db *gorm.DB
}

func NewRequestAnalyticsRepository(db *gorm.DB) RequestAnalyticsRepository {
	return RequestAnalyticsRepository{db: db}
}

func (r RequestAnalyticsRepository) ListCurrencies(ctx context.Context, query RequestAnalyticsQuery) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("request analytics store is not initialized")
	}

	summaryWhere, summaryArgs := analyticsSummaryWhere(query, "s", false)
	detailWhere, detailArgs := analyticsDetailWhere(query, "l", false)
	sql := `
		SELECT currency FROM (
			SELECT COALESCE(NULLIF(s.currency, ''), 'USD') AS currency
			FROM request_usage_daily_summaries s
			WHERE ` + summaryWhere + `
			UNION
			SELECT COALESCE(NULLIF(u.currency, ''), NULLIF(l.metadata->>'currency', ''), 'USD') AS currency
			FROM request_logs l
			LEFT JOIN usage_records u ON u.request_log_id = l.id
			LEFT JOIN canonical_models cm ON cm.id = l.canonical_model_id
			WHERE ` + detailWhere + `
		) currencies
		WHERE currency <> ''
		ORDER BY currency`
	args := append(summaryArgs, detailArgs...)
	var rows []struct {
		Currency string
	}
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list request analytics currencies: %w", err)
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		if value := strings.TrimSpace(row.Currency); value != "" {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (r RequestAnalyticsRepository) ListSummaryAggregates(ctx context.Context, query RequestAnalyticsQuery, from time.Time, to time.Time) ([]RequestAnalyticsAggregate, error) {
	if r.db == nil {
		return nil, fmt.Errorf("request analytics store is not initialized")
	}
	query.CreatedFrom = from
	query.CreatedTo = to
	where, args := analyticsSummaryWhere(query, "s", true)
	currencySelect := "'TOKENS'"
	groupCurrency := ""
	if query.ApplyCurrency {
		currencySelect = "COALESCE(NULLIF(s.currency, ''), 'USD')"
		groupCurrency = ", COALESCE(NULLIF(s.currency, ''), 'USD')"
	}
	sql := `
		SELECT
			s.bucket_start,
			COALESCE(NULLIF(s.site_key, ''), ?) AS site_key,
			COALESCE(NULLIF(s.site_name, ''), NULLIF(s.site_slug, ''), ?) AS site_label,
			COALESCE(NULLIF(s.canonical_model_key, ''), ?) AS model_key,
			COALESCE(NULLIF(s.canonical_model_key, ''), ?) AS model_label,
			COALESCE(NULLIF(s.api_key_key, ''), ?) AS api_key_key,
			COALESCE(NULLIF(s.api_key_name, ''), ?) AS api_key_label,
			SUM(s.request_count) AS request_count,
			SUM(s.total_tokens) AS total_tokens,
			COALESCE(SUM(s.estimated_cost), 0) AS cost,
			TRUE AS has_cost,
			0 AS missing_cost_count,
			` + currencySelect + ` AS currency
		FROM request_usage_daily_summaries s
		WHERE ` + where + `
		GROUP BY s.bucket_start,
			s.site_key, s.site_name, s.site_slug,
			s.canonical_model_key, s.api_key_key, s.api_key_name` + groupCurrency + `
		ORDER BY s.bucket_start`
	args = append([]any{
		requestAnalyticsUnknownKey, requestAnalyticsUnknownLabel,
		requestAnalyticsUnknownKey, requestAnalyticsUnknownKey,
		requestAnalyticsUnknownKey, requestAnalyticsUnknownLabel,
	}, args...)
	var rows []RequestAnalyticsAggregate
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list request analytics summary aggregates: %w", err)
	}
	for index := range rows {
		rows[index] = normalizeRequestAnalyticsAggregate(rows[index])
	}
	return rows, nil
}

func (r RequestAnalyticsRepository) ListDetailAggregates(ctx context.Context, query RequestAnalyticsQuery, from time.Time, to time.Time, bucketUnit string, timeZone string) ([]RequestAnalyticsAggregate, error) {
	if r.db == nil {
		return nil, fmt.Errorf("request analytics store is not initialized")
	}
	query.CreatedFrom = from
	query.CreatedTo = to
	where, args := analyticsDetailWhere(query, "l", true)
	unit := normalizedAnalyticsBucketUnit(bucketUnit)
	currencySelect := "'TOKENS'"
	if query.ApplyCurrency {
		currencySelect = "COALESCE(NULLIF(u.currency, ''), NULLIF(l.metadata->>'currency', ''), 'USD')"
	}
	bucketExpression := "date_trunc('" + unit + "', l.created_at AT TIME ZONE ?) AT TIME ZONE ?"
	sql := `
		WITH detail AS (
			SELECT
				` + bucketExpression + ` AS bucket_start,
				COALESCE(NULLIF(l.site_id::text, ''), NULLIF(l.metadata->>'site_id', ''), ?) AS site_key,
				COALESCE(NULLIF(s.name, ''), NULLIF(l.metadata->>'site_name', ''), NULLIF(s.slug, ''), ?) AS site_label,
				COALESCE(NULLIF(cm.model_key, ''), NULLIF(l.metadata->>'canonical_model', ''), NULLIF(l.metadata->>'requested_model', ''), ?) AS model_key,
				COALESCE(NULLIF(cm.model_key, ''), NULLIF(l.metadata->>'canonical_model', ''), NULLIF(l.metadata->>'requested_model', ''), ?) AS model_label,
				COALESCE(NULLIF(l.api_key_id::text, ''), NULLIF(l.metadata->>'api_key_id', ''), NULLIF(l.metadata->>'credential_name', ''), ?) AS api_key_key,
				COALESCE(NULLIF(k.name, ''), NULLIF(l.metadata->>'credential_name', ''), ?) AS api_key_label,
				l.success,
				COALESCE(u.total_tokens, COALESCE(l.request_tokens, 0) + COALESCE(l.response_tokens, 0))::bigint AS total_tokens,
				u.estimated_cost AS cost,
				` + currencySelect + ` AS currency
			FROM request_logs l
			LEFT JOIN usage_records u ON u.request_log_id = l.id
			LEFT JOIN sites s ON s.id = l.site_id
			LEFT JOIN canonical_models cm ON cm.id = l.canonical_model_id
			LEFT JOIN api_keys k ON k.id = l.api_key_id
			WHERE ` + where + `
		)
		SELECT bucket_start, site_key, site_label, model_key, model_label,
			api_key_key, api_key_label,
			COUNT(*) AS request_count,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(cost), 0) AS cost,
			COUNT(cost) > 0 AS has_cost,
			COUNT(*) FILTER (WHERE cost IS NULL) AS missing_cost_count,
			currency
		FROM detail
		GROUP BY bucket_start, site_key, site_label, model_key, model_label,
			api_key_key, api_key_label` + detailAggregateGroupCurrency(query.ApplyCurrency) + `
		ORDER BY bucket_start`
	args = append([]any{timeZone, timeZone,
		requestAnalyticsUnknownKey, requestAnalyticsUnknownLabel,
		requestAnalyticsUnknownKey, requestAnalyticsUnknownKey,
		requestAnalyticsUnknownKey, requestAnalyticsUnknownLabel}, args...)
	var rows []RequestAnalyticsAggregate
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list request analytics detail aggregates: %w", err)
	}
	for index := range rows {
		rows[index] = normalizeRequestAnalyticsAggregate(rows[index])
	}
	return rows, nil
}

func (r RequestAnalyticsRepository) CountScatterBuckets(ctx context.Context, query RequestAnalyticsQuery, timeZone string) (int64, int64, int64, error) {
	if r.db == nil {
		return 0, 0, 0, fmt.Errorf("request analytics store is not initialized")
	}
	where, args := analyticsDetailWhere(query, "l", true)
	bucketExpression := analyticsFifteenMinuteBucketExpression("l.created_at")
	sql := `
		WITH detail AS (
			SELECT ` + bucketExpression + ` AS bucket_start,
				u.estimated_cost AS cost
			FROM request_logs l
			LEFT JOIN usage_records u ON u.request_log_id = l.id
			LEFT JOIN canonical_models cm ON cm.id = l.canonical_model_id
			WHERE ` + where + `
		)
		SELECT COUNT(*) AS total,
			COUNT(DISTINCT bucket_start) FILTER (WHERE cost IS NOT NULL) AS drawable,
			COUNT(*) FILTER (WHERE cost IS NULL) AS missing_cost
		FROM detail`
	args = append([]any{timeZone, timeZone}, args...)
	var row struct {
		Total       int64
		Drawable    int64
		MissingCost int64
	}
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&row).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("count request analytics scatter buckets: %w", err)
	}
	return row.Total, row.Drawable, row.MissingCost, nil
}

func (r RequestAnalyticsRepository) ListScatterBuckets(ctx context.Context, query RequestAnalyticsQuery, timeZone string) ([]RequestAnalyticsScatterBucket, error) {
	if r.db == nil {
		return nil, fmt.Errorf("request analytics store is not initialized")
	}
	where, args := analyticsDetailWhere(query, "l", true)
	bucketExpression := analyticsFifteenMinuteBucketExpression("l.created_at")
	sql := `
		WITH detail AS (
			SELECT ` + bucketExpression + ` AS bucket_start,
				COALESCE(u.total_tokens, COALESCE(l.request_tokens, 0) + COALESCE(l.response_tokens, 0))::bigint AS total_tokens,
				u.estimated_cost AS cost,
				COALESCE(NULLIF(u.currency, ''), NULLIF(l.metadata->>'currency', ''), 'USD') AS currency
			FROM request_logs l
			LEFT JOIN usage_records u ON u.request_log_id = l.id
			LEFT JOIN canonical_models cm ON cm.id = l.canonical_model_id
			WHERE ` + where + `
		)
		SELECT bucket_start,
			COUNT(*) AS request_count,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(cost), 0) AS cost,
			COUNT(cost) > 0 AS has_cost,
			COUNT(*) FILTER (WHERE cost IS NULL) AS missing_cost_count,
			currency
		FROM detail
		GROUP BY bucket_start, currency
		HAVING COUNT(cost) > 0
		ORDER BY bucket_start`
	args = append([]any{timeZone, timeZone}, args...)
	var rows []RequestAnalyticsScatterBucket
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list request analytics scatter buckets: %w", err)
	}
	return rows, nil
}

func analyticsFifteenMinuteBucketExpression(column string) string {
	return "date_bin(INTERVAL '15 minutes', " + column + " AT TIME ZONE ?, TIMESTAMP '1970-01-01') AT TIME ZONE ?"
}

func detailAggregateGroupCurrency(applyCurrency bool) string {
	_ = applyCurrency
	// ListDetailAggregates groups the outer query over the CTE. The usage
	// record alias is only visible inside the CTE; the projected currency
	// column is the valid outer-query expression, including the token-only
	// Sankey case where the CTE projects a constant currency value.
	return ", currency"
}

func analyticsSummaryWhere(query RequestAnalyticsQuery, alias string, includeCurrency bool) (string, []any) {
	conditions := []string{
		alias + ".bucket_start >= ?",
		alias + ".bucket_start < ?",
		alias + ".internal = FALSE",
	}
	args := []any{query.CreatedFrom, query.CreatedTo}
	if query.Success != nil {
		conditions = append(conditions, alias+".success = ?")
		args = append(args, *query.Success)
	}
	if len(query.SiteIDs) > 0 {
		conditions = append(conditions, alias+".site_id IN ("+analyticsPlaceholders(len(query.SiteIDs))+")")
		for _, id := range query.SiteIDs {
			args = append(args, id)
		}
	}
	if len(query.APIKeyIDs) > 0 {
		conditions = append(conditions, alias+".api_key_id IN ("+analyticsPlaceholders(len(query.APIKeyIDs))+")")
		for _, id := range query.APIKeyIDs {
			args = append(args, id)
		}
	}
	if len(query.ModelKeys) > 0 {
		conditions = append(conditions, alias+".canonical_model_key IN ("+analyticsPlaceholders(len(query.ModelKeys))+")")
		for _, model := range query.ModelKeys {
			args = append(args, model)
		}
	}
	if includeCurrency && query.ApplyCurrency {
		conditions = append(conditions, "COALESCE(NULLIF("+alias+".currency, ''), 'USD') = ?")
		args = append(args, query.Currency)
	}
	return strings.Join(conditions, " AND "), args
}

func analyticsDetailWhere(query RequestAnalyticsQuery, alias string, includeCurrency bool) (string, []any) {
	conditions := []string{
		alias + ".created_at >= ?",
		alias + ".created_at < ?",
		alias + ".internal = FALSE",
	}
	args := []any{query.CreatedFrom, query.CreatedTo}
	if query.Success != nil {
		conditions = append(conditions, alias+".success = ?")
		args = append(args, *query.Success)
	}
	if len(query.SiteIDs) > 0 {
		conditions = append(conditions, alias+".site_id IN ("+analyticsPlaceholders(len(query.SiteIDs))+")")
		for _, id := range query.SiteIDs {
			args = append(args, id)
		}
	}
	if len(query.APIKeyIDs) > 0 {
		conditions = append(conditions, alias+".api_key_id IN ("+analyticsPlaceholders(len(query.APIKeyIDs))+")")
		for _, id := range query.APIKeyIDs {
			args = append(args, id)
		}
	}
	if len(query.ModelKeys) > 0 {
		conditions = append(conditions, "COALESCE(NULLIF(cm.model_key, ''), NULLIF("+alias+".metadata->>'canonical_model', ''), NULLIF("+alias+".metadata->>'requested_model', '')) IN ("+analyticsPlaceholders(len(query.ModelKeys))+")")
		for _, model := range query.ModelKeys {
			args = append(args, model)
		}
	}
	if includeCurrency && query.ApplyCurrency {
		conditions = append(conditions, "COALESCE(NULLIF(u.currency, ''), NULLIF("+alias+".metadata->>'currency', ''), 'USD') = ?")
		args = append(args, query.Currency)
	}
	return strings.Join(conditions, " AND "), args
}

func analyticsPlaceholders(count int) string {
	if count <= 0 {
		return "NULL"
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func normalizedAnalyticsBucketUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "hour":
		return "hour"
	case "week":
		return "week"
	default:
		return "day"
	}
}
