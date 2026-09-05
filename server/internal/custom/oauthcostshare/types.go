package oauthcostshare

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	UnsupportedReasonOAuth = "not_oauth"
	UnsupportedReasonPlan  = "unsupported_plan"
	UnsupportedReasonQuota = "quota_not_configured"
	UnsupportedReasonFee   = "fee_not_configured"
)

type OAuthSite struct {
	ID       uuid.UUID
	Name     string
	PlanType string
	IsOAuth  bool
}

type UsageSource interface {
	Site(ctx context.Context, siteID uuid.UUID) (OAuthSite, error)
	Usage(ctx context.Context, siteID uuid.UUID, from time.Time, to time.Time) ([]UsageRow, error)
}

type SpeedUsageSource interface {
	Usage(ctx context.Context, siteID uuid.UUID, from time.Time, to time.Time) ([]UsageRow, error)
}

type CostShareQuery struct {
	SiteID      uuid.UUID
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type UsageRow struct {
	ModelKey     string
	APIKeyKey    string
	APIKeyName   string
	Cost         float64
	RequestCount int64
	SpeedDeng    bool
}

type CostShareItem struct {
	Name          string  `json:"name"`
	UsageCost     float64 `json:"usage_cost"`
	UsageShare    float64 `json:"usage_share"`
	AllocatedCost float64 `json:"allocated_cost"`
}

type CostShareData struct {
	Supported         bool            `json:"supported"`
	UnsupportedReason string          `json:"unsupported_reason,omitempty"`
	UsageCurrency     string          `json:"usage_currency"`
	FeeCurrency       string          `json:"fee_currency"`
	SiteID            string          `json:"site_id"`
	SiteLabel         string          `json:"site_label"`
	PlanType          string          `json:"plan_type"`
	SingleQuota       float64         `json:"single_quota"`
	ResetCount        int             `json:"reset_count"`
	TotalQuota        float64         `json:"total_quota"`
	AccountFee        float64         `json:"account_fee"`
	TotalUsageCost    float64         `json:"total_usage_cost"`
	TotalUsageRatio   float64         `json:"total_usage_ratio"`
	AllocatedCost     float64         `json:"allocated_cost"`
	UnallocatedCost   float64         `json:"unallocated_cost"`
	OverQuota         bool            `json:"over_quota"`
	Items             []CostShareItem `json:"items"`
}

type CostShareMeta struct {
	RangeStart             string `json:"range_start"`
	RangeEnd               string `json:"range_end"`
	TimeZone               string `json:"timezone"`
	Currency               string `json:"currency"`
	FeeCurrency            string `json:"fee_currency"`
	RequestCount           int64  `json:"request_count"`
	MissingCostRequests    int64  `json:"missing_cost_requests"`
	SpeedDengDataAvailable bool   `json:"speed_deng_data_available"`
	SpeedDengWarning       string `json:"speed_deng_warning,omitempty"`
}

type CostShareResponse struct {
	Meta CostShareMeta `json:"meta"`
	Data CostShareData `json:"data"`
}
