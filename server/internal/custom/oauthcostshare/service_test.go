package oauthcostshare

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"xlyra/server/internal/config"
)

type fakeSource struct {
	site OAuthSite
	rows []UsageRow
}

type fakeSpeedSource struct {
	rows []UsageRow
	err  error
}

func (f fakeSource) Site(context.Context, uuid.UUID) (OAuthSite, error) {
	return f.site, nil
}

func (f fakeSource) Usage(context.Context, uuid.UUID, time.Time, time.Time) ([]UsageRow, error) {
	return f.rows, nil
}

func (f fakeSpeedSource) Usage(context.Context, uuid.UUID, time.Time, time.Time) ([]UsageRow, error) {
	return f.rows, f.err
}

func TestBuildCostShareGroupsNamesAcrossModelsAndAllocatesAccountFee(t *testing.T) {
	t.Parallel()

	cfg := Config{Plus: PlanConfig{SingleQuota: 100, ResetCount: 1, AccountFee: 20}}
	site := OAuthSite{ID: uuid.New(), Name: "Codex OAuth", PlanType: "plus"}
	rows := []UsageRow{
		{ModelKey: "gpt-5", APIKeyName: "Wilson（HUANGJR）", Cost: 20, RequestCount: 1},
		{ModelKey: "claude", APIKeyName: "WIlson（YANGZB）", Cost: 20, RequestCount: 1},
		{ModelKey: "gpt-5", APIKeyName: "Wilson（本体）", Cost: 0, RequestCount: 1},
		{ModelKey: "gpt-5", APIKeyName: "Other", Cost: 10, RequestCount: 1},
	}

	got := BuildCostShare(rows, site, cfg)
	if !got.Supported {
		t.Fatalf("BuildCostShare returned unsupported data: %#v", got)
	}
	if got.TotalQuota != 200 || got.TotalUsageCost != 50 || got.TotalUsageRatio != 0.25 {
		t.Fatalf("summary = %#v, want quota=200 usage=50 ratio=.25", got)
	}
	if got.AllocatedCost != 5 || got.UnallocatedCost != 15 {
		t.Fatalf("allocated/unallocated = %v/%v, want 5/15", got.AllocatedCost, got.UnallocatedCost)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %#v, want Wilson and Other", got.Items)
	}
	wilson := got.Items[0]
	if wilson.Name != "Wilson" || wilson.UsageCost != 40 || wilson.UsageShare != 0.2 || wilson.AllocatedCost != 4 {
		t.Fatalf("Wilson item = %#v, want grouped usage and allocation", wilson)
	}
}

func TestBuildCostShareSeparatesSpeedDengNameAndUsesCNYFee(t *testing.T) {
	t.Parallel()

	cfg := Config{Plus: PlanConfig{SingleQuota: 100, ResetCount: 0, AccountFee: 20}}
	site := OAuthSite{ID: uuid.New(), Name: "Codex OAuth", PlanType: "plus"}
	rows := []UsageRow{
		{APIKeyName: "Wilson", Cost: 20},
		{APIKeyName: "Wilson", Cost: 10, SpeedDeng: true},
	}

	got := BuildCostShare(rows, site, cfg)
	if got.UsageCurrency != "USD" || got.FeeCurrency != "CNY" {
		t.Fatalf("currencies = %q/%q, want USD/CNY", got.UsageCurrency, got.FeeCurrency)
	}
	if got.TotalUsageCost != 30 || got.TotalUsageRatio != 0.3 || got.AllocatedCost != 6 {
		t.Fatalf("summary = %#v, want usage=30 ratio=.3 allocated=6 CNY", got)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %#v, want separate Wilson and Wilson-雷速蹬", got.Items)
	}
	seen := map[string]bool{}
	for _, item := range got.Items {
		seen[item.Name] = true
	}
	if !seen["Wilson"] || !seen["Wilson-雷速蹬"] {
		t.Fatalf("items = %#v, want separate Wilson and Wilson-雷速蹬", got.Items)
	}
}

