package admin

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"xlyra/server/internal/usage"
)

func TestRequestAnalyticsQueryParsesRepeatedFilters(t *testing.T) {
	t.Parallel()

	siteID := uuid.New()
	apiKeyID := uuid.New()
	values := url.Values{}
	values.Set("view", "sankey")
	values.Add("model_key", "gpt-5")
	values.Add("model_key", "claude-3-7")
	values.Add("site_id", siteID.String())
	values.Add("api_key_id", apiKeyID.String())
	values.Set("success", "false")
	req := adminTestRequest(http.MethodGet, "/api/v1/requests/analytics?"+values.Encode(), "")
	rec := adminRecorder()

	query, ok := (Handler{}).requestAnalyticsQuery(rec, req)
	adminRequireParserOK(t, rec, ok, "analytics query")
	if query.View != usage.AnalyticsViewSankey || query.Success == nil || *query.Success {
		t.Fatalf("unexpected analytics query: %#v", query)
	}
	if len(query.ModelKeys) != 2 || query.ModelKeys[0] != "gpt-5" || query.ModelKeys[1] != "claude-3-7" {
		t.Fatalf("model filters = %#v", query.ModelKeys)
	}
	if len(query.SiteIDs) != 1 || query.SiteIDs[0] != siteID || len(query.APIKeyIDs) != 1 || query.APIKeyIDs[0] != apiKeyID {
		t.Fatalf("UUID filters = %#v", query)
	}
}

func TestRequestAnalyticsQueryParsesAllStatuses(t *testing.T) {
	t.Parallel()

	req := adminTestRequest(http.MethodGet, "/api/v1/requests/analytics?view=scatter&success=all", "")
	rec := adminRecorder()
	query, ok := (Handler{}).requestAnalyticsQuery(rec, req)
	adminRequireParserOK(t, rec, ok, "analytics all-status query")
	if !query.AllStatuses || query.Success != nil {
		t.Fatalf("unexpected all-status analytics query: %#v", query)
	}
}

func TestRequestAnalyticsQueryRejectsInvalidParameters(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		target string
		code   string
	}{
		{name: "view", target: "/api/v1/requests/analytics?view=pie", code: "invalid_analytics_view"},
		{name: "success", target: "/api/v1/requests/analytics?success=maybe", code: "invalid_success"},
		{name: "site", target: "/api/v1/requests/analytics?site_id=bad", code: "invalid_site_id"},
		{name: "currency", target: "/api/v1/requests/analytics?currency=US%24", code: "invalid_currency"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := adminRecorder()
			_, ok := (Handler{}).requestAnalyticsQuery(rec, adminTestRequest(http.MethodGet, tc.target, ""))
			adminAssertParserError(t, rec, ok, tc.code)
		})
	}
}
