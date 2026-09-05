package oauthcostshare

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
	"xlyra/server/internal/store"
)

type Service struct {
	source      UsageSource
	speedSource SpeedUsageSource
	confFile    *config.ConfigFile
	timeZone    config.TimeZone
}

func NewService(db *store.Store, confFile *config.ConfigFile, timeZones ...config.TimeZone) *Service {
	timeZone := config.TimeZoneOrDefault(timeZones...)
	return &Service{
		source:      newRepository(db, timeZone),
		speedSource: newSpeedRepository(db),
		confFile:    confFile,
		timeZone:    timeZone,
	}
}

func NewServiceWithSource(source UsageSource, confFile *config.ConfigFile, timeZone config.TimeZone) *Service {
	return NewServiceWithSources(source, nil, confFile, timeZone)
}

func NewServiceWithSources(source UsageSource, speedSource SpeedUsageSource, confFile *config.ConfigFile, timeZone config.TimeZone) *Service {
	return &Service{source: source, speedSource: speedSource, confFile: confFile, timeZone: config.TimeZoneOrDefault(timeZone)}
}

func (s *Service) CostShare(ctx context.Context, query CostShareQuery, now time.Time) (CostShareResponse, error) {
	if s == nil || s.source == nil {
		return CostShareResponse{}, fmt.Errorf("oauth cost share service is not initialized")
	}
	if query.SiteID == uuid.Nil {
		return CostShareResponse{}, fmt.Errorf("site_id is required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	timeZone := config.TimeZoneOrDefault(s.timeZone)
	now = timeZone.In(now)
	from := now.Add(-7 * 24 * time.Hour)
	to := now
	if query.CreatedFrom != nil {
		from = timeZone.In(*query.CreatedFrom)
	}
	if query.CreatedTo != nil {
		to = timeZone.In(*query.CreatedTo)
	}
	if to.After(now) {
		to = now
	}
	if !to.After(from) {
		return CostShareResponse{}, fmt.Errorf("created_to must be after created_from")
	}
	if to.Sub(from) > 365*24*time.Hour {
		return CostShareResponse{}, fmt.Errorf("oauth cost share range must not exceed 365 days")
	}

	site, err := s.source.Site(ctx, query.SiteID)
	if err != nil {
		return CostShareResponse{}, err
	}
	result := CostShareResponse{Meta: CostShareMeta{
		RangeStart:  from.Format(time.RFC3339Nano),
		RangeEnd:    to.Format(time.RFC3339Nano),
		TimeZone:    timeZone.Name,
		Currency:    "USD",
		FeeCurrency: "CNY",
	}}
	if !site.IsOAuth {
		result.Data = CostShareData{UsageCurrency: "USD", FeeCurrency: "CNY", SiteID: site.ID.String(), SiteLabel: site.Name, UnsupportedReason: UnsupportedReasonOAuth, Items: []CostShareItem{}}
		return result, nil
	}
	if NormalizePlanType(site.PlanType) == "" {
		result.Data = BuildCostShare(nil, site, ReadConfig(s.confFile))
		return result, nil
	}
	rows, err := s.source.Usage(ctx, query.SiteID, from, to)
	if err != nil {
		return CostShareResponse{}, err
	}
	if s.speedSource != nil {
		speedRows, speedErr := s.speedSource.Usage(ctx, query.SiteID, from, to)
		if speedErr != nil {
			result.Meta.SpeedDengWarning = speedErr.Error()
		} else {
			result.Meta.SpeedDengDataAvailable = true
			rows = mergeUsageRows(rows, speedRows)
		}
	}
	for _, row := range rows {
		result.Meta.RequestCount += row.RequestCount
	}
	result.Data = BuildCostShare(rows, site, ReadConfig(s.confFile))
	return result, nil
}

func mergeUsageRows(source, speed []UsageRow) []UsageRow {
	if len(speed) == 0 {
		return source
	}
	deductions := make(map[string]UsageRow)
	for _, row := range speed {
		k := row.APIKeyKey
		item := deductions[k]
		item.Cost += row.Cost
		item.RequestCount += row.RequestCount
		deductions[k] = item
	}
	result := make([]UsageRow, 0, len(source)+len(speed))
	for _, row := range source {
		k := row.APIKeyKey
		deduction, ok := deductions[k]
		if !ok {
			result = append(result, row)
			continue
		}
		row.Cost -= deduction.Cost
		row.RequestCount -= deduction.RequestCount
		if row.Cost < 0 && row.Cost > -1e-8 {
			row.Cost = 0
		}
		if row.RequestCount < 0 {
			row.RequestCount = 0
		}
		if row.Cost > 0 || row.RequestCount > 0 {
			result = append(result, row)
		}
		delete(deductions, k)
	}
	for _, row := range speed {
		result = append(result, row)
	}
	return result
}

type costShareAccumulator struct {
	name      string
	usageCost float64
}

func BuildCostShare(rows []UsageRow, site OAuthSite, cfg Config) CostShareData {
	planType := NormalizePlanType(site.PlanType)
	data := CostShareData{
		UsageCurrency: "USD",
		FeeCurrency:   "CNY",
		SiteID:        site.ID.String(),
		SiteLabel:     site.Name,
		PlanType:      planType,
		Items:         []CostShareItem{},
	}
	if planType == "" {
		data.UnsupportedReason = UnsupportedReasonPlan
		return data
	}

	plan := planConfigForType(cfg, planType)
	data.SingleQuota = plan.SingleQuota
	data.ResetCount = plan.ResetCount
	data.TotalQuota = plan.SingleQuota * float64(1+plan.ResetCount)
	data.AccountFee = plan.AccountFee
	if data.TotalQuota <= 0 {
		data.UnsupportedReason = UnsupportedReasonQuota
		return data
	}
	if data.AccountFee <= 0 {
		data.UnsupportedReason = UnsupportedReasonFee
		return data
	}

	byName := map[string]*costShareAccumulator{}
	for _, row := range rows {
		key, label := NormalizeKeyName(row.APIKeyName)
		if row.SpeedDeng {
			key, label = NormalizeSpeedDengKeyName(row.APIKeyName)
		}
		item := byName[key]
		if item == nil {
			item = &costShareAccumulator{name: label}
			byName[key] = item
		}
		item.usageCost += row.Cost
	}

	items := make([]*costShareAccumulator, 0, len(byName))
	for _, item := range byName {
		items = append(items, item)
		data.TotalUsageCost += item.usageCost
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].usageCost != items[j].usageCost {
			return items[i].usageCost > items[j].usageCost
		}
		return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
	})

	data.TotalUsageRatio = data.TotalUsageCost / data.TotalQuota
	data.AllocatedCost = data.TotalUsageRatio * data.AccountFee
	data.UnallocatedCost = maxFloat(1-data.TotalUsageRatio, 0) * data.AccountFee
	data.OverQuota = data.TotalUsageRatio > 1
	data.Items = make([]CostShareItem, 0, len(items))
	for _, item := range items {
		usageShare := item.usageCost / data.TotalQuota
		data.Items = append(data.Items, CostShareItem{
			Name:          item.name,
			UsageCost:     item.usageCost,
			UsageShare:    usageShare,
			AllocatedCost: usageShare * data.AccountFee,
		})
	}
	data.Supported = true
	return data
}