func TestMergeUsageRowsRemovesSpeedDengMirrorFromSourceRows(t *testing.T) {
	source := []UsageRow{
		{ModelKey: "gpt-5", APIKeyKey: "key-a", APIKeyName: "Alano", Cost: 4, RequestCount: 1},
		{ModelKey: "gpt-5.6-sol", APIKeyKey: "key-a", APIKeyName: "Alano", Cost: 6, RequestCount: 1},
	}
	speed := []UsageRow{{ModelKey: "gpt-5.6-sol", APIKeyKey: "key-a", APIKeyName: "Alano", Cost: 10, RequestCount: 2, SpeedDeng: true}}
	merged := mergeUsageRows(source, speed)
	if len(merged) != 1 {
		t.Fatalf("merged rows = %#v, want only speed row", merged)
	}
	if merged[0].Cost != 10 || merged[0].RequestCount != 2 || !merged[0].SpeedDeng {
		t.Fatalf("speed row = %#v, want captured speed usage", merged[0])
	}
}

func TestNormalizeKeyNameMapsMissingKeyMarkersToUnknown(t *testing.T) {
	t.Parallel()

	key, label := NormalizeKeyName(" none ")
	if key != "__unknown__" || label != "未识别" {
		t.Fatalf("NormalizeKeyName(none) = %q/%q, want unknown marker", key, label)
	}
}

func TestNormalizeKeyNameMergesNorthSeaLegacyAndCurrentLabels(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"北海自用",
		"北海（本体）",
		"北海员工用",
		"北海（员工用）",
		"北海新员工密钥",
		"Wilson员工用",
	} {
		key, label := NormalizeKeyName(input)
		wantKey, wantLabel := "beihai", "北海"
		if input == "Wilson员工用" {
			wantKey, wantLabel = "wilson", "Wilson"
		}
		if key != wantKey || label != wantLabel {
			t.Fatalf("NormalizeKeyName(%q) = %q/%q, want %s/%s", input, key, label, wantKey, wantLabel)
		}
	}
}

func TestBuildCostShareReturnsUnsupportedForUnknownPlanOrZeroConfig(t *testing.T) {
	t.Parallel()

	site := OAuthSite{ID: uuid.New(), Name: "OAuth", PlanType: "team"}
	unknown := BuildCostShare(nil, site, DefaultConfig())
	if unknown.Supported || unknown.UnsupportedReason != UnsupportedReasonPlan {
		t.Fatalf("unknown plan result = %#v, want unsupported plan", unknown)
	}

	site.PlanType = "plus"
	zero := BuildCostShare(nil, site, DefaultConfig())
	if zero.Supported || zero.UnsupportedReason != UnsupportedReasonQuota {
		t.Fatalf("zero config result = %#v, want quota not configured", zero)
	}
}

func TestBuildCostShareAllowsOverQuotaAndDoesNotCreateNegativeUnallocatedCost(t *testing.T) {
	t.Parallel()

	cfg := Config{Plus: PlanConfig{SingleQuota: 10, ResetCount: 0, AccountFee: 20}}
	site := OAuthSite{ID: uuid.New(), Name: "OAuth", PlanType: "plus"}
	got := BuildCostShare([]UsageRow{{APIKeyName: "Wilson", Cost: 15}}, site, cfg)

	if !got.OverQuota || got.TotalUsageRatio != 1.5 || got.AllocatedCost != 30 || got.UnallocatedCost != 0 {
		t.Fatalf("over-quota result = %#v, want ratio=1.5 allocation=30 remaining=0", got)
	}
}

func TestServiceCostShareUsesConfiguredOAuthPlanAndDateRange(t *testing.T) {
	t.Parallel()

	confFile, err := config.LoadConfigFile(t.TempDir())
	if err != nil {
		t.Fatalf("load config file: %v", err)
	}
	if err := SaveConfig(confFile, Config{Plus: PlanConfig{SingleQuota: 100, ResetCount: 1, AccountFee: 20}}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	siteID := uuid.New()
	service := NewServiceWithSource(fakeSource{
		site: OAuthSite{ID: siteID, Name: "Codex", PlanType: "PLUS", IsOAuth: true},
		rows: []UsageRow{{ModelKey: "gpt-5", APIKeyName: "Wilson", Cost: 40, RequestCount: 3}},
	}, confFile, config.LoadTimeZone("UTC"))
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)

	result, err := service.CostShare(context.Background(), CostShareQuery{
		SiteID:      siteID,
		CreatedFrom: &from,
		CreatedTo:   &to,
	}, to)
	if err != nil {
		t.Fatalf("CostShare returned error: %v", err)
	}
	if result.Meta.Currency != "USD" || result.Meta.FeeCurrency != "CNY" || result.Meta.RequestCount != 3 {
		t.Fatalf("meta = %#v, want USD/CNY and request count 3", result.Meta)
	}
	if result.Data.SiteID != siteID.String() || result.Data.PlanType != "plus" || result.Data.AllocatedCost != 4 {
		t.Fatalf("data = %#v, want normalized plan and allocated cost 4", result.Data)
	}
}

func TestServiceCostShareMergesSpeedRowsAndReportsSpeedQueryWarning(t *testing.T) {
	t.Parallel()

	siteID := uuid.New()
	base := fakeSource{
		site: OAuthSite{ID: siteID, Name: "Codex", PlanType: "plus", IsOAuth: true},
		rows: []UsageRow{{APIKeyName: "Wilson", Cost: 20, RequestCount: 1}},
	}
	cfgFile, err := config.LoadConfigFile(t.TempDir())
	if err != nil {
		t.Fatalf("load config file: %v", err)
	}
	if err := SaveConfig(cfgFile, Config{Plus: PlanConfig{SingleQuota: 100, AccountFee: 20}}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	service := NewServiceWithSources(base, fakeSpeedSource{
		rows: []UsageRow{{APIKeyName: "Wilson", Cost: 10, RequestCount: 2, SpeedDeng: true}},
	}, cfgFile, config.LoadTimeZone("UTC"))
	result, err := service.CostShare(context.Background(), CostShareQuery{SiteID: siteID, CreatedFrom: &from, CreatedTo: &to}, to)
	if err != nil {
		t.Fatalf("merged CostShare error = %v", err)
	}
	if result.Meta.RequestCount != 2 || !result.Meta.SpeedDengDataAvailable || result.Data.TotalUsageCost != 20 {
		t.Fatalf("merged result = %#v, want count=2 speed available usage=20", result)
	}

	degraded := NewServiceWithSources(base, fakeSpeedSource{err: errors.New("custom table unavailable")}, cfgFile, config.LoadTimeZone("UTC"))
	result, err = degraded.CostShare(context.Background(), CostShareQuery{SiteID: siteID, CreatedFrom: &from, CreatedTo: &to}, to)
	if err != nil {
		t.Fatalf("degraded CostShare error = %v", err)
	}
	if result.Data.TotalUsageCost != 20 || result.Meta.SpeedDengDataAvailable || result.Meta.SpeedDengWarning == "" {
		t.Fatalf("degraded result = %#v, want source-only data with warning", result)
	}
}

func TestServiceCostShareMarksNonOAuthSiteUnsupported(t *testing.T) {
	t.Parallel()

	siteID := uuid.New()
	service := NewServiceWithSource(fakeSource{
		site: OAuthSite{ID: siteID, Name: "OpenAI", IsOAuth: false},
	}, nil, config.LoadTimeZone("UTC"))

	result, err := service.CostShare(context.Background(), CostShareQuery{SiteID: siteID}, time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CostShare returned error: %v", err)
	}
	if result.Data.Supported || result.Data.UnsupportedReason != UnsupportedReasonOAuth {
		t.Fatalf("non-OAuth result = %#v, want unsupported OAuth reason", result.Data)
	}
}