func NormalizeSpeedDengKeyName(value string) (string, string) {
	name := strings.TrimSpace(value)
	name = strings.TrimSpace(strings.TrimSuffix(name, "-雷速蹬"))
	key, label := NormalizeKeyName(name)
	return key + "::speed_deng", label + "-雷速蹬"
}

func NormalizeKeyName(value string) (string, string) {
	name := strings.TrimSpace(value)
	if index := strings.IndexAny(name, "(（"); index >= 0 {
		name = strings.TrimSpace(name[:index])
	}
	for _, suffix := range []string{"员工用", "自用"} {
		if strings.HasSuffix(name, suffix) && len(strings.TrimSuffix(name, suffix)) > 0 {
			name = strings.TrimSpace(strings.TrimSuffix(name, suffix))
			break
		}
	}
	if name == "" || strings.EqualFold(name, "none") || strings.EqualFold(name, "__unknown__") || name == "未识别" {
		return "__unknown__", "未识别"
	}
	if strings.HasPrefix(name, "北海") {
		return "beihai", "北海"
	}
	return strings.ToLower(name), name
}

func planConfigForType(cfg Config, planType string) PlanConfig {
	switch planType {
	case "plus":
		return cfg.Plus
	case "pro lite":
		return cfg.ProLite
	case "pro":
		return cfg.Pro
	default:
		return PlanConfig{}
	}
}

func maxFloat(value float64, floor float64) float64 {
	if value < floor {
		return floor
	}
	return value
}
